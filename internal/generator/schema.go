package generator

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const schemaFileName = "schema_gen.json"

// RepositorySchemaManifest stores normalized repository schema metadata generated
// from canonical hub protobuf files.
type RepositorySchemaManifest struct {
	// Repositories lists one schema entry per repository-enabled hub service.
	Repositories []RepositorySchema `json:"repositories"`
}

// RepositorySchema describes one repository-backed table derived from proto
// annotations.
type RepositorySchema struct {
	// File is the source proto path that declared the repository metadata.
	File string `json:"file"`
	// Package is the proto package that owns the repository service.
	Package string `json:"package"`
	// Service is the proto service name that declared the repository metadata.
	Service string `json:"service"`
	// Entity is the logical repository entity name.
	Entity string `json:"entity"`
	// Table is the physical Postgres table name.
	Table string `json:"table"`
	// Model is the proto message that defines the persistence schema.
	Model string `json:"model"`
	// Columns stores the resolved column list in proto declaration order.
	Columns []RepositoryColumn `json:"columns"`
}

// RepositoryColumn describes one resolved database column.
type RepositoryColumn struct {
	// ProtoField is the source proto field name.
	ProtoField string `json:"proto_field"`
	// Name is the generated column name.
	Name string `json:"name"`
	// SQLType is the resolved Postgres column type.
	SQLType string `json:"sql_type"`
	// PrimaryKey reports whether the column is the table primary key.
	PrimaryKey bool `json:"primary_key"`
	// Nullable reports whether the column may omit NOT NULL.
	Nullable bool `json:"nullable"`
	// Unique reports whether the column carries a UNIQUE constraint.
	Unique bool `json:"unique"`
	// DefaultSQL stores the raw SQL expression used in the DEFAULT clause.
	DefaultSQL string `json:"default_sql,omitempty"`
}

// RepositorySchemaPathAt returns the generated schema metadata path for root.
func RepositorySchemaPathAt(root string) string {
	return filepath.Join(root, "gen", "zengo", schemaFileName)
}

// EncodeRepositorySchema writes schema metadata as deterministic indented JSON.
func EncodeRepositorySchema(w io.Writer, manifest RepositorySchemaManifest) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(manifest)
	if err != nil {
		return fmt.Errorf("encode repository schema: %w", err)
	}
	return nil
}

// ReadRepositorySchemaAt loads generated schema metadata from root.
func ReadRepositorySchemaAt(root string) (RepositorySchemaManifest, error) {
	path := RepositorySchemaPathAt(root)
	data, err := os.ReadFile(path)
	if err != nil {
		return RepositorySchemaManifest{}, err
	}
	var manifest RepositorySchemaManifest

	err = json.Unmarshal(data, &manifest)
	if err != nil {
		return RepositorySchemaManifest{}, fmt.Errorf("decode repository schema: %w", err)
	}
	if manifest.Repositories == nil {
		manifest.Repositories = []RepositorySchema{}
	}
	return manifest, nil
}

// HasRepositorySchemasAt reports whether generated schema metadata contains at
// least one repository entry.
func HasRepositorySchemasAt(root string) (bool, error) {
	manifest, err := ReadRepositorySchemaAt(root)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return len(manifest.Repositories) > 0, nil
}
