package errx

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

const (
	errorInfoDomain = "zengo.platform/sdk/errx"
	errorInfoReason = "APPLICATION_ERROR"

	metadataMessage       = "message"
	metadataPublicMessage = "public_message"
	metadataFieldPrefix   = "field."
)

// Field is a structured key/value pair attached to an error.
type Field struct {
	// Key is the stable field name attached to the error.
	Key string
	// Value is the serialized field value attached to the error.
	Value string
}

// Error is the canonical application error for Go and gRPC transports.
type Error struct {
	code          codes.Code
	message       string
	publicMessage string
	fields        []Field
	stack         []string
	cause         error
}

type options struct {
	publicMessage string
	fields        []Field
	stackSkip     int
}

// Option configures Error construction.
type Option func(*options)

// Public sets the message intended for end users or external clients.
func Public(message string) Option {
	return func(o *options) {
		o.publicMessage = message
	}
}

// Fields attaches structured key/value pairs to the error.
func Fields(fields ...Field) Option {
	return func(o *options) {
		o.fields = mergeFields(o.fields, fields)
	}
}

// StackSkip skips additional frames when capturing the stack.
func StackSkip(skip int) Option {
	return func(o *options) {
		if skip > 0 {
			o.stackSkip += skip
		}
	}
}

// New creates a new rich application error with a captured stack trace.
func New(code codes.Code, message string, opts ...Option) *Error {
	cfg := applyOptions(opts)
	err := &Error{
		code:          normalizeCode(code),
		message:       message,
		publicMessage: cfg.publicMessage,
		fields:        mergeFields(nil, cfg.fields),
		stack:         captureStack(3 + cfg.stackSkip),
	}
	finalizeMessages(err)
	return err
}

// Wrap decorates another error with code, messages, fields, and a new stack trace.
func Wrap(err error, code codes.Code, message string, opts ...Option) *Error {
	if err == nil {
		return nil
	}
	cfg := applyOptions(opts)
	base := FromError(err)
	out := &Error{
		code:          pickCode(code, base.code),
		message:       message,
		publicMessage: cfg.publicMessage,
		fields:        mergeFields(base.fields, cfg.fields),
		stack:         captureStack(3 + cfg.stackSkip),
		cause:         err,
	}
	if out.message == "" {
		out.message = base.message
	}
	if out.publicMessage == "" {
		out.publicMessage = base.publicMessage
	}
	finalizeMessages(out)
	return out
}

// Normalize converts arbitrary errors into Error without adding a new stack.
//
// Use Normalize when an API expects *Error semantics but should preserve the
// existing error payload and stack information.
func Normalize(err error, opts ...Option) *Error {
	if err == nil {
		return nil
	}
	cfg := applyOptions(opts)
	base := FromError(err)
	if len(cfg.fields) == 0 {
		return base
	}
	out := base.clone()
	out.fields = mergeFields(out.fields, cfg.fields)
	finalizeMessages(out)
	return out
}

// FromError extracts Error data from a regular Go error or a gRPC status error.
//
// The returned value is a copy, so callers can inspect it without mutating the
// original error chain.
func FromError(err error) *Error {
	if err == nil {
		return nil
	}
	appErr, ok := errors.AsType[*Error](err)
	if ok {
		return appErr.clone()
	}
	var st *grpcstatus.Status
	st, ok = grpcstatus.FromError(err)
	if ok {
		return fromStatus(st, err)
	}
	out := &Error{
		code:    codeFromError(err),
		message: err.Error(),
		cause:   err,
	}
	finalizeMessages(out)
	return out
}

// Code returns the gRPC code associated with err.
func Code(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	return FromError(err).code
}

// Message returns the primary internal message for err.
func Message(err error) string {
	if err == nil {
		return ""
	}
	return FromError(err).message
}

// PublicMessage returns the public-facing message for err.
func PublicMessage(err error) string {
	if err == nil {
		return ""
	}
	return FromError(err).publicMessage
}

// FieldsOf returns a copy of the structured fields attached to err.
func FieldsOf(err error) []Field {
	if err == nil {
		return nil
	}
	return FromError(err).Fields()
}

