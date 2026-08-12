package generator_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"zengo/platform/internal/generator"
	"zengo/platform/internal/manifest"
)

func TestGenerateMainTracingOnly(t *testing.T) {
	m := baseManifest()
	m.Observability = manifest.Observability{Tracing: true}
	m.Transports = manifest.Transports{GRPC: &manifest.GRPC{Port: 9090}}

	path := filepath.Join(t.TempDir(), "main.go")
	err := generator.GenerateMain(m, path)
	if err != nil {
		t.Fatal(err)
	}
	src := readFile(t, path)
	for _, want := range []string{
		"service.New(ctx, service.Options{",
		"Tracing:",
		"true",
		"service.NewGRPCServerWithOptions(",
		"EnableTracing: true",
		`RegisterHubGRPC(grpcServer, hubHandler)`,
		"tracingConfig(cfgLoader)",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("generated main missing %q", want)
		}
	}
	if strings.Contains(src, "Metrics:     true") {
		t.Fatal("metrics should be disabled")
	}
	if strings.Contains(src, "promhttp") {
		t.Fatal("unexpected promhttp import")
	}
}

func TestGenerateMainMetricsAndTracing(t *testing.T) {
	m := baseManifest()
	m.Observability = manifest.Observability{
		Metrics:           true,
		Tracing:           true,
		Health:            true,
		TracingConfigFrom: "otel",
		LoggingConfigFrom: "app-logging",
	}
	m.Transports = manifest.Transports{
		GRPC: &manifest.GRPC{Port: 9090},
		REST: &manifest.REST{Port: 8080},
	}
	m.Queue = &manifest.Queue{
		Kafka: &manifest.Kafka{BrokersFromConfig: "kafka", Brokers: []string{"localhost:9092"}},
	}

	path := filepath.Join(t.TempDir(), "main.go")
	err := generator.GenerateMain(m, path)
	if err != nil {
		t.Fatal(err)
	}
	src := readFile(t, path)
	for _, want := range []string{
		"consumer.Close()",
		`flag.Parse()`,
		`flag.Bool("version"`,
		`service.PrintVersion(os.Stdout, "test")`,
		`service.New(ctx, service.Options{`,
		`service.NewGRPCServerWithOptions(`,
		`EnableTracing: true`,
		`restGroups := make([]service.RESTRouteGroup, 0, 2)`,
		`RegisterHubGRPC(grpcServer, hubHandler)`,
		`HubGatewayRegisterFuncs()`,
		"loggingConfig(cfgLoader)",
		`loader.Tracing("otel")`,
		`loader.Logging("app-logging")`,
		"kafka.ConfigFromLoader",
		"kafka.Brokers(kafkaCfg)",
		`REST: &service.REST{`,
		`Readiness: map[string]service.Check{`,
		`Cleanup: []service.Hook{`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("generated main missing %q", want)
		}
	}
	for _, unwanted := range []string{
		`buildinfo.Handler(`,
		`observability.MetricsHandler()`,
		`healthHandler.`,
		`app.New(`,
		`gateway.New(`,
		`envOr(`,
		`POSTGRES_HOST`,
	} {
		if strings.Contains(src, unwanted) {
			t.Fatalf("generated main should not contain %q", unwanted)
		}
	}
}

