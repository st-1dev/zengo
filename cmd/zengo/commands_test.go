package main

import (
	"os"
	"path/filepath"
	"testing"
	"zengo/platform/internal/manifest"
	"zengo/platform/internal/scaffold"
)

func TestParseInitArgsSupportsNoGenAfterPositionals(t *testing.T) {
	parsed, err := parseInitArgs([]string{"demo-service", "/tmp/demo", "--no-gen"})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.name != "demo-service" {
		t.Fatalf("name = %q", parsed.name)
	}
	if parsed.dir != "/tmp/demo" {
		t.Fatalf("dir = %q", parsed.dir)
	}
	if !parsed.noGen {
		t.Fatal("expected noGen=true")
	}
	if parsed.manifestFormat != scaffold.ManifestFormatTextproto {
		t.Fatalf("manifestFormat = %q", parsed.manifestFormat)
	}
}

func TestParseInitArgsSupportsManifestFormatAnywhere(t *testing.T) {
	parsed, err := parseInitArgs([]string{"demo-service", "--manifest-format", "yaml", "/tmp/demo"})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.manifestFormat != scaffold.ManifestFormatYAML {
		t.Fatalf("manifestFormat = %q", parsed.manifestFormat)
	}
}

func TestParseInitArgsWithDefaultsKeepsPreParsedFlags(t *testing.T) {
	parsed, err := parseInitArgsWithDefaults([]string{"demo-service", "/tmp/demo"}, initArgs{
		noGen:          true,
		manifestFormat: scaffold.ManifestFormatYAML,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.noGen {
		t.Fatal("expected noGen=true")
	}
	if parsed.manifestFormat != scaffold.ManifestFormatYAML {
		t.Fatalf("manifestFormat = %q", parsed.manifestFormat)
	}
}

func TestParseInitArgsWithDefaultsParsesTrailingFlags(t *testing.T) {
	parsed, err := parseInitArgsWithDefaults(
		[]string{"demo-service", "/tmp/demo", "--manifest-format", "yaml", "--no-gen"},
		initArgs{
			manifestFormat: scaffold.ManifestFormatTextproto,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.noGen {
		t.Fatal("expected noGen=true")
	}
	if parsed.manifestFormat != scaffold.ManifestFormatYAML {
		t.Fatalf("manifestFormat = %q", parsed.manifestFormat)
	}
}

func TestParseInitArgsRejectsUnknownFlags(t *testing.T) {
	_, err := parseInitArgs([]string{"demo-service", "--wat"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestFreezeVersionEnablesLegacyCompatibility(t *testing.T) {
	dir := t.TempDir()
	writeFreezeTestFile(t, filepath.Join(dir, "go.mod"), "module github.com/zengo/demo\n\ngo 1.26\n")
	writeFreezeTestFile(
		t,
		filepath.Join(dir, "zengo.textproto"),
		`service: { name: "demo" module: "github.com/zengo/demo" }`,
	)
	writeFreezeTestFile(t, filepath.Join(dir, "gen", "api", "hub", "demo", "service_grpc.pb.go"), `package demo

type DemoServiceServer interface {
	Do()
}
`)
	writeFreezeTestFile(t, filepath.Join(dir, "api", "hub", "demo", "service.proto"), `syntax = "proto3";

package demo.hub;

option go_package = "github.com/zengo/demo/gen/api/hub/demo;demo";

service DemoService {}
`)

	err := freezeVersion(dir, "v1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(filepath.Join(dir, "api", "v1", "demo", "service.proto"))
	if err != nil {
		t.Fatalf("expected frozen v1 proto: %v", err)
	}
	var m *manifest.Manifest
	m, err = manifest.Load(filepath.Join(dir, "zengo.textproto"))
	if err != nil {
		t.Fatal(err)
	}
	if !m.LegacyCompatibilityEnabled() {
		t.Fatalf("expected compatibility mode enabled, got %q", m.LegacyCompatibilityMode())
	}
}

func writeFreezeTestFile(t *testing.T, path, content string) {
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
