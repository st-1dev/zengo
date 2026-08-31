package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"zengo/platform/internal/naming"
)

//go:embed all:templates
var templates embed.FS

// ManifestFormat selects the manifest file format created by InitService.
type ManifestFormat string

const (
	// ManifestFormatTextproto writes zengo.textproto.
	ManifestFormatTextproto ManifestFormat = "textproto"
	// ManifestFormatYAML writes zengo.yaml.
	ManifestFormatYAML ManifestFormat = "yaml"
)

// InitOptions controls scaffold generation behavior.
type InitOptions struct {
	// ManifestFormat selects the manifest file format. The zero value defaults to textproto.
	ManifestFormat ManifestFormat
}

// ParseManifestFormat normalizes CLI input into a supported manifest format.
func ParseManifestFormat(raw string) (ManifestFormat, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(ManifestFormatTextproto):
		return ManifestFormatTextproto, nil
	case string(ManifestFormatYAML), "yml":
		return ManifestFormatYAML, nil
	default:
		return "", fmt.Errorf("unsupported manifest format %q (expected textproto or yaml)", raw)
	}
}

// InitService creates a new service repository from embedded scaffold templates.
func InitService(name, dir string, opts InitOptions) error {
	targetDir := filepath.Clean(dir)
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("resolve init target %q: %w", targetDir, err)
	}
	platformRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve platform root: %w", err)
	}
	platformReplace, err := filepath.Rel(absTarget, platformRoot)
	if err != nil {
		return fmt.Errorf("resolve platform replace: %w", err)
	}
	platformReplace = filepath.ToSlash(platformReplace)
	entries, err := os.ReadDir(targetDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect init target %q: %w", targetDir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("init target %q is not empty", targetDir)
	}

	module := "github.com/zengo/" + name
	manifestFormat := opts.ManifestFormat
	if manifestFormat == "" {
		manifestFormat = ManifestFormatTextproto
	}
	handlerPkg := naming.HandlerPackage(name)
	handlerTitle := naming.HandlerTitle(handlerPkg)
	replacer := strings.NewReplacer(
		"{{SERVICE_NAME}}", name,
		"{{MODULE}}", module,
		"{{HANDLER_PKG}}", handlerPkg,
		"{{HANDLER_TITLE}}", handlerTitle,
		"{{PLATFORM_REPLACE}}", platformReplace,
	)
	return fs.WalkDir(templates, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if skipTemplatePath(path, d, manifestFormat) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		var rel string

		rel, err = filepath.Rel("templates", path)
		if err != nil {
			return err
		}
		rel = strings.ReplaceAll(rel, "_handler_", handlerPkg)
		target := filepath.Join(targetDir, replacer.Replace(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if strings.HasSuffix(path, ".tmpl") {
			target = strings.TrimSuffix(target, ".tmpl")
		}
		var data []byte

		data, err = templates.ReadFile(path)
		if err != nil {
			return err
		}
		err = os.MkdirAll(filepath.Dir(target), 0o755)
		if err != nil {
			return err
		}
		content := replacer.Replace(string(data))
		err = os.WriteFile(target, []byte(content), 0o644)
		if err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		return nil
	})
}

func skipTemplatePath(path string, d fs.DirEntry, manifestFormat ManifestFormat) bool {
	if d.IsDir() && strings.HasPrefix(path, "templates/third_party") {
		return true
	}
	if d.IsDir() && path == "templates/api/v1" {
		return true
	}
	if strings.HasPrefix(path, "templates/third_party/") {
		return true
	}
	if manifestFormat == ManifestFormatTextproto && path == "templates/zengo.yaml" {
		return true
	}
	if manifestFormat == ManifestFormatYAML && path == "templates/zengo.textproto.tmpl" {
		return true
	}
	return path == "templates/api/hub/_handler_/_handler_.proto.tmpl" ||
		strings.HasPrefix(path, "templates/api/v1/")
}
