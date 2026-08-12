package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateCursorRulesCreatesExpectedPath(t *testing.T) {
	t.Chdir(t.TempDir())
	err := GenerateCursorRules("demo-service")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(".cursor", "rules", "zengo-service.mdc")

	var data []byte
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("cursor rules file is empty")
	}
}

func TestGenerateBufRegistryDocCreatesDocsDir(t *testing.T) {
	t.Chdir(t.TempDir())
	err := GenerateBufRegistryDoc("demo-service")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("docs", "PROTO_REGISTRY.md")
	var data []byte
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("proto registry doc is empty")
	}
}
