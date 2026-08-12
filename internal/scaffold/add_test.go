package scaffold

import (
	"os"
	"path/filepath"
	"testing"
	"zengo/platform/internal/manifest"
)

func TestAddPreservesTextprotoManifest(t *testing.T) {
	dir := t.TempDir()
	writeScaffoldFile(
		t,
		filepath.Join(dir, "zengo.textproto"),
		`service: { name: "demo-service" module: "github.com/zengo/demo-service" }`,
	)

	err := Add(dir, "observability")
	if err != nil {
		t.Fatal(err)
	}
	err = Add(dir, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(filepath.Join(dir, "zengo.textproto"))
	if err != nil {
		t.Fatalf("expected zengo.textproto: %v", err)
	}
	_, err = os.Stat(filepath.Join(dir, "zengo.yaml"))
	if !os.IsNotExist(err) {
		t.Fatalf("unexpected zengo.yaml: %v", err)
	}
	var m *manifest.Manifest
	m, err = manifest.Load(filepath.Join(dir, "zengo.textproto"))
	if err != nil {
		t.Fatal(err)
	}
	if !m.Observability.Metrics || m.DB == nil || m.DB.Postgres == nil {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	for _, want := range []string{
		filepath.Join(dir, "configs", "logging.yaml"),
		filepath.Join(dir, "configs", "tracing.yaml"),
		filepath.Join(dir, "configs", "postgres.yaml"),
		filepath.Join(dir, "queries", "demo.sql"),
	} {
		_, err := os.Stat(want)
		if err != nil {
			t.Fatalf("expected generated file %s: %v", want, err)
		}
	}
	_, err = os.Stat(filepath.Join(dir, "migrations", "001_init.sql"))
	if !os.IsNotExist(err) {
		t.Fatalf("unexpected migration scaffold: %v", err)
	}
}

func TestAddPreservesYAMLManifest(t *testing.T) {
	dir := t.TempDir()
	writeScaffoldFile(t, filepath.Join(dir, "zengo.yaml"), `service:
  name: demo-service
  module: github.com/zengo/demo-service
`)

	err := Add(dir, "observability")
	if err != nil {
		t.Fatal(err)
	}
	err = Add(dir, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(filepath.Join(dir, "zengo.yaml"))
	if err != nil {
		t.Fatalf("expected zengo.yaml: %v", err)
	}
	_, err = os.Stat(filepath.Join(dir, "zengo.textproto"))
	if !os.IsNotExist(err) {
		t.Fatalf("unexpected zengo.textproto: %v", err)
	}
	var m *manifest.Manifest
	m, err = manifest.Load(filepath.Join(dir, "zengo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !m.Observability.Tracing || m.DB == nil || m.DB.Postgres == nil {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}

func TestAddCanonicalInfraSections(t *testing.T) {
	dir := t.TempDir()
	writeScaffoldFile(t, filepath.Join(dir, "zengo.yaml"), `service:
  name: demo-service
  module: github.com/zengo/demo-service
`)

	for _, target := range []string{"redis", "s3"} {
		err := Add(dir, target)
		if err != nil {
			t.Fatalf("Add(%q): %v", target, err)
		}
	}

	m, err := manifest.Load(filepath.Join(dir, "zengo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Cache == nil || m.Cache.Redis == nil {
		t.Fatalf("unexpected cache: %+v", m.Cache)
	}
	if m.Storage == nil || m.Storage.S3 == nil {
		t.Fatalf("unexpected storage: %+v", m.Storage)
	}
	for _, want := range []string{
		filepath.Join(dir, "configs", "redis.yaml"),
		filepath.Join(dir, "configs", "s3.yaml"),
	} {
		_, err := os.Stat(want)
		if err != nil {
			t.Fatalf("expected generated file %s: %v", want, err)
		}
	}
}

func writeScaffoldFile(t *testing.T, path, content string) {
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
