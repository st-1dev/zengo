package gateway

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"zengo/platform/sdk/policy"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

func TestNewReturnsRegisterError(t *testing.T) {
	oldListen := listen
	listen = func(_, _ string) (net.Listener, error) {
		return &stubListener{addr: stubAddr("127.0.0.1:0")}, nil
	}
	defer func() { listen = oldListen }()

	_, err := New(Options{
		Addr:     "127.0.0.1:0",
		GRPCAddr: "127.0.0.1:9090",
		Register: []RegisterFunc{
			func(_ context.Context, _ *runtime.ServeMux, _ string, _ []grpc.DialOption) error {
				return errors.New("boom")
			},
		},
	})
	if err == nil {
		t.Fatal("expected register error")
	}
}

func TestNewReturnsConfiguredAddr(t *testing.T) {
	oldListen := listen
	listen = func(_, _ string) (net.Listener, error) {
		return &stubListener{addr: stubAddr("127.0.0.1:18080")}, nil
	}
	defer func() { listen = oldListen }()

	srv, err := New(Options{Addr: "127.0.0.1:18080", GRPCAddr: "127.0.0.1:9090"})
	if err != nil {
		t.Fatal(err)
	}
	if srv.Addr() != "127.0.0.1:18080" {
		t.Fatalf("addr = %q", srv.Addr())
	}
	if srv.server.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout = %s, want 5s", srv.server.ReadHeaderTimeout)
	}
}

func TestNewUsesConfiguredReadHeaderTimeout(t *testing.T) {
	oldListen := listen
	listen = func(_, _ string) (net.Listener, error) {
		return &stubListener{addr: stubAddr("127.0.0.1:18080")}, nil
	}
	defer func() { listen = oldListen }()

	srv, err := New(Options{
		Addr:              "127.0.0.1:18080",
		ReadHeaderTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if srv.server.ReadHeaderTimeout != time.Second {
		t.Fatalf("ReadHeaderTimeout = %s, want 1s", srv.server.ReadHeaderTimeout)
	}
}

func TestNewCallsRegisters(t *testing.T) {
	oldListen := listen
	listen = func(_, _ string) (net.Listener, error) {
		return &stubListener{addr: stubAddr("127.0.0.1:18080")}, nil
	}
	defer func() { listen = oldListen }()

	called := false
	srv, err := New(Options{
		Addr:     "127.0.0.1:18080",
		GRPCAddr: "127.0.0.1:9090",
		Register: []RegisterFunc{
			func(_ context.Context, _ *runtime.ServeMux, _ string, _ []grpc.DialOption) error {
				called = true
				return nil
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected register to be called")
	}
	if srv == nil {
		t.Fatal("expected server")
	}
}

func TestNewAppliesHTTPPolicy(t *testing.T) {
	oldListen := listen
	listen = func(_, _ string) (net.Listener, error) {
		return &stubListener{addr: stubAddr("127.0.0.1:18080")}, nil
	}
	defer func() { listen = oldListen }()

	srv, err := New(Options{
		Addr: "127.0.0.1:18080",
		ExtraHandlers: map[string]http.Handler{
			"/ready": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		},
		Policy: policy.Options{
			RateLimit: policy.RateLimit{Requests: 1, Per: time.Hour, Burst: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	first := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(
		first,
		httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ready", nil),
	)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d", first.Code)
	}

	second := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(
		second,
		httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ready", nil),
	)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d", second.Code)
	}
}

func TestNewMountsPrefixedRouteGroup(t *testing.T) {
	oldListen := listen
	listen = func(_, _ string) (net.Listener, error) {
		return &stubListener{addr: stubAddr("127.0.0.1:18080")}, nil
	}
	defer func() { listen = oldListen }()

	srv, err := New(Options{
		Addr: "127.0.0.1:18080",
		Groups: []RouteGroup{
			{
				Prefix:   "/hub",
				GRPCAddr: "127.0.0.1:9090",
				Register: []RegisterFunc{
					func(_ context.Context, mux *runtime.ServeMux, _ string, _ []grpc.DialOption) error {
						return mux.HandlePath(
							http.MethodGet,
							"/users",
							func(w http.ResponseWriter, _ *http.Request, _ map[string]string) {
								w.WriteHeader(http.StatusNoContent)
							},
						)
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(
		rr,
		httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/hub/users", nil),
	)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rr.Code)
	}
}

type stubListener struct {
	addr net.Addr
}

func (l *stubListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (l *stubListener) Close() error              { return nil }
func (l *stubListener) Addr() net.Addr            { return l.addr }

type stubAddr string

func (a stubAddr) Network() string { return "tcp" }
func (a stubAddr) String() string  { return string(a) }
