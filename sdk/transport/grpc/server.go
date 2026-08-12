package grpc

import (
	"context"
	"fmt"
	"net"
	"zengo/platform/sdk/tlsconfig"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

var listen = net.Listen

// Server serves a configured gRPC server on a TCP listener.
type Server struct {
	grpc *grpc.Server
	lis  net.Listener
	addr string
}

// Options controls gRPC transport setup.
type Options struct {
	// Addr is the TCP listen address passed to net.Listen.
	Addr string
	// Server is the preconfigured gRPC server to expose. When nil, New creates one.
	Server *grpc.Server
	// TLS configures TLS for servers created by New when Server is nil.
	TLS *tlsconfig.ServerOptions
	// EnableReflection registers the gRPC reflection service on the server.
	EnableReflection bool
}

// New creates a gRPC server and binds a listener.
func New(opts Options) (*Server, error) {
	srv := opts.Server
	if srv == nil {
		serverOpts := make([]grpc.ServerOption, 0, 1)
		if opts.TLS != nil {
			tlsCfg, err := tlsconfig.ServerConfig(opts.TLS)
			if err != nil {
				return nil, fmt.Errorf("grpc server tls: %w", err)
			}
			serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
		}
		srv = grpc.NewServer(serverOpts...)
	} else if opts.TLS != nil {
		return nil, fmt.Errorf("grpc tls must be configured when constructing the provided grpc.Server")
	}
	if opts.EnableReflection {
		reflection.Register(srv)
	}
	lis, err := listen("tcp", opts.Addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", opts.Addr, err)
	}
	return &Server{
		grpc: srv,
		lis:  lis,
		addr: opts.Addr,
	}, nil
}

// GRPC returns the underlying gRPC server for handler registration.
func (s *Server) GRPC() *grpc.Server {
	return s.grpc
}

// Addr returns the configured listen address.
func (s *Server) Addr() string {
	return s.addr
}

// Serve starts serving on the bound listener.
func (s *Server) Serve() error {
	return s.grpc.Serve(s.lis)
}

// Shutdown gracefully stops the server, forcing Stop only when the context expires.
func (s *Server) Shutdown(ctx context.Context) error {
	stopped := make(chan struct{})
	go func() {
		s.grpc.GracefulStop()
		close(stopped)
	}()
	select {
	case <-ctx.Done():
		s.grpc.Stop()
		return ctx.Err()
	case <-stopped:
		return nil
	}
}
