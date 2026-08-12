package health_test

import (
	"context"
	"net/http"
	"zengo/platform/sdk/health"
)

func ExampleNew() {
	handler := health.New(health.Options{
		Readiness: map[string]health.Check{
			"db": func(context.Context) error { return nil },
		},
	})
	mux := http.NewServeMux()
	handler.Register(mux)
}
