package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderUsesNearestOverrideTemplate(t *testing.T) {
	root := t.TempDir()
	overrideDir := filepath.Join(root, ".zengo", "templates")
	err := os.MkdirAll(overrideDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(overrideDir, "runtime.go.tmpl"), []byte("override {{ .Value }}\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(root, "cmd", "runtime.txt")
	err = Render(File{
		Template:   "runtime.go.tmpl",
		Data:       struct{ Value string }{Value: "ok"},
		OutputPath: outPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	var body []byte
	body, err = os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "override ok\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestRenderPrefersCloserOverrideTemplate(t *testing.T) {
	root := t.TempDir()
	top := filepath.Join(root, ".zengo", "templates")
	err := os.MkdirAll(top, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(top, "runtime.go.tmpl"), []byte("top\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(root, "service", ".zengo", "templates")
	err = os.MkdirAll(local, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(local, "runtime.go.tmpl"), []byte("local\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(root, "service", "gen", "runtime.txt")
	err = Render(File{
		Template:   "runtime.go.tmpl",
		Data:       struct{}{},
		OutputPath: outPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	var body []byte
	body, err = os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "local\n" {
		t.Fatalf("body = %q", body)
	}
}
