package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitServiceSkipsDuplicateHubAndThirdPartyTemplates(t *testing.T) {
	dir := t.TempDir()
	err := InitService("demo-service", dir, InitOptions{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(dir, "api", "hub", "demo", "demo.proto"))
	if !os.IsNotExist(err) {
		t.Fatalf("unexpected legacy hub template output: %v", err)
	}
	_, err = os.Stat(filepath.Join(dir, "third_party", "zengo", "options", "kafka.proto"))
	if !os.IsNotExist(err) {
		t.Fatalf("unexpected third_party options output: %v", err)
	}
	_, err = os.Stat(filepath.Join(dir, "api", "v1"))
	if !os.IsNotExist(err) {
		t.Fatalf("unexpected legacy v1 scaffold output: %v", err)
	}
	_, err = os.Stat(filepath.Join(dir, "zengo", "options", "kafka.proto"))
	if err != nil {
		t.Fatalf("expected zengo/options/kafka.proto: %v", err)
	}

	protoPath := filepath.Join(dir, "api", "hub", "demo", "service.proto")
	var data []byte

	data, err = os.ReadFile(protoPath)
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, "package demo.hub;") {
		t.Fatalf("hub proto package mismatch:\n%s", src)
	}
	if strings.Contains(src, "/v1/demos") {
		t.Fatalf("hub proto should not contain versioned routes:\n%s", src)
	}
}

func TestInitServiceDefaultsToTextprotoManifest(t *testing.T) {
	dir := t.TempDir()
	err := InitService("demo-service", dir, InitOptions{})
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
}

func TestInitServiceSupportsYAMLManifest(t *testing.T) {
	dir := t.TempDir()
	err := InitService("demo-service", dir, InitOptions{ManifestFormat: ManifestFormatYAML})
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
}

func TestInitServiceNormalizesHandlerNames(t *testing.T) {
	dir := t.TempDir()
	err := InitService("user-profile-service", dir, InitOptions{})
	if err != nil {
		t.Fatal(err)
	}

	handlerPath := filepath.Join(dir, "internal", "user_profile", "handler.go")
	var data []byte
	data, err = os.ReadFile(handlerPath)
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, "package user_profile") {
		t.Fatalf("handler package mismatch:\n%s", src)
	}
	if !strings.Contains(src, "UnimplementedUserProfileServiceServer") {
		t.Fatalf("handler title mismatch:\n%s", src)
	}

	protoPath := filepath.Join(dir, "api", "hub", "user_profile", "service.proto")
	var protoData []byte
	protoData, err = os.ReadFile(protoPath)
	if err != nil {
		t.Fatal(err)
	}
	protoSrc := string(protoData)
	if !strings.Contains(protoSrc, "package user_profile.hub;") {
		t.Fatalf("proto package mismatch:\n%s", protoSrc)
	}
	if !strings.Contains(protoSrc, "service UserProfileService") {
		t.Fatalf("proto service name mismatch:\n%s", protoSrc)
	}
}

func TestInitServiceRefusesNonEmptyTarget(t *testing.T) {
	dir := t.TempDir()
	sentinelPath := filepath.Join(dir, "go.mod")
	want := "module existing\n"
	if err := os.WriteFile(sentinelPath, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	err := InitService("demo-service", dir, InitOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("error = %q", err)
	}
	got, err := os.ReadFile(sentinelPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("go.mod = %q, want %q", got, want)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "go.mod" {
		t.Fatalf("target entries = %v, want only go.mod", entries)
	}
}

func TestInitServiceRefusesNonEmptyCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	err := InitService("demo-service", "", InitOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "existing.txt" {
		t.Fatalf("target entries = %v, want only existing.txt", entries)
	}
}

func TestInitServiceCreatesMissingTarget(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "service")
	if err := InitService("demo-service", dir, InitOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "zengo.textproto")); err != nil {
		t.Fatalf("expected zengo.textproto: %v", err)
	}
}
