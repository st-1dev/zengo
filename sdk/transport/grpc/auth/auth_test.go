package auth_test

import (
	"context"
	"testing"
	"zengo/platform/sdk/transport/grpc/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestUnaryServerInterceptor(t *testing.T) {
	interceptor := auth.UnaryServerInterceptor(auth.Config{
		Enabled: true,
		APIKeys: map[string]string{"dev-token": "dev"},
	})
	handler := func(context.Context, any) (any, error) { return "ok", nil }
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer dev-token"))
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/GetUser"}, handler)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	ctxBad := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer bad"))
	_, err = interceptor(ctxBad, nil, &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/GetUser"}, handler)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}
}
