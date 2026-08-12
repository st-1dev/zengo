package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateCursorRules writes the default Cursor rules file into the current root.
func GenerateCursorRules(serviceName string) error {
	return GenerateCursorRulesAt(".", serviceName)
}

// GenerateCursorRulesAt writes the default Cursor rules file under root.
func GenerateCursorRulesAt(root, serviceName string) error {
	content := strings.ReplaceAll(cursorRulesTemplate, "{{SERVICE}}", serviceName)
	path := filepath.Join(root, ".cursor", "rules", "zengo-service.mdc")
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

const cursorRulesTemplate = `---
description: Zengo service development conventions for {{SERVICE}}
globs: "**/*.{go,proto,sql,yaml,yml,textproto,pbtxt}"
---
- Business logic lives only in internal/**/handler.go (hub API).
- Edit only api/hub/ for active API development; api/vN/ directories are frozen legacy contracts.
- Never edit files under gen/.
- Run zengo gen or mage gen after changing .proto, queries, or zengo manifest/config files.
- configs/* and zengo.* accept YAML or prototext (.textproto/.pbtxt); schema is defined in proto.
- Canonical postgres config kind is postgres.
`

// GenerateBufRegistryDoc writes the default Buf registry guide into the current root.
func GenerateBufRegistryDoc(serviceName string) error {
	return GenerateBufRegistryDocAt(".", serviceName)
}

// GenerateBufRegistryDocAt writes the default Buf registry guide under root.
func GenerateBufRegistryDocAt(root, serviceName string) error {
	doc := fmt.Sprintf(`# Proto registry

Service %q should publish protos to Buf Schema Registry:

`+"```bash"+`
buf push --label %s
`+"```"+`

Breaking change detection:

`+"```bash"+`
buf breaking --against '.git#branch=main'
`+"```"+`
`, serviceName, serviceName)
	path := filepath.Join(root, "docs", "PROTO_REGISTRY.md")
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(doc), 0o644)
}
