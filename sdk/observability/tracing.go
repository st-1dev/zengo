package observability

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"zengo/platform/sdk/tlsconfig"

	tracingcfg "zengo/platform/api/config/tracing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

// tracerState stores the initialized tracer.
// The struct wrapper allows storing an interface safely in atomic.Pointer
// without panicking on nil values or concrete-type changes.
type tracerState struct {
	tracer trace.Tracer
}

// tracerHolder stores the package-wide tracer state installed by Init.
var tracerHolder atomic.Pointer[tracerState]

// storeTracer installs the tracer used by the package.
// Call it once during application startup before tracing begins.
// If Init is not called, tracing helpers behave as no-ops.
func storeTracer(tracer trace.Tracer) {
	tracerHolder.Store(&tracerState{tracer: tracer})
}

// loadTracer returns the tracer installed by Init, or nil when Init has not
// been called.
func loadTracer() trace.Tracer {
	state := tracerHolder.Load()
	if state == nil {
		return nil
	}
	return state.tracer
}

func initTracer(ctx context.Context, serviceName string, cfg *tracingcfg.Config) (func(context.Context) error, error) {
	exporter, err := newTraceExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}
	ratio := samplingRatio(cfg)
	var res *resource.Resource
	res, err = resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			attribute.String("service.name", serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("trace resource: %w", err)
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	storeTracer(provider.Tracer(serviceName))
	return provider.Shutdown, nil
}

func newTraceExporter(ctx context.Context, cfg *tracingcfg.Config) (sdktrace.SpanExporter, error) {
	if cfg == nil || cfg.GetSpec() == nil {
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	}
	spec := cfg.GetSpec()
	endpoint := strings.TrimSpace(spec.GetEndpoint())
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}
	if endpoint == "" {
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	}
	useInsecure := strings.HasPrefix(endpoint, "http://") || os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true"
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")
	clientTLS, err := tlsconfig.ClientConfig(tlsconfig.ClientOptionsFromProto(spec.GetTls()))
	if err != nil {
		return nil, fmt.Errorf("trace exporter tls: %w", err)
	}
	switch spec.GetProtocol() {
	case tracingcfg.Spec_HTTP:
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(endpoint)}
		if useInsecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		} else if clientTLS != nil {
			opts = append(opts, otlptracehttp.WithTLSClientConfig(clientTLS))
		}
		return otlptracehttp.New(ctx, opts...)
	default:
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(endpoint)}
		if useInsecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		} else if clientTLS != nil {
			opts = append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(clientTLS)))
		}
		return otlptracegrpc.New(ctx, opts...)
	}
}

func samplingRatio(cfg *tracingcfg.Config) float64 {
	if cfg == nil || cfg.GetSpec() == nil || cfg.GetSpec().SamplingRatio == nil {
		v := os.Getenv("OTEL_TRACES_SAMPLER_ARG")
		if v != "" {
			var f float64
			_, err := fmt.Sscanf(v, "%f", &f)
			if err == nil && f >= 0 && f <= 1 {
				return f
			}
		}
		return 1.0
	}
	r := cfg.GetSpec().GetSamplingRatio()
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}
