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
