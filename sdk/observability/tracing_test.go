package observability_test

import (
	"context"
	"testing"
	"zengo/platform/sdk/observability"

	"go.opentelemetry.io/otel/trace"
)

func TestInitTracingStdout(t *testing.T) {
	ctx := context.Background()
	shutdown, err := observability.Init(ctx, observability.Options{
		ServiceName: "test",
		Tracing:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = shutdown(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func TestKafkaHeaderPropagation(t *testing.T) {
	ctx := context.Background()
	shutdown, err := observability.Init(ctx, observability.Options{
		ServiceName: "test",
		Tracing:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := shutdown(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}()
	spanCtx, endSpan := observability.StartSpan(
		context.Background(),
		observability.StringAttribute("messaging.system", "kafka"),
		observability.StringAttribute("messaging.operation", "publish"),
		observability.StringAttribute("messaging.destination.name", "users"),
	)
	ctx = spanCtx
	defer endSpan()

	sourceTraceID := trace.SpanContextFromContext(ctx).TraceID()
	if !sourceTraceID.IsValid() {
		t.Fatal("expected valid trace id on source context")
	}

	headers := observability.InjectKafkaHeaders(ctx, nil)
	if len(headers) == 0 {
		t.Fatal("expected trace headers")
	}

	extracted := observability.ExtractKafkaContext(context.Background(), headers)
	extractedTraceID := trace.SpanContextFromContext(extracted).TraceID()
	if extractedTraceID != sourceTraceID {
		t.Fatalf("trace id mismatch: got %v want %v", extractedTraceID, sourceTraceID)
	}
}

func TestInitMetricsOnlyShutdown(t *testing.T) {
	ctx := context.Background()
	shutdown, err := observability.Init(ctx, observability.Options{
		ServiceName: "test",
		Metrics:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = shutdown(ctx)
	if err != nil {
		t.Fatal(err)
	}
}
