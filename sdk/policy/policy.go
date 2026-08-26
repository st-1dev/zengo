package policy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

var (
	// ErrRateLimited reports that rate limiting rejected the operation.
	ErrRateLimited = errors.New("policy: rate limited")
	// ErrCircuitOpen reports that the circuit breaker rejected the operation.
	ErrCircuitOpen = errors.New("policy: circuit open")
	// ErrConcurrencyLimited reports that the concurrency semaphore rejected the operation.
	ErrConcurrencyLimited = errors.New("policy: concurrency limit reached")
)

// Options configures the shared runtime policy executor.
type Options struct {
	// Timeout applies a per-attempt deadline to the wrapped operation.
	Timeout time.Duration
	// Retry controls retry attempts for retryable failures.
	Retry Retry
	// RateLimit controls request admission over time.
	RateLimit RateLimit
	// CircuitBreaker controls temporary rejection after repeated failures.
	CircuitBreaker CircuitBreaker
	// ConcurrencyLimit caps the number of in-flight operations.
	ConcurrencyLimit int
}

// Retry configures retry count and backoff between attempts.
type Retry struct {
	// Attempts is the total number of attempts, including the first call.
	Attempts int
	// Backoff is the wait duration between retry attempts.
	Backoff time.Duration
}

// RateLimit configures token-bucket style admission limits.
type RateLimit struct {
	// Requests is the steady-state request budget for each period.
	Requests int
	// Per is the token refill period.
	Per time.Duration
	// Burst is the maximum burst size. Zero defaults to Requests.
	Burst int
}

// CircuitBreaker configures when failures open the circuit and for how long.
type CircuitBreaker struct {
	// Failures is the number of consecutive failures that opens the circuit.
	Failures int
	// OpenFor is the duration the circuit remains open before trying again.
	OpenFor time.Duration
}

// Enabled reports whether any policy behavior is active.
func (o Options) Enabled() bool {
	return o.Timeout > 0 ||
		o.Retry.Attempts > 1 ||
		o.RateLimit.Requests > 0 ||
		o.CircuitBreaker.Failures > 0 ||
		o.ConcurrencyLimit > 0
}

// Executor applies Options to arbitrary function calls.
type Executor struct {
	opts    Options
	limiter *rateLimiter
	breaker *circuitState
	sem     chan struct{}
}

// NewExecutor creates an Executor with normalized option defaults.
func NewExecutor(opts Options) *Executor {
	exec := &Executor{opts: normalize(opts)}
	if exec.opts.RateLimit.Requests > 0 && exec.opts.RateLimit.Per > 0 {
		exec.limiter = newRateLimiter(exec.opts.RateLimit)
	}
	if exec.opts.CircuitBreaker.Failures > 0 && exec.opts.CircuitBreaker.OpenFor > 0 {
		exec.breaker = newCircuitState(exec.opts.CircuitBreaker)
	}
	if exec.opts.ConcurrencyLimit > 0 {
		exec.sem = make(chan struct{}, exec.opts.ConcurrencyLimit)
	}
	return exec
}

// Do runs fn under the configured policy chain.
func (e *Executor) Do(ctx context.Context, fn func(context.Context) error) error {
	if e == nil {
		return fn(ctx)
	}
	if e.limiter != nil && !e.limiter.Allow(time.Now()) {
		return ErrRateLimited
	}
	if e.breaker != nil && e.breaker.Open(time.Now()) {
		return ErrCircuitOpen
	}
	if e.sem != nil {
		select {
		case e.sem <- struct{}{}:
			defer func() { <-e.sem }()
		default:
			return ErrConcurrencyLimited
		}
	}

	attempts := e.opts.Retry.Attempts
	var finalErr error
	for attempt := range attempts {
		callCtx := ctx
		cancel := func() {}
		if e.opts.Timeout > 0 {
			callCtx, cancel = context.WithTimeout(ctx, e.opts.Timeout)
		}
		err := fn(callCtx)
		cancel()
		if err == nil {
			if e.breaker != nil {
				e.breaker.Success()
			}
			return nil
		}
		finalErr = err
		if attempt == attempts-1 || !shouldRetry(err) {
			break
		}
		err = sleepContext(ctx, e.opts.Retry.Backoff)
		if err != nil {
			finalErr = err
			break
		}
	}
	if e.breaker != nil {
		e.breaker.Failure(time.Now())
	}
	return finalErr
}

// GRPCUnaryServerInterceptor applies Options to gRPC unary handlers.
func GRPCUnaryServerInterceptor(opts Options) grpc.UnaryServerInterceptor {
	serverOpts := opts
	serverOpts.Retry.Attempts = 1
	exec := NewExecutor(serverOpts)
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		var resp any
		err := exec.Do(ctx, func(callCtx context.Context) error {
			out, err := handler(callCtx, req)
			if err == nil {
				resp = out
			}
			return err
		})
		if err == nil {
			return resp, nil
		}
		return nil, grpcError(err)
	}
}

