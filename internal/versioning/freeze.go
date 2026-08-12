package versioning

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var httpPathRe = regexp.MustCompile(`(get|post|put|patch|delete):\s*"([^"]+)"`)

// Freeze snapshots api/hub into a new legacy api/vN directory.
func Freeze(apiRoot, version, module, genDir string) error {
	if !legacyDirPattern.MatchString(version) {
		return fmt.Errorf("version must match v[0-9]+, got %q", version)
	}
	hub := filepath.Join(apiRoot, "hub")
	meta, err := DiscoverHubMeta(module, genDir)
	if err != nil {
		return err
	}
	target := filepath.Join(apiRoot, version)
	_, err = os.Stat(target)
	if err == nil {
		return fmt.Errorf("legacy version directory already exists: %s", target)
	}
	return filepath.WalkDir(hub, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		var rel string

		rel, err = filepath.Rel(hub, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(target, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		var data []byte

		data, err = os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		if strings.HasSuffix(path, ".proto") {
			content = transformFrozenProto(content, version, meta)
		}
		err = os.MkdirAll(filepath.Dir(dst), 0o755)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, []byte(content), 0o644)
	})
}

func transformFrozenProto(content, version string, meta HubMeta) string {
	hubPkg := meta.PackageSuffix
	if hubPkg == "" {
		hubPkg = "hub"
	}
	pkg := packageRe.FindStringSubmatch(content)
	if len(pkg) == 2 {
		base := strings.TrimSuffix(pkg[1], "."+hubPkg)
		content = strings.Replace(content, pkg[0], fmt.Sprintf("package %s.%s;", base, version), 1)
	}
	content = strings.ReplaceAll(content, "/api/hub/", "/api/"+version+"/")
	m := goPackageRe.FindStringSubmatch(content)
	if len(m) == 2 {
		old := m[1]
		newImport := strings.Replace(old, "/hub/", "/"+version+"/", 1)
		newImport = strings.Replace(
			newImport,
			";"+hubPkg,
			";"+version+strings.ToLower(serviceBaseName(meta.PrimaryService)),
			1,
		)
		content = strings.Replace(content, old, newImport, 1)
	}
	content = httpPathRe.ReplaceAllStringFunc(content, func(s string) string {
		m := httpPathRe.FindStringSubmatch(s)
		if len(m) != 3 {
			return s
		}
		p := m[2]
		if strings.HasPrefix(p, "/"+version+"/") || strings.HasPrefix(p, "/v") {
			return s
		}
		return m[1] + `: "/` + version + p + `"`
	})
	return content
}
