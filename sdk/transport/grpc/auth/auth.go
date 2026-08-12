package auth

import (
	"context"
	"strings"
	"zengo/platform/sdk/errx"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

// Config controls API-key based gRPC authentication.
type Config struct {
	// Enabled toggles the interceptor on or off.
	Enabled bool
	// APIKeys maps accepted API key values to stable logical names.
	APIKeys map[string]string
}

// UnaryServerInterceptor authenticates incoming gRPC requests against Config.
//
// The interceptor skips checks when authentication is disabled and for gRPC
// health endpoints.
func UnaryServerInterceptor(cfg Config) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !cfg.Enabled || strings.HasPrefix(info.FullMethod, "/grpc.health") {
			return handler(ctx, req)
		}
		err := authorize(ctx, cfg)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func authorize(ctx context.Context, cfg Config) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errx.New(
			codes.Unauthenticated,
			"missing authentication metadata",
			errx.Public("authentication required"),
		)
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return errx.New(
			codes.Unauthenticated,
			"missing authorization header",
			errx.Public("authentication required"),
		)
	}
	token := strings.TrimPrefix(values[0], "Bearer ")
	if token == values[0] {
		token = strings.TrimPrefix(values[0], "ApiKey ")
	}
	_, ok = cfg.APIKeys[token]
	if !ok {
		return errx.New(
			codes.PermissionDenied,
			"invalid api key",
			errx.Public("access denied"),
		)
	}
	return nil
}
