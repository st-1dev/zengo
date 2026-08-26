package errx_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"zengo/platform/sdk/errx"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
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

func newErrorWithInternalStackMarker() *errx.Error {
	return errx.New(codes.Unavailable, "internal-message-secret-marker", errx.Public("service temporarily unavailable"), errx.Fields(errx.Field{Key: "field-secret-marker", Value: "value-secret-marker"}))
}

func TestGRPCStatusDoesNotExposeInternalDiagnostics(t *testing.T) {
	appErr := newErrorWithInternalStackMarker()
	if !strings.Contains(strings.Join(appErr.StackTrace(), "\n"), "newErrorWithInternalStackMarker") {
		t.Fatal("expected locally captured stack marker")
	}
	if appErr.Message() != "internal-message-secret-marker" {
		t.Fatalf("local message = %q", appErr.Message())
	}
	if fields := appErr.Fields(); len(fields) != 1 || fields[0] != (errx.Field{Key: "field-secret-marker", Value: "value-secret-marker"}) {
		t.Fatalf("local fields = %#v", fields)
	}
	st := appErr.GRPCStatus()
	if st.Code() != codes.Unavailable {
		t.Fatalf("code = %v", st.Code())
	}
	if st.Message() != "service temporarily unavailable" {
		t.Fatalf("message = %q", st.Message())
	}
	var info *errdetails.ErrorInfo
	for _, detail := range st.Details() {
		switch typed := detail.(type) {
		case *errdetails.ErrorInfo:
			info = typed
		case *errdetails.DebugInfo:
			t.Fatalf("unexpected DebugInfo: %#v", typed)
		default:
			t.Fatalf("unexpected status detail: %T", typed)
		}
	}
	if info == nil {
		t.Fatal("expected ErrorInfo")
	}
	if info.Reason != "APPLICATION_ERROR" || info.Domain != "zengo.platform/sdk/errx" {
		t.Fatalf("ErrorInfo = %#v", info)
	}
	if len(info.Metadata) != 0 {
		t.Fatalf("metadata = %#v", info.Metadata)
	}
}

func TestGRPCRoundTripPreservesPublicDetails(t *testing.T) {
	roundTrip := errx.FromError(newErrorWithInternalStackMarker().GRPCStatus().Err())
	if roundTrip.Code() != codes.Unavailable {
		t.Fatalf("code = %v", roundTrip.Code())
	}
	if roundTrip.PublicMessage() != "service temporarily unavailable" {
		t.Fatalf("public message = %q", roundTrip.PublicMessage())
	}
	if roundTrip.Message() != "service temporarily unavailable" {
		t.Fatalf("message = %q", roundTrip.Message())
	}
	if fields := roundTrip.Fields(); len(fields) != 0 {
		t.Fatalf("fields = %#v", fields)
	}
	if stack := roundTrip.StackTrace(); len(stack) != 0 {
		t.Fatalf("stack = %#v", stack)
	}
}

func TestFromErrorDecodesLegacyGRPCDetails(t *testing.T) {
	st := grpcstatus.New(codes.InvalidArgument, "legacy public message")
	legacy, err := st.WithDetails(&errdetails.ErrorInfo{Reason: "APPLICATION_ERROR", Domain: "zengo.platform/sdk/errx", Metadata: map[string]string{"message": "legacy internal message", "public_message": "legacy public message", "field.field": "email"}}, &errdetails.DebugInfo{StackEntries: []string{"legacy stack entry"}, Detail: "legacy debug detail"})
	if err != nil {
		t.Fatalf("build legacy status: %v", err)
	}
	decoded := errx.FromError(legacy.Err())
	if decoded.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v", decoded.Code())
	}
	if decoded.PublicMessage() != "legacy public message" {
		t.Fatalf("public message = %q", decoded.PublicMessage())
	}
	if decoded.Message() != "legacy internal message" {
		t.Fatalf("message = %q", decoded.Message())
	}
	if fields := decoded.Fields(); len(fields) != 1 || fields[0] != (errx.Field{Key: "field", Value: "email"}) {
		t.Fatalf("fields = %#v", fields)
	}
	if stack := decoded.StackTrace(); len(stack) != 1 || stack[0] != "legacy stack entry" {
		t.Fatalf("stack = %#v", stack)
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
	if gotMessage != "invalid request" {
		t.Fatalf("message = %q", gotMessage)
	}
	fields := appErr.Fields()
	if len(fields) != 0 {
		t.Fatalf("fields = %#v", fields)
	}
	if stack := appErr.StackTrace(); len(stack) != 0 {
		t.Fatalf("stack = %#v", stack)
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
