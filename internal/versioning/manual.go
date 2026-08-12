package versioning

import (
	"os"
	"path/filepath"
	"strings"
)

func manualConvertersDir(internal, version string) string {
	return filepath.Join(internal, "convert", version)
}

func hasManualConverter(internal, version, message string) bool {
	dir := manualConvertersDir(internal, version)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	safe := strings.ToLower(message)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".go")
		if name == safe || strings.Contains(name, safe) {
			return true
		}
	}
	return false
}
