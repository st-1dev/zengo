package service

import (
	"strings"
	"testing"
	"zengo/platform/sdk/tlsconfig"
)

func TestNewGRPCServerWithOptionsReturnsTLSError(t *testing.T) {
	_, err := NewGRPCServerWithOptions(GRPCServerOptions{
		TLS: &tlsconfig.ServerOptions{},
	})
	if err == nil {
		t.Fatal("expected TLS error")
	}
	if !strings.Contains(err.Error(), "build grpc server tls") {
		t.Fatalf("error = %v, want TLS context", err)
	}
}
