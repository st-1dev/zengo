package observability

import (
	"context"
	"runtime"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SpanEndFunc closes a span started by StartSpan or StartSpanNamed.
type SpanEndFunc func()

// StartSpan starts a child span from the current span in ctx.
// If ctx has no active span, it starts a new root span.
//
// The span name is derived automatically from the call stack.
// Use this helper in handwritten code when automatic naming is acceptable.
// Generated code can prefer StartSpanNamed to avoid stack inspection overhead.
//
// The returned context carries the new span.
// Call the returned SpanEndFunc to avoid leaving spans open.
func StartSpan(ctx context.Context, attributes ...Attribute) (spanCtx context.Context, spanEndFunc SpanEndFunc) {
	const skip = 3
	return StartSpanNamed(ctx, getSpanName(skip), attributes...)
}

// StartSpanNamed starts a child span from the current span in ctx.
// If ctx has no active span, it starts a new root span.
//
// The returned context carries the new span.
// Call the returned SpanEndFunc to avoid leaving spans open.
func StartSpanNamed(ctx context.Context, title string, attributes ...Attribute) (spanCtx context.Context, spanEndFunc SpanEndFunc) {
	tr := loadTracer()
	if tr == nil {
		return ctx, func() {}
	}

	var opts []trace.SpanStartOption
	attrs := transformAttributes(attributes)
	if attrs != nil {
		opts = append(opts, trace.WithAttributes(attrs...))
	}

	var span trace.Span
	spanCtx, span = tr.Start(ctx, title, opts...)

	return spanCtx, func() { span.End() }
}

// DoSpan starts a span with the provided title, runs action with the span
// context, and closes the span afterwards.
//
// action must not be nil or the call will panic.
func DoSpan(ctx context.Context, title string, attributes []Attribute, action func(context.Context)) {
	spanCtx, endFunc := StartSpanNamed(ctx, title, attributes...)
	defer endFunc()

	action(spanCtx)
}

// AddEvent adds an event to the current span.
// The event is timestamped automatically when it is added.
// Spans should represent coarse-grained operations, while events should
// capture fine-grained activity inside them.
//
// The event name is derived automatically from the call stack.
// Use this helper in handwritten code when automatic naming is acceptable.
// Generated code can prefer AddEventNamed to avoid stack inspection overhead.
func AddEvent(ctx context.Context, attributes ...Attribute) {
	const skip = 3
	AddEventNamed(ctx, getSpanName(skip), attributes...)
}

// AddEventNamed adds a named event to the current span.
// The event is timestamped automatically when it is added.
// Spans should represent coarse-grained operations, while events should
// capture fine-grained activity inside them.
func AddEventNamed(ctx context.Context, title string, attributes ...Attribute) {
	span := trace.SpanFromContext(ctx)

	var opts []trace.EventOption
	attrs := transformAttributes(attributes)
	if attrs != nil {
		opts = append(opts, trace.WithAttributes(attrs...))
	}

	span.AddEvent(title, opts...)
}

// RecordException records an exception event on the current span.
// The emitted event name is "exception".
//
// Supported event attributes:
// Attribute				Type		Required
// exception.type			string		see below
// exception.message		string		see below
// exception.stacktrace		string		no
// exception.escaped 		boolean		no
//
// At least one of the following attribute sets is required:
// * exception.type
// * exception.message.
func RecordException(ctx context.Context, err error, errDescription string, attributes ...Attribute) {
	if err == nil {
		return
	}

	span := trace.SpanFromContext(ctx)

	var opts []trace.EventOption
	attrs := transformAttributes(attributes)
	if attrs != nil {
		opts = append(opts, trace.WithAttributes(attrs...))
	}

	// RecordError converts the error into a span event.
	span.RecordError(err, opts...)
	// The whole operation failed, so update the span status as well.
	span.SetStatus(codes.Error, errDescription)
}

// replacement rewrites escaped fragments in function names extracted from the
// call stack. runtime.FuncForPC returns URI-escaped names for some symbols.
var replacement = map[string]string{
	"%2e": ".",
}

// getSpanName returns the function name for the call site that created the
// span. It skips the requested stack frames and applies replacement rewrites to
// the resolved function name.
func getSpanName(skip int) (name string) {
	const unknown = "unknown"

	var pc [1]uintptr
	if runtime.Callers(skip, pc[:]) == 0 {
		return unknown
	}

	f := runtime.FuncForPC(pc[0])
	if f == nil {
		return unknown
	}

	name = f.Name()
	for k, v := range replacement {
		name = strings.ReplaceAll(name, k, v)
	}
	return name
}

// transformAttributes converts a slice of Attribute values into a slice of
// OpenTelemetry key-value pairs. It returns nil for an empty input slice.
func transformAttributes(attributes []Attribute) (out []attribute.KeyValue) {
	if len(attributes) == 0 {
		return nil
	}

	out = make([]attribute.KeyValue, 0, len(attributes))
	for _, attr := range attributes {
		out = append(out, attr.transform())
	}
	return out
}
