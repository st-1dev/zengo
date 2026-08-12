package errx_test

import (
	"context"
	"errors"
	"testing"
	"zengo/platform/sdk/errx"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestWrapCapturesMessageAndPublicMessages(t *testing.T) {
	base := errors.New("db connection refused")

	appErr := errx.Wrap(
		base,
		codes.Internal,
		"load profile",
		errx.Public("could not load profile"),
		errx.Fields(
			errx.Field{Key: "user_id", Value: "42"},
			errx.Field{Key: "attempt", Value: "2"},
		),
	)
	gotCode := appErr.Code()
	if gotCode != codes.Internal {
		t.Fatalf("code = %v", gotCode)
	}
	gotPublicMessage := appErr.PublicMessage()
	if gotPublicMessage != "could not load profile" {
		t.Fatalf("public message = %q", gotPublicMessage)
	}
	gotMessage := appErr.Message()
	if gotMessage != "load profile" {
		t.Fatalf("message = %q", gotMessage)
	}
	gotError := appErr.Error()
	if gotError != "load profile: db connection refused" {
		t.Fatalf("error = %q", gotError)
	}
	if len(appErr.StackTrace()) == 0 {
		t.Fatal("expected stack trace")
	}
	fields := appErr.Fields()
	if len(fields) != 2 {
		t.Fatalf("fields = %#v", fields)
	}
	if fields[0] != (errx.Field{Key: "user_id", Value: "42"}) {
		t.Fatalf("field[0] = %#v", fields[0])
	}
	if fields[1] != (errx.Field{Key: "attempt", Value: "2"}) {
		t.Fatalf("field[1] = %#v", fields[1])
	}
}

func TestGRPCRoundTripPreservesDetails(t *testing.T) {
	appErr := errx.Wrap(
		errors.New("write timeout"),
		codes.Unavailable,
		"publish event",
		errx.Public("service temporarily unavailable"),
		errx.Fields(errx.Field{Key: "topic", Value: "users"}, errx.Field{Key: "retryable", Value: "true"}),
	)
	roundTrip := errx.FromError(appErr.GRPCStatus().Err())

	gotCode := roundTrip.Code()
	if gotCode != codes.Unavailable {
		t.Fatalf("code = %v", gotCode)
	}
	gotPublicMessage := roundTrip.PublicMessage()
	if gotPublicMessage != "service temporarily unavailable" {
		t.Fatalf("public message = %q", gotPublicMessage)
	}
	gotMessage := roundTrip.Message()
	if gotMessage != "publish event" {
		t.Fatalf("message = %q", gotMessage)
	}
	if len(roundTrip.StackTrace()) == 0 {
		t.Fatal("expected stack trace")
	}
	fields := roundTrip.Fields()
	if len(fields) != 2 {
		t.Fatalf("fields = %#v", fields)
	}
	if fields[0] != (errx.Field{Key: "retryable", Value: "true"}) {
		t.Fatalf("field[0] = %#v", fields[0])
	}
	if fields[1] != (errx.Field{Key: "topic", Value: "users"}) {
		t.Fatalf("fields = %#v", fields)
	}
}

func TestUnaryServerInterceptorNormalizesStatusErrors(t *testing.T) {
	interceptor := errx.UnaryServerInterceptor()

	_, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/GetUser"},
		func(context.Context, any) (any, error) {
			return nil, grpcstatus.Error(codes.NotFound, "user not found")
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	appErr := errx.FromError(err)
	gotCode := appErr.Code()
	if gotCode != codes.NotFound {
		t.Fatalf("code = %v", gotCode)
	}
	gotPublicMessage := appErr.PublicMessage()
	if gotPublicMessage != "user not found" {
		t.Fatalf("public message = %q", gotPublicMessage)
	}
	if len(appErr.StackTrace()) == 0 {
		t.Fatal("expected stack trace")
	}
}

func TestUnaryClientInterceptorDecodesRemoteErrx(t *testing.T) {
	interceptor := errx.UnaryClientInterceptor()
	remote := errx.New(
		codes.InvalidArgument,
		"validate create user request",
		errx.Public("invalid request"),
		errx.Fields(errx.Field{Key: "field", Value: "email"}),
	)

	err := interceptor(
		context.Background(),
		"/user.v1.UserService/CreateUser",
		nil,
		nil,
		nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			return remote.GRPCStatus().Err()
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	appErr := errx.FromError(err)
	gotCode := appErr.Code()
	if gotCode != codes.InvalidArgument {
		t.Fatalf("code = %v", gotCode)
	}
	gotPublicMessage := appErr.PublicMessage()
	if gotPublicMessage != "invalid request" {
		t.Fatalf("public message = %q", gotPublicMessage)
	}
	gotMessage := appErr.Message()
	if gotMessage != "validate create user request" {
		t.Fatalf("message = %q", gotMessage)
	}
	fields := appErr.Fields()
	if len(fields) != 1 || fields[0] != (errx.Field{Key: "field", Value: "email"}) {
		t.Fatalf("fields = %#v", fields)
	}
}

func TestWrapMergesFields(t *testing.T) {
	base := errx.New(
		codes.Internal,
		"db write failed",
		errx.Fields(
			errx.Field{Key: "repo", Value: "users"},
			errx.Field{Key: "op", Value: "insert"},
		),
	)

	wrapped := errx.Wrap(
		base,
		codes.Internal,
		"create user",
		errx.Fields(
			errx.Field{Key: "op", Value: "create_user"},
			errx.Field{Key: "email", Value: "demo@example.com"},
		),
	)
	fields := wrapped.Fields()
	if len(fields) != 3 {
		t.Fatalf("fields = %#v", fields)
	}
	if fields[0] != (errx.Field{Key: "repo", Value: "users"}) {
		t.Fatalf("field[0] = %#v", fields[0])
	}
	if fields[1] != (errx.Field{Key: "op", Value: "create_user"}) {
		t.Fatalf("field[1] = %#v", fields[1])
	}
	if fields[2] != (errx.Field{Key: "email", Value: "demo@example.com"}) {
		t.Fatalf("field[2] = %#v", fields[2])
	}
}
