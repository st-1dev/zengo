package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"zengo/platform/pkg/sdk/config/configfmt"
)

// Update loads a manifest, applies patch, and writes it back using the same format.
func Update(path string, patch func(*Manifest) error) error {
	resolved, pb, err := loadProto(path)
	if err != nil {
		return err
	}
	m := fromProto(pb)
	err = patch(m)
	if err != nil {
		return err
	}
	err = validate(m)
	if err != nil {
		return err
	}
	var data []byte
	data, err = configfmt.Marshal(resolved, m.toProto())
	if err != nil {
		return err
	}
	return os.WriteFile(resolved, data, 0o644)
}

// FindPath returns the resolved manifest file path under dir.
//
// When dir is empty or ".", FindPath falls back to the current working directory.
func FindPath(dir string) (string, error) {
	if dir == "" || dir == "." {
		return resolveManifestPath("")
	}
	var found []string
	for _, name := range manifestCandidates {
		path := filepath.Join(dir, name)
		_, err := os.Stat(path)
		if err == nil {
			found = append(found, path)
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf(
			"manifest not found: expected one of %s in %s",
			strings.Join(manifestCandidates, ", "),
			dir,
		)
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf(
			"multiple manifest files found (%s); pass --manifest explicitly",
			strings.Join(found, ", "),
		)
	}
}
