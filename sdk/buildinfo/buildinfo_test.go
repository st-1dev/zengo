package buildinfo

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"testing"
)

func TestCurrentFromBuildInfo(t *testing.T) {
	resetBuildVars(t)
	info := currentFrom("user-service", &debug.BuildInfo{
		GoVersion: "go1.26.1",
		Main: debug.Module{
			Path:    "example.com/user-service",
			Version: "v1.2.3",
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc123"},
			{Key: "vcs.time", Value: "2026-06-03T08:00:00Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	})

	if info.Service != "user-service" {
		t.Fatalf("service = %q", info.Service)
	}
	if info.Module != "example.com/user-service" {
		t.Fatalf("module = %q", info.Module)
	}
	if info.Version != "v1.2.3" {
		t.Fatalf("version = %q", info.Version)
	}
	if info.Commit != "abc123" {
		t.Fatalf("commit = %q", info.Commit)
	}
	if info.Time != "2026-06-03T08:00:00Z" {
		t.Fatalf("time = %q", info.Time)
	}
	if !info.Dirty {
		t.Fatal("dirty = false")
	}
	if info.GoVersion != "go1.26.1" {
		t.Fatalf("go_version = %q", info.GoVersion)
	}
}

func TestCurrentFromDevelBuildOmitsVersion(t *testing.T) {
	resetBuildVars(t)
	info := currentFrom("svc", &debug.BuildInfo{
		Main: debug.Module{Path: "example.com/svc", Version: "(devel)"},
	})
	if info.Version != "" {
		t.Fatalf("version = %q", info.Version)
	}
}

func TestCurrentFromMissingSettings(t *testing.T) {
	resetBuildVars(t)
	info := currentFrom("svc", &debug.BuildInfo{
		Main:      debug.Module{Path: "example.com/svc"},
		GoVersion: "go1.26.1",
	})
	if info.Commit != "" {
		t.Fatalf("commit = %q", info.Commit)
	}
	if info.Time != "" {
		t.Fatalf("time = %q", info.Time)
	}
	if info.Dirty {
		t.Fatal("dirty = true")
	}
}

func TestCurrentFromLinkerVarsOverrideVersionAndBranch(t *testing.T) {
	resetBuildVars(t)
	Version = "v9.9.9"
	Branch = "release/test"
	info := currentFrom("svc", &debug.BuildInfo{
		Main: debug.Module{Path: "example.com/svc", Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc123"},
			{Key: "vcs.modified", Value: "false"},
		},
	})
	if info.Version != "v9.9.9" {
		t.Fatalf("version = %q", info.Version)
	}
	if info.Branch != "release/test" {
		t.Fatalf("branch = %q", info.Branch)
	}
	if info.Commit != "abc123" {
		t.Fatalf("commit = %q", info.Commit)
	}
}

func TestHandlerAndPrintMatch(t *testing.T) {
	resetBuildVars(t)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/buildz", nil)
	rec := httptest.NewRecorder()

	Handler("svc").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	got := rec.Header().Get("Content-Type")
	if got != "application/json" {
		t.Fatalf("content-type = %q", got)
	}

	var printed bytes.Buffer
	err := Print(&printed, "svc")
	if err != nil {
		t.Fatal(err)
	}

	handlerInfo := decodeInfo(t, rec.Body.Bytes())
	printInfo := decodeInfo(t, printed.Bytes())
	if handlerInfo != printInfo {
		t.Fatalf("handler info = %+v, print info = %+v", handlerInfo, printInfo)
	}
}

func resetBuildVars(t *testing.T) {
	t.Helper()
	oldVersion := Version
	oldBranch := Branch
	Version = ""
	Branch = ""
	t.Cleanup(func() {
		Version = oldVersion
		Branch = oldBranch
	})
}

func decodeInfo(t *testing.T, data []byte) Info {
	t.Helper()
	var info Info
	err := json.Unmarshal(data, &info)
	if err != nil {
		t.Fatal(err)
	}
	return info
}
