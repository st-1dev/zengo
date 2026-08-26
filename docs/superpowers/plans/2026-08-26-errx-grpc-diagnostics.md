# errx gRPC Diagnostics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep internal `errx.Error` diagnostics local while outgoing gRPC statuses retain only their public message and stable `ErrorInfo` identity.

**Architecture:** `(*errx.Error).GRPCStatus` will create the status from the existing normalized code and `publicMessage`, then attach an `ErrorInfo` containing only the stable reason and domain. The inbound `fromStatus` decoder and its legacy metadata constants remain unchanged so it can continue to read old peers' `ErrorInfo.Metadata` and `DebugInfo` payloads.

**Tech Stack:** Go 1.26, `google.golang.org/grpc`, `google.golang.org/genproto/googleapis/rpc/errdetails`, Go standard `testing` package.

**Spec:** `docs/superpowers/specs/2026-08-26-errx-grpc-diagnostics-design.md`

**Issue:** https://github.com/st-1dev/zengo/issues/2

## Global Constraints

- Public functions and types remain unchanged.
- The outgoing status message is derived only from `PublicMessage`.
- Outgoing `ErrorInfo` retains `Reason: "APPLICATION_ERROR"` and `Domain: "zengo.platform/sdk/errx"` and contains no metadata.
- Outgoing statuses contain no `DebugInfo`, internal message, structured fields, stack trace, or copied public message metadata.
- Incoming legacy metadata keys and `DebugInfo` remain decodable; do not remove `metadataMessage`, `metadataPublicMessage`, or `metadataFieldPrefix`.
- Do not change local error logging or stack capture, add an opt-in mode, add public types, or add dependencies.

---

## Files

- Modify: `sdk/errx/errx.go` — make `(*Error).GRPCStatus` emit the minimal public wire payload and remove its now-unused metadata builder.
- Modify: `sdk/errx/errx_test.go` — assert the new wire contract, new-status round trip, and legacy inbound decoding.

### Task 1: Emit only the public errx gRPC status and preserve legacy decoding

**Files:**
- Modify: `sdk/errx/errx.go:302-341`
- Test: `sdk/errx/errx_test.go:3-198`

**Interfaces:**
- Consumes: `func (e *Error) GRPCStatus() *grpcstatus.Status`, `func FromError(err error) *Error`, `func fromStatus(st *grpcstatus.Status, cause error) *Error`.
- Produces: outgoing `*grpcstatus.Status` with `codes.Code`, `Status.Message()`, and exactly one `*errdetails.ErrorInfo` containing the stable reason/domain; legacy incoming payloads still produce `*errx.Error` with their message, fields, and stack.

- [ ] **Step 1: Replace unsafe round-trip expectations and add outbound and legacy-wire regression tests**

Add `"strings"` and `"google.golang.org/genproto/googleapis/rpc/errdetails"` to `sdk/errx/errx_test.go` imports. Replace `TestGRPCRoundTripPreservesDetails` and `TestUnaryClientInterceptorDecodesRemoteErrx`, then add the following helpers/tests:

```go
func newErrorWithInternalStackMarker() *errx.Error {
	return errx.New(
		codes.Unavailable,
		"internal-message-secret-marker",
		errx.Public("service temporarily unavailable"),
		errx.Fields(errx.Field{Key: "field-secret-marker", Value: "value-secret-marker"}),
	)
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
	legacy, err := st.WithDetails(
		&errdetails.ErrorInfo{
			Reason: "APPLICATION_ERROR",
			Domain: "zengo.platform/sdk/errx",
			Metadata: map[string]string{
				"message":        "legacy internal message",
				"public_message": "legacy public message",
				"field.field":    "email",
			},
		},
		&errdetails.DebugInfo{
			StackEntries: []string{"legacy stack entry"},
			Detail:       "legacy debug detail",
		},
	)
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
	if appErr.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v", appErr.Code())
	}
	if appErr.PublicMessage() != "invalid request" {
		t.Fatalf("public message = %q", appErr.PublicMessage())
	}
	if appErr.Message() != "invalid request" {
		t.Fatalf("message = %q", appErr.Message())
	}
	if fields := appErr.Fields(); len(fields) != 0 {
		t.Fatalf("fields = %#v", fields)
	}
	if stack := appErr.StackTrace(); len(stack) != 0 {
		t.Fatalf("stack = %#v", stack)
	}
}
```

- [ ] **Step 2: Run the focused tests to verify the current outbound encoding fails**

Run: `go test ./sdk/errx -run '^(TestGRPCStatusDoesNotExposeInternalDiagnostics|TestGRPCRoundTripPreservesPublicDetails|TestFromErrorDecodesLegacyGRPCDetails|TestUnaryClientInterceptorDecodesRemoteErrx)$' -count=1`

Expected: RED — outbound and client-interceptor tests expose the current internal metadata/stack; the legacy decoder test is already green.

- [ ] **Step 3: Replace the outbound details construction with the minimal public payload**

In `sdk/errx/errx.go`, replace `GRPCStatus` with this implementation and delete `fieldMetadata`. Keep the three `metadata*` constants because `fromStatus` and `decodeFields` still decode legacy inbound statuses.

```go
func (e *Error) GRPCStatus() *grpcstatus.Status {
	if e == nil {
		return grpcstatus.New(codes.OK, "")
	}
	st := grpcstatus.New(normalizeCode(e.code), e.publicMessage)
	withInfo, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason: errorInfoReason,
		Domain: errorInfoDomain,
	})
	if err != nil {
		return st
	}
	return withInfo
}
```

Run: `gofmt -w sdk/errx/errx.go sdk/errx/errx_test.go`

Expected: `gofmt` completes without output.

- [ ] **Step 4: Run the focused tests to verify the public-only wire contract**

Run: `go test ./sdk/errx -run '^(TestGRPCStatusDoesNotExposeInternalDiagnostics|TestGRPCRoundTripPreservesPublicDetails|TestFromErrorDecodesLegacyGRPCDetails|TestUnaryClientInterceptorDecodesRemoteErrx)$' -count=1`

Expected: GREEN — the outgoing status has only public code/message and stable `ErrorInfo`; a new-status decode restores no internal fields or stack; the manually built legacy status still restores them.

- [ ] **Step 5: Run package and repository verification**

Run: `go test ./sdk/errx -count=1`

Expected: GREEN — all errx tests pass.

Run: `go test ./...`

Expected: GREEN — all repository tests pass.

- [ ] **Step 6: Commit the implementation**

```bash
git add sdk/errx/errx.go sdk/errx/errx_test.go
git commit -m "fix(errx): hide internal gRPC diagnostics"
```
