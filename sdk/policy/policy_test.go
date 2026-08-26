package policy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestExecutorRetriesAndRecovers(t *testing.T) {
	var calls int32
	exec := NewExecutor(Options{
		Retry: Retry{Attempts: 3},
		CircuitBreaker: CircuitBreaker{
			Failures: 2,
			OpenFor:  time.Second,
		},
	})
	err := exec.Do(context.Background(), func(context.Context) error {
		if atomic.AddInt32(&calls, 1) < 3 {
			return errors.New("boom")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestHTTPMiddlewareRateLimit(t *testing.T) {
	handler := HTTPMiddleware(Options{
		RateLimit: RateLimit{Requests: 1, Per: time.Minute, Burst: 1},
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d", second.Code)
	}
}

func TestHTTPMiddlewareDoesNotRetryServerErrors(t *testing.T) {
	var calls int32
	handler := HTTPMiddleware(Options{Retry: Retry{Attempts: 2}})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)
			http.Error(w, "server failure", http.StatusInternalServerError)
		}),
	)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestHTTPMiddlewareKeepsRetryOnlyPolicyEnabled(t *testing.T) {
	var calls int32
	handler := HTTPMiddleware(Options{Retry: Retry{Attempts: 3}})(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			atomic.AddInt32(&calls, 1)
			panic("handler panic")
		}),
	)

	rr := httptest.NewRecorder()
	var escaped any
	func() {
		defer func() { escaped = recover() }()
		handler.ServeHTTP(rr, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil))
	}()
	if escaped != nil {
		t.Fatalf("panic escaped: %v", escaped)
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestGRPCUnaryServerInterceptorDoesNotRetryHandlers(t *testing.T) {
	var calls int32
	interceptor := GRPCUnaryServerInterceptor(Options{Retry: Retry{Attempts: 3}})
	_, err := interceptor(
		context.Background(),
		"request",
		nil,
		func(context.Context, any) (any, error) {
			atomic.AddInt32(&calls, 1)
			return nil, grpcstatus.Error(codes.Unavailable, "temporarily unavailable")
		},
	)
	if grpcstatus.Code(err) != codes.Unavailable {
		t.Fatalf("code = %v", grpcstatus.Code(err))
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestGRPCUnaryServerInterceptorMapsPolicyErrors(t *testing.T) {
	interceptor := GRPCUnaryServerInterceptor(Options{
		RateLimit: RateLimit{Requests: 1, Per: time.Minute, Burst: 1},
	})
	handler := func(context.Context, any) (any, error) { return "ok", nil }
	_, err := interceptor(context.Background(), "req", nil, handler)
	if err != nil {
		t.Fatal(err)
	}
	_, err = interceptor(context.Background(), "req", nil, handler)
	if grpcstatus.Code(err) != codes.ResourceExhausted {
		t.Fatalf("code = %v", grpcstatus.Code(err))
	}
}

func TestExecutorConcurrencyLimit(t *testing.T) {
	exec := NewExecutor(Options{ConcurrencyLimit: 1})
	blocked := make(chan struct{})
	done := make(chan struct{})
	go func() {
		_ = exec.Do(context.Background(), func(context.Context) error {
			close(blocked)
			<-done
			return nil
		})
	}()
	<-blocked
	err := exec.Do(context.Background(), func(context.Context) error { return nil })
	if !errors.Is(err, ErrConcurrencyLimited) {
		t.Fatalf("err = %v", err)
	}
	close(done)
}