// HTTPMiddleware applies Options to an HTTP handler tree.
func HTTPMiddleware(opts Options) func(http.Handler) http.Handler {
	enabled := opts.Enabled()
	serverOpts := opts
	serverOpts.Retry.Attempts = 1
	exec := NewExecutor(serverOpts)
	return func(next http.Handler) http.Handler {
		if next == nil {
			return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
		}
		if !enabled {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var capture *bufferedResponse
			err := exec.Do(r.Context(), func(callCtx context.Context) error {
				current := newBufferedResponse()
				var handlerErr error
				func() {
					defer func() {
						recovered := recover()
						if recovered != nil {
							handlerErr = fmt.Errorf("http handler panic: %v", recovered)
							if current.status == 0 {
								current.status = http.StatusInternalServerError
							}
						}
					}()
					next.ServeHTTP(current, r.WithContext(callCtx))
				}()
				capture = current
				if handlerErr != nil {
					return handlerErr
				}
				if current.Status() >= http.StatusInternalServerError {
					return responseError{capture: current}
				}
				return nil
			})
			if err == nil {
				capture.Commit(w)
				return
			}
			var respErr responseError
			switch {
			case errors.As(err, &respErr):
				respErr.capture.Commit(w)
			case errors.Is(err, ErrRateLimited):
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			case errors.Is(err, ErrCircuitOpen), errors.Is(err, ErrConcurrencyLimited):
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			case errors.Is(err, context.DeadlineExceeded):
				http.Error(w, http.StatusText(http.StatusGatewayTimeout), http.StatusGatewayTimeout)
			default:
				if capture != nil && capture.Status() >= http.StatusInternalServerError {
					capture.Commit(w)
					return
				}
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		})
	}
}

func normalize(opts Options) Options {
	if opts.Retry.Attempts <= 0 {
		opts.Retry.Attempts = 1
	}
	if opts.RateLimit.Requests > 0 && opts.RateLimit.Burst <= 0 {
		opts.RateLimit.Burst = opts.RateLimit.Requests
	}
	return opts
}

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrRateLimited),
		errors.Is(err, ErrCircuitOpen),
		errors.Is(err, ErrConcurrencyLimited),
		errors.Is(err, context.Canceled):
		return false
	case errors.Is(err, context.DeadlineExceeded):
		return true
	}
	var respErr responseError
	if errors.As(err, &respErr) {
		return respErr.capture.Status() >= http.StatusInternalServerError
	}
	code := grpcstatus.Code(err)
	switch code {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Aborted, codes.Internal:
		return true
	case codes.Unknown:
		return true
	default:
		return false
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func grpcError(err error) error {
	switch {
	case errors.Is(err, ErrRateLimited):
		return grpcstatus.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, ErrCircuitOpen), errors.Is(err, ErrConcurrencyLimited):
		return grpcstatus.Error(codes.Unavailable, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return grpcstatus.Error(codes.DeadlineExceeded, err.Error())
	default:
		return err
	}
}

type responseError struct {
	capture *bufferedResponse
}

func (e responseError) Error() string {
	if e.capture == nil {
		return "http response error"
	}
	return fmt.Sprintf("http response status %d", e.capture.Status())
}

type bufferedResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newBufferedResponse() *bufferedResponse {
	return &bufferedResponse{header: make(http.Header)}
}

func (r *bufferedResponse) Header() http.Header {
	return r.header
}

func (r *bufferedResponse) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(data)
}

func (r *bufferedResponse) WriteHeader(statusCode int) {
	if r.status != 0 {
		return
	}
	r.status = statusCode
}

func (r *bufferedResponse) Status() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}

func (r *bufferedResponse) Commit(w http.ResponseWriter) {
	for key, values := range r.header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(r.Status())
	_, _ = w.Write(r.body.Bytes())
}

type rateLimiter struct {
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
	mu     sync.Mutex
}

func newRateLimiter(opts RateLimit) *rateLimiter {
	rate := float64(opts.Requests) / opts.Per.Seconds()
	return &rateLimiter{
		rate:   rate,
		burst:  float64(opts.Burst),
		tokens: float64(opts.Burst),
		last:   time.Now(),
	}
}

func (l *rateLimiter) Allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	elapsed := now.Sub(l.last).Seconds()
	l.last = now
	l.tokens = minFloat(l.burst, l.tokens+(elapsed*l.rate))
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

type circuitState struct {
	failures int
	openFor  time.Duration

	mu          sync.Mutex
	consecutive int
	openUntil   time.Time
}

func newCircuitState(opts CircuitBreaker) *circuitState {
	return &circuitState{failures: opts.Failures, openFor: opts.OpenFor}
}

func (c *circuitState) Open(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return now.Before(c.openUntil)
}

func (c *circuitState) Success() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.consecutive = 0
	c.openUntil = time.Time{}
}

func (c *circuitState) Failure(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.consecutive++
	if c.consecutive >= c.failures {
		c.openUntil = now.Add(c.openFor)
		c.consecutive = 0
	}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