// StackTrace returns a copy of the captured stack entries.
func StackTrace(err error) []string {
	if err == nil {
		return nil
	}
	stack := FromError(err).stack
	if len(stack) == 0 {
		return nil
	}
	return append([]string(nil), stack...)
}

// UnaryServerInterceptor normalizes outbound gRPC errors and preserves rich details.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}
		appErr := serverError(err)
		if appErr != nil {
			return resp, appErr
		}
		return resp, err
	}
}

// UnaryClientInterceptor decodes rich gRPC errors into Error on the caller side.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err == nil {
			return nil
		}
		return Normalize(err)
	}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	msg := e.message
	if msg == "" {
		msg = e.publicMessage
	}
	if msg == "" && e.cause != nil {
		msg = e.cause.Error()
	}
	if e.cause == nil {
		return msg
	}
	cause := e.cause.Error()
	if cause == "" || cause == msg {
		return msg
	}
	if msg == "" {
		return cause
	}
	return msg + ": " + cause
}

// Unwrap returns the wrapped cause.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Code returns the gRPC status code associated with Error.
func (e *Error) Code() codes.Code {
	if e == nil {
		return codes.OK
	}
	return e.code
}

// Message returns the primary internal message stored in Error.
func (e *Error) Message() string {
	if e == nil {
		return ""
	}
	return e.message
}

// PublicMessage returns the public-facing message stored in Error.
func (e *Error) PublicMessage() string {
	if e == nil {
		return ""
	}
	return e.publicMessage
}

// Fields returns the structured fields stored in Error.
func (e *Error) Fields() []Field {
	if e == nil || len(e.fields) == 0 {
		return nil
	}
	return append([]Field(nil), e.fields...)
}

// StackTrace returns the captured stack for Error.
func (e *Error) StackTrace() []string {
	if e == nil || len(e.stack) == 0 {
		return nil
	}
	return append([]string(nil), e.stack...)
}

// GRPCStatus exposes Error as a gRPC status with structured details.
func (e *Error) GRPCStatus() *grpcstatus.Status {
	if e == nil {
		return grpcstatus.New(codes.OK, "")
	}
	st := grpcstatus.New(normalizeCode(e.code), e.publicMessage)
	withInfo, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason:   errorInfoReason,
		Domain:   errorInfoDomain,
		Metadata: fieldMetadata(e),
	})
	if err != nil {
		return st
	}
	if len(e.stack) > 0 || e.message != "" {
		withDebug, err := withInfo.WithDetails(&errdetails.DebugInfo{
			StackEntries: append([]string(nil), e.stack...),
			Detail:       e.message,
		})
		if err != nil {
			return withInfo
		}
		return withDebug
	}
	return withInfo
}

func fieldMetadata(e *Error) map[string]string {
	metadata := map[string]string{
		metadataMessage:       e.message,
		metadataPublicMessage: e.publicMessage,
	}
	for _, field := range e.fields {
		if field.Key == "" {
			continue
		}
		metadata[metadataFieldPrefix+field.Key] = field.Value
	}
	return metadata
}

func applyOptions(opts []Option) options {
	cfg := options{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

func serverError(err error) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		out := appErr.clone()
		finalizeMessages(out)
		return out
	}
	st, ok := grpcstatus.FromError(err)
	if ok {
		if hasErrxDetails(st) {
			out := fromStatus(st, err)
			finalizeMessages(out)
			return out
		}
		return Wrap(err, st.Code(), st.Message(), Public(st.Message()), StackSkip(1))
	}
	return Wrap(err, codeFromError(err), err.Error(), StackSkip(1))
}

