package observability_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"zengo/platform/sdk/observability"
)

func TestHTTPHandlerAllowsNilHandler(t *testing.T) {
	h := observability.HTTPHandler("test", nil)
	if h == nil {
		t.Fatal("expected wrapper handler")
	}
}

func TestHTTPHandlerWrapsHandler(t *testing.T) {
	called := false
	h := observability.HTTPHandler("test", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called {
		t.Fatal("expected underlying handler to run")
	}
}