func TestGenerateMainCompiles(t *testing.T) {
	m := baseManifest()
	m.Observability = manifest.Observability{Metrics: true, Tracing: true, Health: true}
	m.Transports = manifest.Transports{
		GRPC: &manifest.GRPC{Port: 9090},
		REST: &manifest.REST{Port: 8080},
	}
	m.DB = &manifest.DB{Postgres: &manifest.PostgresDB{}}
	m.Queue = &manifest.Queue{
		Kafka: &manifest.Kafka{BrokersFromConfig: "kafka"},
	}

	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.go")
	err := generator.GenerateMain(m, mainPath)
	if err != nil {
		t.Fatal(err)
	}

	stubDir := filepath.Join(dir, "gen", "zengo")
	err = os.MkdirAll(stubDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	stub := `package zengo

import (
	"context"

	"zengo/platform/sdk/gateway"
	"zengo/platform/sdk/transport/queue/kafka"
	"zengo/platform/sdk/router"
	"google.golang.org/grpc"
)

type Dependencies struct {
	KafkaBrokers []string
	Producer     *kafka.Producer
}

func RegisterGRPC(*grpc.Server, any) {}
func RegisterHubGRPC(*grpc.Server, any) {}

func GatewayRegisterFuncs() []gateway.RegisterFunc {
	return nil
}

func HubGatewayRegisterFuncs() []gateway.RegisterFunc {
	return nil
}

	func RegisterKafka(*kafka.Consumer, Dependencies, func(context.Context, router.EventEnvelope) error) error {
		return nil
	}
	`
	err = os.WriteFile(filepath.Join(stubDir, "runtime_gen.go"), []byte(stub), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	handlerDir := filepath.Join(dir, "internal", "test")
	err = os.MkdirAll(handlerDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	handlerStub := `package test

import (
	"compiletest/gen/zengo"
	"zengo/platform/sdk/transport/queue/kafka"
)

type Repository struct{}

func NewRepository(any) *Repository { return &Repository{} }

type Handler struct{}

func NewHandler(*Repository, *kafka.Producer, []string) *Handler { return &Handler{} }

	func RegisterKafka(*kafka.Consumer, zengo.Dependencies, *Handler) error {
		return nil
	}
	`
	err = os.WriteFile(filepath.Join(handlerDir, "handler.go"), []byte(handlerStub), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	platformRoot := platformModuleRoot(t)
	mod := fmt.Sprintf(`module compiletest

go 1.24

require zengo/platform v0.0.0

replace zengo/platform => %s
`, platformRoot)
	err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tidy := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidy.Dir = dir
	var out []byte
	out, err = tidy.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}

	cmd := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", filepath.Join(dir, "bin"), ".")
	cmd.Dir = dir
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
}

func TestGenerateMainRejectsRESTWithoutGRPC(t *testing.T) {
	m := baseManifest()
	m.Transports = manifest.Transports{
		REST: &manifest.REST{Port: 8080},
	}

	path := filepath.Join(t.TempDir(), "main.go")
	err := generator.GenerateMain(m, path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "manifest.transports.grpc is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateMainCompilesWithoutREST(t *testing.T) {
	m := baseManifest()
	m.Observability = manifest.Observability{Tracing: true}
	m.Transports = manifest.Transports{
		GRPC: &manifest.GRPC{Port: 9090},
	}

	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.go")
	err := generator.GenerateMain(m, mainPath)
	if err != nil {
		t.Fatal(err)
	}

	stubDir := filepath.Join(dir, "gen", "zengo")
	err = os.MkdirAll(stubDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	stub := `package zengo

import "google.golang.org/grpc"

func RegisterGRPC(*grpc.Server, any) {}
func RegisterHubGRPC(*grpc.Server, any) {}
`
	err = os.WriteFile(filepath.Join(stubDir, "runtime_gen.go"), []byte(stub), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	handlerDir := filepath.Join(dir, "internal", "test")
	err = os.MkdirAll(handlerDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	handlerStub := `package test

type Repository struct{}

func NewRepository(any) *Repository { return &Repository{} }

type Handler struct{}

func NewHandler(*Repository) *Handler { return &Handler{} }
`
	err = os.WriteFile(filepath.Join(handlerDir, "handler.go"), []byte(handlerStub), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	platformRoot := platformModuleRoot(t)
	mod := fmt.Sprintf(`module compiletest

go 1.24

require zengo/platform v0.0.0

replace zengo/platform => %s
`, platformRoot)
	err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tidy := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidy.Dir = dir
	var out []byte
	out, err = tidy.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}

	cmd := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", filepath.Join(dir, "bin"), ".")
	cmd.Dir = dir
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
}

func baseManifest() *manifest.Manifest {
	return &manifest.Manifest{
		Service: manifest.Service{Name: "test", Module: "compiletest"},
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func platformModuleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
