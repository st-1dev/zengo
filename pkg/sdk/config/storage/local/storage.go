package local

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"zengo/platform/api/config/meta"
	"zengo/platform/pkg/sdk/config/configfmt"
	"zengo/platform/pkg/sdk/config/storage"

	"google.golang.org/protobuf/proto"
)

// Storage loads typed configs from a local directory.
type Storage struct {
	path string
}

// New creates a local config storage rooted at path.
func New(path string) *Storage {
	return &Storage{path: path}
}

// Get loads the config file registered for kind into cfg.
func (s *Storage) Get(kind string, cfg proto.Message) error {
	index, err := s.indexFiles()
	if err != nil {
		return err
	}
	files := lookupKind(index, kind)
	if len(files) == 0 {
		return storage.ErrNotFound
	}
	if len(files) > 1 {
		return fmt.Errorf("multiple config files for kind %q: %s", kind, strings.Join(files, ", "))
	}
	var data []byte

	data, err = os.ReadFile(files[0])
	if err != nil {
		return err
	}
	return configfmt.Unmarshal(files[0], data, cfg)
}

func (s *Storage) indexFiles() (map[string][]string, error) {
	byKind := map[string][]string{}
	for _, ext := range configfmt.SupportedExtensions() {
		matches, err := filepath.Glob(filepath.Join(s.path, "*"+ext))
		if err != nil {
			return nil, err
		}
		for _, path := range matches {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			var metaType meta.Type
			err = configfmt.UnmarshalMeta(path, data, &metaType)
			if err != nil {
				return nil, fmt.Errorf("parse config metadata %s: %w", path, err)
			}
			if metaType.Kind == "" {
				continue
			}
			byKind[metaType.Kind] = append(byKind[metaType.Kind], path)
		}
	}
	return byKind, nil
}

func lookupKind(index map[string][]string, kind string) []string {
	return index[kind]
}
