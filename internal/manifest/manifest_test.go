package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	content := `service:
  name: demo
  module: github.com/zengo/demo
transports:
  grpc:
    port: 9091
  rest:
    port: 8081
observability:
  metrics: true
  tracing: false
  health: true
`
	path := filepath.Join(dir, "zengo.yaml")
	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var m *Manifest

	m, err = Load("")
	if err != nil {
		t.Fatal(err)
	}
	if m.Service.Name != "demo" || m.GRPCPort() != 9091 {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}

func TestLoadTextproto(t *testing.T) {
	dir := t.TempDir()
	content := `service: {
  name: "demo"
  module: "github.com/zengo/demo"
}
transports: {
  grpc: { port: 9092 }
}
observability: {
  metrics: true
  tracing: true
  health: false
}
`
	path := filepath.Join(dir, "zengo.textproto")
	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	var m *Manifest

	m, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Service.Module != "github.com/zengo/demo" || m.RESTPort() != 8080 {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}

func TestLoadRejectsRESTWithoutGRPC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zengo.yaml")
	content := `service:
  name: demo
  module: github.com/zengo/demo
transports:
  rest:
    port: 8080
`
	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "manifest.transports.grpc is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsPortOutsideTCPRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zengo.yaml")
	content := `
service:
  name: demo
  module: example/demo
transports:
  grpc:
    port: 70000
`
	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Load(path)
	if err == nil {
		t.Fatal("expected invalid port error")
	}
	if !strings.Contains(err.Error(), "between 1 and 65535") {
		t.Fatalf("error = %v, want port range", err)
	}
}

func TestAutoDiscoverMultipleManifests(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"zengo.yaml", "zengo.textproto"} {
		err := os.WriteFile(filepath.Join(dir, name), []byte("service: { name: x module: y }"), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)

	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for multiple manifests")
	}
}

func TestUpdatePreservesTextprotoFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zengo.textproto")
	writeManifestFile(t, path, `service: { name: "demo-service" module: "github.com/zengo/demo-service" }`)
	err := Update(path, func(m *Manifest) error {
		m.Observability.Health = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(path)
	if err != nil {
		t.Fatalf("expected textproto manifest: %v", err)
	}
	_, err = os.Stat(filepath.Join(dir, "zengo.yaml"))
	if !os.IsNotExist(err) {
		t.Fatalf("unexpected yaml manifest: %v", err)
	}
	var m *Manifest

	m, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Observability.Health {
		t.Fatal("expected health=true after update")
	}
}

func TestUpdatePreservesYAMLFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zengo.yaml")
	writeManifestFile(t, path, `service:
  name: demo-service
  module: github.com/zengo/demo-service
`)
	err := Update(path, func(m *Manifest) error {
		m.DB = &DB{Postgres: &PostgresDB{Queries: "queries/"}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(path)
	if err != nil {
		t.Fatalf("expected yaml manifest: %v", err)
	}
	_, err = os.Stat(filepath.Join(dir, "zengo.textproto"))
	if !os.IsNotExist(err) {
		t.Fatalf("unexpected textproto manifest: %v", err)
	}
	var m *Manifest

	m, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.DB == nil || m.DB.Postgres == nil || m.DB.Postgres.Queries != "queries/" {
		t.Fatalf("unexpected manifest after update: %+v", m)
	}
}

func TestObservabilityConfigKeys(t *testing.T) {
	o := Observability{
		TracingConfigFrom: "otel",
		LoggingConfigFrom: "app-logging",
	}
	if o.TracingConfigKey() != "otel" || o.LoggingConfigKey() != "app-logging" {
		t.Fatalf("unexpected keys: tracing=%q logging=%q", o.TracingConfigKey(), o.LoggingConfigKey())
	}
	defaults := Observability{Tracing: true}
	if defaults.TracingConfigKey() != "tracing" || defaults.LoggingConfigKey() != "logging" {
		t.Fatalf("unexpected defaults: tracing=%q logging=%q", defaults.TracingConfigKey(), defaults.LoggingConfigKey())
	}
}

func TestKafkaConfigKey(t *testing.T) {
	k := &Kafka{BrokersFromConfig: "events"}
	if k.ConfigKey() != "events" {
		t.Fatalf("ConfigKey = %q", k.ConfigKey())
	}
	if (&Kafka{}).ConfigKey() != "kafka" {
		t.Fatal("expected default kafka key")
	}
}

func TestLoadCanonicalQueueKafka(t *testing.T) {
	m := &Manifest{
		Queue: &Queue{
			Kafka: &Kafka{BrokersFromConfig: "queue"},
		},
	}
	if m.Queue == nil || m.Queue.Kafka == nil || m.Queue.Kafka.ConfigKey() != "queue" {
		t.Fatalf("Queue = %+v", m.Queue)
	}
}

func TestNeedsConfigLoaderForCanonicalDBAndQueue(t *testing.T) {
	m := &Manifest{
		DB: &DB{
			Postgres: &PostgresDB{Queries: "queries/"},
		},
		Queue: &Queue{
			Kafka: &Kafka{BrokersFromConfig: "kafka"},
		},
	}
	if !m.NeedsConfigLoader() {
		t.Fatal("expected config loader for canonical db/queue")
	}
}

func TestNeedsConfigLoaderForCacheStorageAnalytics(t *testing.T) {
	m := &Manifest{
		Cache:   &Cache{Redis: &Redis{ConfigFrom: "redis"}},
		Storage: &Storage{S3: &S3{ConfigFrom: "s3"}},
	}
	if !m.NeedsConfigLoader() {
		t.Fatal("expected config loader for cache/storage")
	}
}

func TestUpdatePreservesCanonicalInfraSections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zengo.yaml")
	writeManifestFile(t, path, `service:
  name: demo-service
  module: github.com/zengo/demo-service
`)
	err := Update(path, func(m *Manifest) error {
		m.Cache = &Cache{Redis: &Redis{ConfigFrom: "redis"}}
		m.Storage = &Storage{S3: &S3{ConfigFrom: "s3"}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var m *Manifest

	m, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Cache == nil || m.Cache.Redis == nil || m.Cache.Redis.ConfigKey() != "redis" {
		t.Fatalf("unexpected cache: %+v", m.Cache)
	}
	if m.Storage == nil || m.Storage.S3 == nil || m.Storage.S3.ConfigKey() != "s3" {
		t.Fatalf("unexpected storage: %+v", m.Storage)
	}
}

func TestHandlerPackageNormalizesServiceName(t *testing.T) {
	m := &Manifest{Service: Service{Name: "user-profile-service"}}
	if m.HandlerPackage() != "user_profile" {
		t.Fatalf("HandlerPackage = %q", m.HandlerPackage())
	}
}

func TestCompatibilityModeLoadsFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zengo.yaml")
	writeManifestFile(t, path, `service:
  name: demo-service
  module: github.com/zengo/demo-service
compatibility:
  legacy_versions: ENABLED
`)
	m, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !m.LegacyCompatibilityEnabled() {
		t.Fatalf("expected enabled compatibility, got %q", m.LegacyCompatibilityMode())
	}
}

func TestUpdatePreservesExplicitCompatibilityDisable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zengo.textproto")
	writeManifestFile(t, path, `service: { name: "demo-service" module: "github.com/zengo/demo-service" }
compatibility: { legacy_versions: DISABLED }
`)
	err := Update(path, func(m *Manifest) error {
		m.Observability.Health = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var m *Manifest

	m, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !m.LegacyCompatibilityConfigured() {
		t.Fatal("expected compatibility mode to stay configured")
	}
	if m.LegacyCompatibilityMode() != CompatibilityDisabled {
		t.Fatalf("expected disabled compatibility, got %q", m.LegacyCompatibilityMode())
	}
}

func writeManifestFile(t *testing.T, path, content string) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}
}
