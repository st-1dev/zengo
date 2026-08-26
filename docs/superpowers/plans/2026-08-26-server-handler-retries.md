# Server Handler Retry Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply server policy limits and timeout once per request while ensuring HTTP and unary gRPC handlers are never retried.

**Architecture:** Each server adapter copies its supplied `Options`, sets only `Retry.Attempts` to one on that copy, and creates its `Executor` from the copy. `HTTPMiddleware` records `opts.Enabled()` before this neutralization, so retry-only configuration still selects the existing buffered, panic-safe HTTP wrapper; direct `NewExecutor` behavior remains untouched.

**Tech Stack:** Go 1.26, Go standard `net/http`, `net/http/httptest`, `sync/atomic`, `testing`, and `google.golang.org/grpc`.

**Spec:** `docs/superpowers/specs/2026-08-26-server-handler-retries-design.md`

**Issue:** https://github.com/st-1dev/zengo/issues/3

## Global Constraints

- Public signatures remain unchanged.
- `Executor.Do` and its retry semantics remain unchanged.
- Each server adapter uses a local `Options` copy with `Retry.Attempts` forced to one; all other policy options remain unchanged.
- `HTTPMiddleware` evaluates the original `opts.Enabled()` before retry neutralization.
- HTTP 5xx and retryable unary gRPC statuses are returned after a single handler call.
- Do not add idempotency configuration, client retry middleware, stream interceptors, dependencies, or alter retry classification.

---

## Files

- Modify: `sdk/policy/policy.go` — construct server-only executors with one attempt while retaining all other policy behavior.
- Modify: `sdk/policy/policy_test.go` — replace the retrying HTTP server assertion and add gRPC and retry-only wrapper regressions.

### Task 1: Neutralize retries only at server-adapter boundaries

**Files:**
- Modify: `sdk/policy/policy.go:150-228`
- Test: `sdk/policy/policy_test.go:59-96`

**Interfaces:**
- Consumes: `type Options struct { Retry Retry }`, `func (Options) Enabled() bool`, `func NewExecutor(Options) *Executor`, and `func (e *Executor) Do(context.Context, func(context.Context) error) error`.
- Produces: `GRPCUnaryServerInterceptor(opts Options) grpc.UnaryServerInterceptor` and `HTTPMiddleware(opts Options) func(http.Handler) http.Handler` that execute a handler at most once while applying the original timeout, rate limit, circuit breaker, and concurrency options.

- [ ] **Step 1: Replace the HTTP retry test and add gRPC and retry-only wrapper regression tests**

Replace `TestHTTPMiddlewareRetriesServerErrors` and add these tests in `sdk/policy/policy_test.go`:

```go
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
```

- [ ] **Step 2: Run the focused tests to verify the handlers are currently retried**

Run: `go test ./sdk/policy -run '^(TestHTTPMiddlewareDoesNotRetryServerErrors|TestHTTPMiddlewareKeepsRetryOnlyPolicyEnabled|TestGRPCUnaryServerInterceptorDoesNotRetryHandlers)$' -count=1`

Expected: RED — the HTTP 500 and retry-only panic cases report two or three handler calls, and the gRPC interceptor reports three calls.

- [ ] **Step 3: Copy options inside each server adapter and force exactly one attempt**

At the start of `GRPCUnaryServerInterceptor`, create its executor from a local copy:

```go
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
```

Replace `HTTPMiddleware` with this complete function. It evaluates original enablement before copying and neutralizing only retries.

```go
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
```

Run: `gofmt -w sdk/policy/policy.go sdk/policy/policy_test.go`

Expected: `gofmt` completes without output.

- [ ] **Step 4: Run the focused tests to verify one server-handler invocation**

Run: `go test ./sdk/policy -run '^(TestHTTPMiddlewareDoesNotRetryServerErrors|TestHTTPMiddlewareKeepsRetryOnlyPolicyEnabled|TestGRPCUnaryServerInterceptorDoesNotRetryHandlers)$' -count=1`

Expected: GREEN — HTTP returns its single 500 response, retry-only options still use the panic-safe wrapper, and the gRPC `codes.Unavailable` status is returned after one handler call.

- [ ] **Step 5: Verify the direct executor and all policy tests**

Run: `go test ./sdk/policy -run '^TestExecutorRetriesAndRecovers$' -count=1`

Expected: GREEN — direct `Executor.Do` still invokes its function three times and succeeds.

Run: `go test ./sdk/policy -count=1`

Expected: GREEN — rate limit, concurrency, HTTP, gRPC, and direct executor tests pass.

Run: `go test ./...`

Expected: GREEN — all repository tests pass.

- [ ] **Step 6: Commit the implementation**

```bash
git add sdk/policy/policy.go sdk/policy/policy_test.go
git commit -m "fix(policy): run server handlers once"
```
