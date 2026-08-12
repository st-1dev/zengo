package service

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"time"
	"zengo/platform/app"
	"zengo/platform/sdk/buildinfo"
	"zengo/platform/sdk/errx"
	"zengo/platform/sdk/gateway"
	"zengo/platform/sdk/health"
	"zengo/platform/sdk/observability"
	"zengo/platform/sdk/policy"
	"zengo/platform/sdk/tlsconfig"

	loggingcfg "zengo/platform/api/config/logging"
	tracingcfg "zengo/platform/api/config/tracing"

	pgrpc "zengo/platform/sdk/transport/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Check reports probe state for the current request context.
type Check = health.Check

// Hook runs during shutdown.
type Hook func(context.Context) error

// RESTRouteGroup mounts a dedicated REST API group under an optional prefix.
type RESTRouteGroup = gateway.RouteGroup

// PolicyOptions is the canonical runtime policy config shared across transports.
type PolicyOptions = policy.Options

// RetryPolicy controls automatic retries for transient failures.
type RetryPolicy = policy.Retry

// RateLimitPolicy controls token-bucket rate limiting.
type RateLimitPolicy = policy.RateLimit

// CircuitBreakerPolicy controls the transport circuit breaker.
type CircuitBreakerPolicy = policy.CircuitBreaker

// Options describes the standard runtime for a generated service.
type Options struct {
	// Name is the logical service name used in observability payloads and /buildz.
	Name string
	// ShutdownTimeout bounds the total shutdown window used by app.App.
	ShutdownTimeout time.Duration
	// Metrics enables the shared Prometheus/OpenTelemetry metrics pipeline.
	Metrics bool
	// Tracing enables the shared OpenTelemetry tracing pipeline.
	Tracing bool
	// Logging is retained for generated-service compatibility.
	Logging *loggingcfg.Config
	// TracingCfg configures the tracing exporter when tracing is enabled.
	TracingCfg *tracingcfg.Config
	// GRPC enables and configures the managed gRPC transport.
	GRPC *GRPC
	// REST enables and configures the managed grpc-gateway HTTP transport.
	REST *REST
	// Liveness supplies /livez checks.
	Liveness map[string]Check
	// Readiness supplies /readyz checks.
	Readiness map[string]Check
	// Startup supplies /startupz checks. When empty, readiness checks are reused.
	Startup map[string]Check
	// Policies applies transport-specific runtime policies.
	Policies TransportPolicies
	// BeforeShutdown hooks run before managed components begin shutting down.
	BeforeShutdown []Hook
	// Cleanup hooks run after managed components stop, in reverse order.
	Cleanup []Hook
}

// TransportPolicies applies runtime policies to individual transports.
type TransportPolicies struct {
	// GRPC applies to the managed gRPC server.
	GRPC policy.Options
	// REST applies to the managed HTTP gateway.
	REST policy.Options
	// Kafka is exposed for generated/runtime helper wiring that manages Kafka consumers.
	Kafka policy.Options
}

// GRPC configures the gRPC transport managed by Runtime.
type GRPC struct {
	// Addr is the TCP listen address passed to the managed gRPC server wrapper.
	Addr string
	// Server is the preconfigured gRPC server with registered handlers.
	Server *grpc.Server
	// TLS configures TLS for wrappers that create a managed gRPC server internally.
	TLS *tlsconfig.ServerOptions
	// EnableReflection toggles gRPC reflection registration on the managed transport.
	EnableReflection bool
}

// REST configures the HTTP transport managed by Runtime.
type REST struct {
	// Addr is the HTTP listen address for the gateway server.
	Addr string
	// GRPCAddr is the gRPC endpoint dialed by grpc-gateway handlers.
	GRPCAddr string
	// GRPCClientTLS configures grpc-gateway TLS when dialing the backing gRPC server.
	GRPCClientTLS *tlsconfig.ClientOptions
	// TLS configures TLS and optional mTLS for the REST listener.
	TLS *tlsconfig.ServerOptions
	// Register mounts handlers on the default route group.
	Register []gateway.RegisterFunc
	// Groups mounts additional route groups, such as "/hub".
	Groups []gateway.RouteGroup
	// ExtraHandlers mounts extra HTTP handlers alongside the managed operational endpoints.
	ExtraHandlers map[string]http.Handler
}

// Runtime is the managed service runtime built from Options.
type Runtime struct {
	app *app.App
}

