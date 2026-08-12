package versioning

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	api := filepath.Join(root, "api")
	hub := filepath.Join(api, "hub", "user")
	v1 := filepath.Join(api, "v1", "user")
	err := os.MkdirAll(hub, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(v1, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(hub, "service.proto"), []byte("package user.hub;"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	var layout Layout
	layout, err = Discover(api)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if layout.Hub != filepath.Join(api, "hub") {
		t.Fatalf("hub path = %q", layout.Hub)
	}
	if len(layout.Legacy) != 1 || layout.Legacy[0] != "v1" {
		t.Fatalf("legacy = %v", layout.Legacy)
	}
}

func TestDiscoverRequiresHub(t *testing.T) {
	root := t.TempDir()
	api := filepath.Join(root, "api")
	err := os.MkdirAll(api, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Discover(api)
	if err == nil {
		t.Fatal("expected error when hub missing")
	}
}

func TestLegacyVersionsForMode(t *testing.T) {
	legacy := []string{"v1", "v2"}
	autoVersions := legacyVersionsForMode(legacy, CompatibilityAuto)
	if len(autoVersions) != 2 {
		t.Fatalf("auto = %v", autoVersions)
	}
	enabledVersions := legacyVersionsForMode(legacy, CompatibilityEnabled)
	if len(enabledVersions) != 2 {
		t.Fatalf("enabled = %v", enabledVersions)
	}
	disabledVersions := legacyVersionsForMode(legacy, CompatibilityDisabled)
	if len(disabledVersions) != 0 {
		t.Fatalf("disabled = %v", disabledVersions)
	}
}

func TestCompareMessagesAuto(t *testing.T) {
	hub := Message{
		Name: "User",
		Fields: []Field{
			{Name: "id", Type: "string"},
			{Name: "email", Type: "string"},
			{Name: "display_name", Type: "string"},
		},
	}
	legacy := Message{
		Name: "User",
		Fields: []Field{
			{Name: "id", Type: "string"},
			{Name: "email", Type: "string"},
		},
	}
	conv := compareMessages(hub, legacy)
	if conv.Mode != "AUTO" {
		t.Fatalf("mode = %q", conv.Mode)
	}
}

func TestLoadSchemaGengoHub(t *testing.T) {
	InvalidateLoaderCache()
	t.Chdir("../../examples/user-service")
	s, err := LoadSchemaGengo("github.com/zengo/user-service", "gen", "hub")
	if err != nil {
		t.Fatal(err)
	}
	user := s.Messages["User"]
	if len(user.Fields) < 3 {
		t.Fatalf("User fields = %d: %+v", len(user.Fields), user.Fields)
	}
	if len(s.Services) < 1 {
		t.Fatal("no services")
	}
}
