package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestGeneratedTargetsIncludesGeneratedMain(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeTestFile(t, "zengo.textproto", `service: { name: "demo-service" module: "github.com/zengo/demo-service" }`)

	plan, err := generatedTargets("")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.forceMain {
		t.Fatal("expected forceMain=true when cmd/main.go is missing")
	}
	for _, want := range []string{"cmd/main.go", "gen/api", filepath.Join("docs", "PROTO_REGISTRY.md")} {
		if !slices.Contains(plan.targets, want) {
			t.Fatalf("targets missing %q: %v", want, plan.targets)
		}
	}
}

func TestGeneratedTargetsSkipsHandWrittenMain(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeTestFile(t, "zengo.textproto", `service: { name: "demo-service" module: "github.com/zengo/demo-service" }`)
	writeTestFile(t, filepath.Join("cmd", "main.go"), "package main\n")

	plan, err := generatedTargets("")
	if err != nil {
		t.Fatal(err)
	}
	if plan.forceMain {
		t.Fatal("expected forceMain=false for hand-written cmd/main.go")
	}
	if slices.Contains(plan.targets, filepath.Join("cmd", "main.go")) {
		t.Fatalf("unexpected generated main target: %v", plan.targets)
	}
}

func TestGeneratedSnapshotsDetectChangedAndMissingTargets(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeTestFile(t, filepath.Join("gen", "api", "demo.pb.go"), "v1")
	writeTestFile(t, filepath.Join("cmd", "main.go"), generatedMainHeader+"\npackage main\n")
	writeTestFile(t, filepath.Join("docs", "PROTO_REGISTRY.md"), "doc")

	bakRoot := t.TempDir()
	snapshots, err := snapshotGeneratedTargets([]string{
		filepath.Join("gen", "api"),
		filepath.Join("cmd", "main.go"),
		filepath.Join("docs", "PROTO_REGISTRY.md"),
	}, bakRoot)
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join("gen", "api", "demo.pb.go"), "v2")
	writeTestFile(t, filepath.Join("cmd", "main.go"), generatedMainHeader+"\npackage main\n// changed\n")
	err = os.Remove(filepath.Join("docs", "PROTO_REGISTRY.md"))
	if err != nil {
		t.Fatal(err)
	}
	var stale []string

	stale, err = diffGeneratedTargets(snapshots)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{filepath.Join("gen", "api"), filepath.Join("cmd", "main.go"), filepath.Join("docs", "PROTO_REGISTRY.md")} {
		if !slices.Contains(stale, want) {
			t.Fatalf("stale targets missing %q: %v", want, stale)
		}
	}
	err = restoreGeneratedTargets(snapshots)
	if err != nil {
		t.Fatal(err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
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

func TestGeneratedSnapshotsIgnoreUnchangedDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeTestFile(t, filepath.Join("gen", "zengo", "runtime_gen.go"), "package zengo\n")
	bakRoot := t.TempDir()
	snapshots, err := snapshotGeneratedTargets([]string{filepath.Join("gen", "zengo")}, bakRoot)
	if err != nil {
		t.Fatal(err)
	}
	var stale []string

	stale, err = diffGeneratedTargets(snapshots)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 0 {
		t.Fatalf("unexpected stale targets: %v", stale)
	}
}
