package observability

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// HTTPHandler wraps handler with OpenTelemetry HTTP instrumentation.
func HTTPHandler(name string, handler http.Handler) http.Handler {
	return otelhttp.NewHandler(handler, name)
}
