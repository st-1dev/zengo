package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"zengo/platform/internal/manifest"
)

func TestGeneratePostgresMigrationAtCreatesManagedBaseline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRepositorySchemaFile(t, root, RepositorySchemaManifest{
		Repositories: []RepositorySchema{
			{
				Table: "users",
				Columns: []RepositoryColumn{
					{Name: "id", SQLType: "TEXT", PrimaryKey: true},
					{Name: "email", SQLType: "TEXT", Unique: true},
					{Name: "created_at", SQLType: "TIMESTAMPTZ", DefaultSQL: "NOW()"},
				},
			},
		},
	})

	m := &manifest.Manifest{DB: &manifest.DB{Postgres: &manifest.PostgresDB{}}}
	err := GeneratePostgresMigrationAt(root, m)
	if err != nil {
		t.Fatal(err)
	}
	body := readFile(t, PostgresMigrationPathAt(root))
	for _, want := range []string{
		generatedPostgresMigrationHeader,
		"CREATE TABLE IF NOT EXISTS users",
		"id TEXT PRIMARY KEY",
		"email TEXT NOT NULL UNIQUE",
		"created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("migration missing %q\n%s", want, body)
		}
	}
}

func TestGeneratePostgresMigrationAtSortsTablesDeterministically(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRepositorySchemaFile(t, root, RepositorySchemaManifest{
		Repositories: []RepositorySchema{
			{Table: "zebra", Columns: []RepositoryColumn{{Name: "id", SQLType: "TEXT", PrimaryKey: true}}},
			{Table: "aardvark", Columns: []RepositoryColumn{{Name: "id", SQLType: "TEXT", PrimaryKey: true}}},
		},
	})

	m := &manifest.Manifest{DB: &manifest.DB{Postgres: &manifest.PostgresDB{}}}
	err := GeneratePostgresMigrationAt(root, m)
	if err != nil {
		t.Fatal(err)
	}
	body := readFile(t, PostgresMigrationPathAt(root))
	if strings.Index(body, "aardvark") > strings.Index(body, "zebra") {
		t.Fatalf("tables are not sorted:\n%s", body)
	}
}

func TestGeneratePostgresMigrationAtRewritesManagedFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRepositorySchemaFile(t, root, RepositorySchemaManifest{
		Repositories: []RepositorySchema{
			{Table: "users", Columns: []RepositoryColumn{{Name: "id", SQLType: "TEXT", PrimaryKey: true}}},
		},
	})
	path := PostgresMigrationPathAt(root)
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(path, []byte(generatedPostgresMigrationHeader+"\nOLD"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	m := &manifest.Manifest{DB: &manifest.DB{Postgres: &manifest.PostgresDB{}}}
	err = GeneratePostgresMigrationAt(root, m)
	if err != nil {
		t.Fatal(err)
	}
	body := readFile(t, path)
	if strings.Contains(body, "OLD") {
		t.Fatalf("managed migration was not rewritten:\n%s", body)
	}
}

func TestGeneratePostgresMigrationAtPreservesManualFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRepositorySchemaFile(t, root, RepositorySchemaManifest{
		Repositories: []RepositorySchema{
			{Table: "users", Columns: []RepositoryColumn{{Name: "id", SQLType: "TEXT", PrimaryKey: true}}},
		},
	})
	path := PostgresMigrationPathAt(root)
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	const manual = "-- manual migration\nSELECT 1;\n"
	err = os.WriteFile(path, []byte(manual), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	m := &manifest.Manifest{DB: &manifest.DB{Postgres: &manifest.PostgresDB{}}}
	err = GeneratePostgresMigrationAt(root, m)
	if err != nil {
		t.Fatal(err)
	}
	body := readFile(t, path)
	if body != manual {
		t.Fatalf("manual migration changed:\n%s", body)
	}
}

func TestShouldTrackPostgresMigrationAt(t *testing.T) {
	t.Parallel()
	m := &manifest.Manifest{DB: &manifest.DB{Postgres: &manifest.PostgresDB{}}}
	t.Run("missing managed migration with schema", func(t *testing.T) {
		root := t.TempDir()
		writeRepositorySchemaFile(t, root, RepositorySchemaManifest{
			Repositories: []RepositorySchema{
				{Table: "users", Columns: []RepositoryColumn{{Name: "id", SQLType: "TEXT", PrimaryKey: true}}},
			},
		})
		track, err := ShouldTrackPostgresMigrationAt(root, m)
		if err != nil {
			t.Fatal(err)
		}
		if !track {
			t.Fatal("expected missing managed migration to be tracked")
		}
	})
	t.Run("manual migration", func(t *testing.T) {
		root := t.TempDir()
		writeRepositorySchemaFile(t, root, RepositorySchemaManifest{
			Repositories: []RepositorySchema{
				{Table: "users", Columns: []RepositoryColumn{{Name: "id", SQLType: "TEXT", PrimaryKey: true}}},
			},
		})
		path := PostgresMigrationPathAt(root)
		err := os.MkdirAll(filepath.Dir(path), 0o755)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile(path, []byte("-- manual\n"), 0o644)
		if err != nil {
			t.Fatal(err)
		}
		track, err := ShouldTrackPostgresMigrationAt(root, m)
		if err != nil {
			t.Fatal(err)
		}
		if track {
			t.Fatal("manual migration should not be tracked")
		}
	})
}

func writeRepositorySchemaFile(t *testing.T, root string, manifestData RepositorySchemaManifest) {
	t.Helper()
	path := RepositorySchemaPathAt(root)
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer

	err = EncodeRepositorySchema(&buf, manifestData)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(path, buf.Bytes(), 0o644)
	if err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
