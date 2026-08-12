package local

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"zengo/platform/api/config/db/cassandra"
	"zengo/platform/api/config/db/postgres"
	"zengo/platform/api/config/db/redis"
	"zengo/platform/api/config/logging"
	"zengo/platform/api/config/queue/kafka"
	"zengo/platform/api/config/queue/nats"
	"zengo/platform/api/config/storage/s3"
	"zengo/platform/api/config/tracing"
	"zengo/platform/pkg/sdk/config/storage"
)

func TestGetPostgresYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "postgres.yaml", `kind: postgres
api_version: v1
spec:
  host: localhost
  port: 5432
  db_name: users
  user_name: postgres
  password: secret
`)

	s := New(dir)
	var cfg postgres.Config
	err := s.Get("postgres", &cfg)
	if err != nil {
		t.Fatal(err)
	}
	spec := cfg.GetSpec()
	if spec.GetHost() != "localhost" || spec.GetDbName() != "users" {
		t.Fatalf("unexpected spec: %+v", spec)
	}
}

func TestGetPostgresCanonicalKind(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "postgres.yaml", `kind: postgres
api_version: v1
spec:
  host: localhost
  db_name: users
  user_name: postgres
`)

	s := New(dir)
	var cfg postgres.Config
	err := s.Get("postgres", &cfg)
	if err != nil {
		t.Fatalf("Get(%q): %v", "postgres", err)
	}
}

func TestGetLoggingTextproto(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "logging.textproto", `# proto-file: api/config/infra/logging/config.proto
kind: "logging"
spec: {
  mode: DEV
  level: INFO
}
`)

	s := New(dir)
	var cfg logging.Config
	err := s.Get("logging", &cfg)
	if err != nil {
		t.Fatal(err)
	}
	spec := cfg.GetSpec()
	if spec.GetMode() != logging.Spec_DEV {
		t.Fatalf("mode = %v", spec.GetMode())
	}
}

func TestGetLoggingPbtxt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "logging.pbtxt", `kind: "logging"
spec: {
  level: WARN
}
`)

	s := New(dir)
	var cfg logging.Config
	err := s.Get("logging", &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GetSpec().GetLevel() != logging.Spec_WARN {
		t.Fatalf("level = %v", cfg.GetSpec().GetLevel())
	}
}

func TestDuplicateKindError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", `kind: postgres
spec:
  host: one
  db_name: users
  user_name: postgres
`)
	writeFile(t, dir, "b.textproto", `kind: "postgres"
spec: {
  host: "two"
  db_name: "users"
  user_name: "postgres"
}
`)

	s := New(dir)
	var cfg postgres.Config
	err := s.Get("postgres", &cfg)
	if err == nil {
		t.Fatal("expected duplicate kind error")
	}
}

func TestGetNotFound(t *testing.T) {
	s := New(t.TempDir())
	var cfg postgres.Config
	err := s.Get("postgres", &cfg)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestGetRedisYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "redis.yaml", `kind: redis
api_version: v1
spec:
  addrs:
    - localhost:6379
  db: 0
`)

	s := New(dir)
	var cfg redis.Config
	err := s.Get("redis", &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GetSpec().GetDb() != 0 {
		t.Fatalf("db = %d", cfg.GetSpec().GetDb())
	}
}

func TestGetNatsTextproto(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "nats.textproto", `kind: "nats"
spec: {
  urls: ["nats://localhost:4222"]
}
`)

	s := New(dir)
	var cfg nats.Config
	err := s.Get("nats", &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.GetSpec().GetUrls()) != 1 {
		t.Fatalf("urls = %v", cfg.GetSpec().GetUrls())
	}
}

func TestGetS3YAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "s3.yaml", `kind: s3
api_version: v1
spec:
  bucket: zengo
  region: us-east-1
  endpoint: http://localhost:9000
  access_key_id: minioadmin
  secret_access_key: minioadmin
  force_path_style: true
`)

	s := New(dir)
	var cfg s3.Config
	err := s.Get("s3", &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GetSpec().GetBucket() != "zengo" || !cfg.GetSpec().GetForcePathStyle() {
		t.Fatalf("spec = %+v", cfg.GetSpec())
	}
}

func TestGetTracingYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tracing.yaml", `kind: tracing
api_version: v1
spec:
  endpoint: localhost:4317
  protocol: 1
  sampling_ratio: 1.0
  insecure: true
`)

	s := New(dir)
	var cfg tracing.Config
	err := s.Get("tracing", &cfg)
	if err != nil {
		t.Fatal(err)
	}
	spec := cfg.GetSpec()
	if spec.GetEndpoint() != "localhost:4317" || spec.GetProtocol() != tracing.Spec_GRPC {
		t.Fatalf("spec = %+v", spec)
	}
}

func TestGetKafkaYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "kafka.yaml", `kind: kafka
api_version: v1
spec:
  brokers:
    - localhost:9092
`)

	s := New(dir)
	var cfg kafka.Config
	err := s.Get("kafka", &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.GetSpec().GetBrokers()) != 1 || cfg.GetSpec().GetBrokers()[0] != "localhost:9092" {
		t.Fatalf("spec = %+v", cfg.GetSpec())
	}
}

func TestGetCassandraYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "cassandra.yaml", `kind: cassandra
api_version: v1
spec:
  hosts:
    - localhost:9042
  keyspace: system
`)

	s := New(dir)
	var cfg cassandra.Config
	err := s.Get("cassandra", &cfg)
	if err != nil {
		t.Fatal(err)
	}
	spec := cfg.GetSpec()
	if len(spec.GetHosts()) != 1 || spec.GetKeyspace() != "system" {
		t.Fatalf("spec = %+v", spec)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}
}