func fromStatus(st *grpcstatus.Status, cause error) *Error {
	out := &Error{
		code:          normalizeCode(st.Code()),
		message:       st.Message(),
		publicMessage: st.Message(),
		cause:         cause,
	}
	for _, detail := range st.Details() {
		switch typed := detail.(type) {
		case *errdetails.ErrorInfo:
			if typed.Domain != errorInfoDomain || typed.Reason != errorInfoReason {
				continue
			}
			msg := typed.Metadata[metadataMessage]
			if msg != "" {
				out.message = msg
			}
			publicMsg := typed.Metadata[metadataPublicMessage]
			if publicMsg != "" {
				out.publicMessage = publicMsg
			}
			out.fields = decodeFields(typed.Metadata)
		case *errdetails.DebugInfo:
			if len(typed.StackEntries) > 0 {
				out.stack = append([]string(nil), typed.StackEntries...)
			}
			if out.message == "" && typed.Detail != "" {
				out.message = typed.Detail
			}
		}
	}
	finalizeMessages(out)
	return out
}

func hasErrxDetails(st *grpcstatus.Status) bool {
	for _, detail := range st.Details() {
		info, ok := detail.(*errdetails.ErrorInfo)
		if !ok {
			continue
		}
		if info.Domain == errorInfoDomain && info.Reason == errorInfoReason {
			return true
		}
	}
	return false
}

func pickCode(preferred, fallback codes.Code) codes.Code {
	if preferred != codes.OK {
		return normalizeCode(preferred)
	}
	return normalizeCode(fallback)
}

func normalizeCode(code codes.Code) codes.Code {
	if code == codes.OK {
		return codes.Internal
	}
	return code
}

func finalizeMessages(err *Error) {
	if err.message == "" && err.cause != nil {
		err.message = err.cause.Error()
	}
	if err.publicMessage == "" {
		err.publicMessage = defaultPublicMessage(err.code)
	}
	if err.message == "" {
		err.message = err.publicMessage
	}
}

func defaultPublicMessage(code codes.Code) string {
	switch normalizeCode(code) {
	case codes.InvalidArgument:
		return "invalid request"
	case codes.NotFound:
		return "resource not found"
	case codes.AlreadyExists:
		return "resource already exists"
	case codes.PermissionDenied:
		return "permission denied"
	case codes.Unauthenticated:
		return "authentication required"
	case codes.DeadlineExceeded:
		return "request timeout"
	case codes.Unavailable:
		return "service temporarily unavailable"
	case codes.ResourceExhausted:
		return "resource limit exceeded"
	case codes.Canceled:
		return "request canceled"
	default:
		return "internal server error"
	}
}

func codeFromError(err error) codes.Code {
	switch {
	case err == nil:
		return codes.OK
	case errors.Is(err, context.Canceled):
		return codes.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return codes.DeadlineExceeded
	}
	st, ok := grpcstatus.FromError(err)
	if ok {
		return st.Code()
	}
	return codes.Internal
}

func decodeFields(metadata map[string]string) []Field {
	if len(metadata) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		if strings.HasPrefix(key, metadataFieldPrefix) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	fields := make([]Field, 0, len(keys))
	for _, key := range keys {
		name := strings.TrimPrefix(key, metadataFieldPrefix)
		if name == "" {
			continue
		}
		fields = append(fields, Field{Key: name, Value: metadata[key]})
	}
	return fields
}

func mergeFields(base, extra []Field) []Field {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := make([]Field, 0, len(base)+len(extra))
	index := map[string]int{}

	for _, field := range base {
		if field.Key == "" {
			continue
		}
		index[field.Key] = len(out)
		out = append(out, field)
	}

	for _, field := range extra {
		if field.Key == "" {
			continue
		}
		idx, ok := index[field.Key]
		if ok {
			out[idx] = field
			continue
		}
		index[field.Key] = len(out)
		out = append(out, field)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func captureStack(skip int) []string {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(skip, pcs)
	if n == 0 {
		return nil
	}
	pcs = pcs[:n]
	frames := runtime.CallersFrames(pcs)
	stack := make([]string, 0, n)
	for {
		frame, more := frames.Next()
		if frame.Function != "" {
			stack = append(stack, fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function))
		}
		if !more {
			break
		}
	}
	return stack
}

func (e *Error) clone() *Error {
	if e == nil {
		return nil
	}
	out := *e
	if len(e.fields) > 0 {
		out.fields = append([]Field(nil), e.fields...)
	}
	if len(e.stack) > 0 {
		out.stack = append([]string(nil), e.stack...)
	}
	return &out
}
