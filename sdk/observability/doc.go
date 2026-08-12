// Package observability configures Prometheus metrics and OpenTelemetry tracing
// for Zengo services.
//
// Call Init early in main before opening data sources or starting servers.
// The returned shutdown function must run on exit (via app.AddCleanup) so spans
// and metrics are flushed.
//
// Tracing export order:
//  1. OTLP endpoint from tracing config spec (kind: tracing)
//  2. OTEL_EXPORTER_OTLP_ENDPOINT env var
//  3. stdout pretty-print exporter (development fallback)
//
// Local Jaeger (docker compose service jaeger) listens on localhost:4317 (gRPC).
// Match configs/tracing.yaml endpoint for local dev.
//
// Instrumentation helpers:
//   - GRPCServerOptions / GRPCDialOptions — pass to grpc.NewServer and gateway dial opts
//   - HTTPHandler — wrap the root http.Handler for REST tracing
//   - MetricsHandler — expose Prometheus /metrics
//   - InjectKafkaHeaders / ExtractKafkaContext — W3C trace context in Kafka headers
//
// Standard OTel env vars (OTEL_EXPORTER_OTLP_INSECURE, OTEL_TRACES_SAMPLER_ARG)
// are honored when set.
package observability
