package versioning

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var legacyDirPattern = regexp.MustCompile(`^v[0-9]+$`)

// Layout describes the API directory layout discovered under api/.
type Layout struct {
	// APIRoot is the absolute or relative root passed to Discover.
	APIRoot string
	// Hub is the required api/hub directory path.
	Hub string
	// Legacy lists discovered legacy directories like v1 or v2.
	Legacy []string
}

// Discover scans apiRoot and reports the canonical hub plus legacy version directories.
func Discover(apiRoot string) (Layout, error) {
	info, err := os.Stat(apiRoot)
	if err != nil {
		return Layout{}, fmt.Errorf("api root %q: %w", apiRoot, err)
	}
	if !info.IsDir() {
		return Layout{}, fmt.Errorf("api root %q is not a directory", apiRoot)
	}

	hubPath := filepath.Join(apiRoot, "hub")
	var st os.FileInfo
	st, err = os.Stat(hubPath)
	if err != nil || !st.IsDir() {
		return Layout{}, fmt.Errorf("required hub directory missing: %s", hubPath)
	}
	var entries []os.DirEntry
	entries, err = os.ReadDir(apiRoot)
	if err != nil {
		return Layout{}, err
	}

	var legacy []string
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "hub" || stringsHasPrefixDot(entry.Name()) {
			continue
		}
		if legacyDirPattern.MatchString(entry.Name()) {
			legacy = append(legacy, entry.Name())
		}
	}
	sort.Strings(legacy)

	return Layout{
		APIRoot: apiRoot,
		Hub:     hubPath,
		Legacy:  legacy,
	}, nil
}

func stringsHasPrefixDot(name string) bool {
	return len(name) > 0 && name[0] == '.'
}