// New builds the managed runtime, including observability, probes, build info, and transports.
//
// Example: see ExampleNew in service_example_test.go.
func New(ctx context.Context, opts Options) (*Runtime, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("service name is required")
	}

	managed := app.New(app.Options{ShutdownTimeout: opts.ShutdownTimeout})

	if opts.Metrics || opts.Tracing {
		shutdownObs, err := observability.Init(ctx, observability.Options{
			ServiceName: opts.Name,
			TracingCfg:  opts.TracingCfg,
			Metrics:     opts.Metrics,
			Tracing:     opts.Tracing,
		})
		if err != nil {
			return nil, err
		}
		managed.AddCleanup(shutdownObs)
	}

	for _, hook := range opts.Cleanup {
		if hook != nil {
			managed.AddCleanup(hook)
		}
	}

	healthHandler := health.New(health.Options{
		Liveness:  cloneChecks(opts.Liveness),
		Readiness: cloneChecks(opts.Readiness),
		Startup:   cloneChecks(opts.Startup),
	})
	managed.AddBeforeShutdown(func(context.Context) error {
		healthHandler.MarkShuttingDown()
		return nil
	})
	for _, hook := range opts.BeforeShutdown {
		if hook != nil {
			managed.AddBeforeShutdown(hook)
		}
	}

	if opts.GRPC != nil {
		grpcSrv, err := pgrpc.New(pgrpc.Options{
			Addr:             opts.GRPC.Addr,
			Server:           opts.GRPC.Server,
			TLS:              opts.GRPC.TLS,
			EnableReflection: opts.GRPC.EnableReflection,
		})
		if err != nil {
			return nil, err
		}
		managed.AddComponent("grpc", grpcSrv)
	}

	if opts.REST != nil {
		extraHandlers := cloneHandlers(opts.REST.ExtraHandlers)
		extraHandlers["/buildz"] = buildinfo.Handler(opts.Name)
		extraHandlers["/livez"] = healthHandler.LivenessHandler()
		extraHandlers["/readyz"] = healthHandler.ReadinessHandler()
		extraHandlers["/startupz"] = healthHandler.StartupHandler()
		if opts.Metrics {
			extraHandlers["/metrics"] = observability.MetricsHandler()
		}

		restSrv, err := gateway.New(gateway.Options{
			Addr:          opts.REST.Addr,
			GRPCAddr:      opts.REST.GRPCAddr,
			GRPCTLS:       opts.REST.GRPCClientTLS,
			TLS:           opts.REST.TLS,
			Register:      opts.REST.Register,
			Groups:        opts.REST.Groups,
			ExtraHandlers: extraHandlers,
			EnableTracing: opts.Tracing,
			Policy:        opts.Policies.REST,
		})
		if err != nil {
			return nil, err
		}
		managed.AddComponent("rest", restSrv)
	}

	return &Runtime{app: managed}, nil
}

// Run starts the managed runtime and blocks until ctx is canceled or a component exits.
func (r *Runtime) Run(ctx context.Context) error {
	return r.app.Run(ctx)
}

// NewGRPCServer constructs a gRPC server with standard tracing wiring when enabled.
//
// Example: see ExampleNewGRPCServer.
func NewGRPCServer(enableTracing bool, opts ...grpc.ServerOption) (*grpc.Server, error) {
	return NewGRPCServerWithOptions(GRPCServerOptions{
		EnableTracing: enableTracing,
		ServerOptions: opts,
	})
}

// GRPCServerOptions controls standard gRPC runtime middleware composition.
type GRPCServerOptions struct {
	// EnableTracing injects the standard OpenTelemetry server interceptors.
	EnableTracing bool
	// TLS configures transport credentials for the created gRPC server.
	TLS *tlsconfig.ServerOptions
	// Policy injects the shared runtime policy interceptor ahead of user interceptors.
	Policy policy.Options
	// UnaryInterceptors are appended after the built-in policy interceptor and before errx.
	UnaryInterceptors []grpc.UnaryServerInterceptor
	// ServerOptions are passed through to grpc.NewServer.
	ServerOptions []grpc.ServerOption
}

// NewGRPCServerWithOptions constructs a gRPC server with tracing and runtime policy wiring.
func NewGRPCServerWithOptions(opts GRPCServerOptions) (*grpc.Server, error) {
	serverOpts := append([]grpc.ServerOption(nil), opts.ServerOptions...)
	if opts.TLS != nil {
		tlsCfg, err := tlsconfig.ServerConfig(opts.TLS)
		if err != nil {
			return nil, fmt.Errorf("build grpc server tls: %w", err)
		}
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}
	if opts.EnableTracing {
		serverOpts = append(observability.GRPCServerOptions(), serverOpts...)
	}
	interceptors := compactUnaryInterceptors(opts.UnaryInterceptors)
	if opts.Policy.Enabled() {
		interceptors = append(
			[]grpc.UnaryServerInterceptor{policy.GRPCUnaryServerInterceptor(opts.Policy)},
			interceptors...)
	}
	interceptors = append([]grpc.UnaryServerInterceptor{errx.UnaryServerInterceptor()}, interceptors...)
	switch len(interceptors) {
	case 0:
	case 1:
		serverOpts = append(serverOpts, grpc.UnaryInterceptor(interceptors[0]))
	default:
		serverOpts = append(serverOpts, grpc.ChainUnaryInterceptor(interceptors...))
	}
	return grpc.NewServer(serverOpts...), nil
}

// PrintVersion writes the canonical build metadata JSON used by generated services.
//
// Example: see ExamplePrintVersion.
func PrintVersion(w io.Writer, serviceName string) error {
	return buildinfo.Print(w, serviceName)
}

func cloneChecks(src map[string]Check) map[string]Check {
	if len(src) == 0 {
		return map[string]Check{}
	}
	dst := make(map[string]Check, len(src))
	maps.Copy(dst, src)
	return dst
}

func cloneHandlers(src map[string]http.Handler) map[string]http.Handler {
	if len(src) == 0 {
		return map[string]http.Handler{}
	}
	dst := make(map[string]http.Handler, len(src))
	maps.Copy(dst, src)
	return dst
}

func compactUnaryInterceptors(src []grpc.UnaryServerInterceptor) []grpc.UnaryServerInterceptor {
	if len(src) == 0 {
		return nil
	}
	dst := make([]grpc.UnaryServerInterceptor, 0, len(src))
	for _, interceptor := range src {
		if interceptor != nil {
			dst = append(dst, interceptor)
		}
	}
	return dst
}
