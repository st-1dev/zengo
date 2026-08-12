// Package grpc serves a preconfigured *grpc.Server on a TCP listener.
// OpenTelemetry server tracing is not enabled automatically: pass
// observability.GRPCServerOptions() to grpc.NewServer in main when tracing is on.
package grpc
