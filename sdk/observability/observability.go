package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	tracingcfg "zengo/platform/api/config/tracing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Options configures the observability providers installed by Init.
type Options struct {
	// ServiceName is attached to metrics and traces.
	ServiceName string
	// TracingCfg configures the tracing exporter when tracing is enabled.
	TracingCfg *tracingcfg.Config
	// Metrics enables the Prometheus/OpenTelemetry metrics pipeline.
	Metrics bool
	// Tracing enables the OpenTelemetry tracing pipeline.
	Tracing bool
}

// Init configures metrics and/or tracing. Metrics and tracing are independent;
// either or both may be enabled. Returns a shutdown function that must be
// invoked on process exit.
func Init(ctx context.Context, opts Options) (shutdown func(context.Context) error, err error) {
	var cleanups []func(context.Context) error
	if opts.Metrics {
		exporter, err := prometheus.New()
		if err != nil {
			return nil, fmt.Errorf("init prometheus exporter: %w", err)
		}
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
		otel.SetMeterProvider(provider)
		cleanups = append(cleanups, provider.Shutdown)
	}
	if opts.Tracing {
		shutdownTracer, err := initTracer(ctx, opts.ServiceName, opts.TracingCfg)
		if err != nil {
			cleanupErr := buildShutdown(cleanups)(ctx)
			return nil, errors.Join(fmt.Errorf("init tracer: %w", err), cleanupErr)
		}
		cleanups = append(cleanups, shutdownTracer)
	}
	return buildShutdown(cleanups), nil
}

func buildShutdown(cleanups []func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		var errs []error
		for i := len(cleanups) - 1; i >= 0; i-- {
			err := cleanups[i](ctx)
			if err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
}

// MetricsHandler returns the shared Prometheus scrape handler.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
