package gateway

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
	"zengo/platform/sdk/observability"
	"zengo/platform/sdk/policy"
	"zengo/platform/sdk/tlsconfig"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var listen = net.Listen

// Server serves grpc-gateway handlers and optional extra HTTP endpoints.
type Server struct {
	mux    *runtime.ServeMux
	server *http.Server
	lis    net.Listener
	addr   string
}

// RegisterFunc matches the grpc-gateway generated registration function shape.
type RegisterFunc func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error

// RouteGroup mounts a dedicated grpc-gateway mux under an optional URL prefix.
type RouteGroup struct {
	// Prefix is the URL path prefix mounted in front of the group, for example "/hub".
	Prefix string
	// GRPCAddr overrides the default gRPC endpoint used by the group's handlers.
	GRPCAddr string
	// Register holds grpc-gateway registration functions for this route group.
	Register []RegisterFunc
}

// Options controls gateway transport setup.
type Options struct {
	// Addr is the HTTP listen address for the gateway server.
	Addr string
	// GRPCAddr is the default gRPC endpoint dialed by Register handlers.
	GRPCAddr string
	// GRPCTLS configures grpc-gateway TLS when it dials the backing gRPC server.
	GRPCTLS *tlsconfig.ClientOptions
	// TLS configures TLS and optional mTLS for the HTTP listener.
	TLS *tlsconfig.ServerOptions
	// Register mounts handlers on the default route group.
	Register []RegisterFunc
	// Groups mounts additional route groups, optionally under prefixes.
	Groups []RouteGroup
	// ExtraHandlers are mounted on the root mux alongside grpc-gateway handlers.
	ExtraHandlers map[string]http.Handler
	// EnableTracing wraps HTTP and gRPC client handlers with OpenTelemetry.
	// When false, handlers are registered without otel instrumentation.
	EnableTracing bool
	// Policy wraps the top-level HTTP handler with runtime policy middleware.
	Policy policy.Options
	// ReadHeaderTimeout bounds how long the server waits for request headers.
	ReadHeaderTimeout time.Duration
}

// New constructs a gateway server and registers all provided handlers.
func New(opts Options) (*Server, error) {
	root := http.NewServeMux()
	groups := normalizeGroups(opts)
	var defaultMux *runtime.ServeMux
	for i, group := range groups {
		mux, err := newGroupMux(group, opts)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			defaultMux = mux
		}
		mountGroup(root, group.Prefix, mux)
	}
	if defaultMux == nil {
		defaultMux = runtime.NewServeMux()
		root.Handle("/", defaultMux)
	}
	for pattern, handler := range opts.ExtraHandlers {
		root.Handle(pattern, handler)
	}
	handler := http.Handler(root)
	if opts.Policy.Enabled() {
		handler = policy.HTTPMiddleware(opts.Policy)(handler)
	}
	if opts.EnableTracing {
		handler = observability.HTTPHandler("rest", handler)
	}
	lis, err := listen("tcp", opts.Addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", opts.Addr, err)
	}
	if opts.TLS != nil {
		tlsCfg, err := tlsconfig.ServerConfig(opts.TLS)
		if err != nil {
			_ = lis.Close()
			return nil, fmt.Errorf("gateway server tls: %w", err)
		}
		lis = tls.NewListener(lis, tlsCfg)
	}
	readHeaderTimeout := opts.ReadHeaderTimeout
	if readHeaderTimeout <= 0 {
		readHeaderTimeout = 5 * time.Second
	}
	return &Server{
		mux: defaultMux,
		server: &http.Server{
			Addr:              opts.Addr,
			Handler:           handler,
			ReadHeaderTimeout: readHeaderTimeout,
		},
		lis:  lis,
		addr: opts.Addr,
	}, nil
}

func normalizeGroups(opts Options) []RouteGroup {
	groups := make([]RouteGroup, 0, len(opts.Groups)+1)
	if len(opts.Register) > 0 {
		groups = append(groups, RouteGroup{
			GRPCAddr: opts.GRPCAddr,
			Register: opts.Register,
		})
	}
	for _, group := range opts.Groups {
		if group.GRPCAddr == "" {
			group.GRPCAddr = opts.GRPCAddr
		}
		groups = append(groups, group)
	}
	return groups
}

func newGroupMux(group RouteGroup, opts Options) (*runtime.ServeMux, error) {
	mux := runtime.NewServeMux()
	endpoint := group.GRPCAddr
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if opts.GRPCTLS != nil {
		tlsCfg, err := tlsconfig.ClientConfig(opts.GRPCTLS)
		if err != nil {
			return nil, fmt.Errorf("gateway grpc client tls: %w", err)
		}
		dialOpts = []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg))}
	}
	if opts.EnableTracing {
		dialOpts = append(dialOpts, observability.GRPCDialOptions()...)
	}
	ctx := context.Background()
	if len(group.Register) > 0 && endpoint == "" {
		return nil, fmt.Errorf("gateway grpc address is required")
	}
	for _, register := range group.Register {
		err := register(ctx, mux, endpoint, dialOpts)
		if err != nil {
			return nil, fmt.Errorf("register gateway handler: %w", err)
		}
	}
	return mux, nil
}

func mountGroup(root *http.ServeMux, prefix string, mux *runtime.ServeMux) {
	if prefix == "" || prefix == "/" {
		root.Handle("/", mux)
		return
	}
	prefix = normalizePrefix(prefix)
	root.Handle(prefix+"/", http.StripPrefix(prefix, mux))
}

func normalizePrefix(prefix string) string {
	prefix = "/" + strings.Trim(prefix, "/")
	if prefix == "/" {
		return ""
	}
	return prefix
}

// Addr returns the configured listen address.
func (s *Server) Addr() string {
	return s.addr
}

// Serve starts the HTTP server.
func (s *Server) Serve() error {
	return s.server.Serve(s.lis)
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
