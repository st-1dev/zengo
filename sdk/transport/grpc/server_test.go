package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	gogrpc "google.golang.org/grpc"
)

func TestNewUsesProvidedServer(t *testing.T) {
	oldListen := listen
	listen = func(_, _ string) (net.Listener, error) {
		return &stubListener{addr: stubAddr("127.0.0.1:0")}, nil
	}
	defer func() { listen = oldListen }()

	inner := gogrpc.NewServer()
	srv, err := New(Options{Addr: "127.0.0.1:0", Server: inner})
	if err != nil {
		t.Fatal(err)
	}
	if srv.GRPC() != inner {
		t.Fatal("expected provided grpc server to be reused")
	}
}

func TestShutdownReturnsContextErrorWhenExpired(t *testing.T) {
	oldListen := listen
	listen = func(_, _ string) (net.Listener, error) {
		return &stubListener{addr: stubAddr("127.0.0.1:0")}, nil
	}
	defer func() { listen = oldListen }()

	srv, err := New(Options{Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)
	err = srv.Shutdown(ctx)
	if err == nil {
		t.Fatal("expected context error")
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
