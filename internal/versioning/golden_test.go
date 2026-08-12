package versioning

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateUserServiceGolden(t *testing.T) {
	InvalidateLoaderCache()

	exampleRoot := filepath.Clean(filepath.Join("..", "..", "examples", "user-service"))
	st, err := os.Stat(exampleRoot)
	if err != nil || !st.IsDir() {
		t.Skip("examples/user-service not available")
	}

	apiRoot := filepath.Join(exampleRoot, "api")
	genAPI := filepath.Join(exampleRoot, "gen", "api")
	_, err = os.Stat(genAPI)
	if err != nil {
		t.Skip("examples/user-service/gen/api not available; run buf generate in example first")
	}

	tmp := t.TempDir()
	genRoot := filepath.Join(tmp, "gen")
	err = copyTree(genAPI, filepath.Join(genRoot, "api"))
	if err != nil {
		t.Fatal(err)
	}

	opts := GenerateOptions{
		Module:   "github.com/zengo/user-service",
		APIRoot:  apiRoot,
		GenDir:   genRoot,
		Internal: filepath.Join(exampleRoot, "internal"),
	}
	err = Generate(opts)
	if err != nil {
		t.Fatal(err)
	}

	goldenRoot := filepath.Join("testdata", "golden", "user-service", "gen", "zengo")
	gotRoot := filepath.Join(genRoot, "zengo")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		err = os.RemoveAll(goldenRoot)
		if err != nil {
			t.Fatal(err)
		}
		err = copyTree(gotRoot, goldenRoot)
		if err != nil {
			t.Fatal(err)
		}
		t.Skip("golden files updated; re-run without UPDATE_GOLDEN=1")
	}

	err = compareTree(gotRoot, goldenRoot)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGenerateWireRemovesStaleFileWithoutLegacyVersions(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "zengo", "legacy_wire_gen.go")
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(path, []byte("stale"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	err = generateWire(Layout{}, GenerateOptions{GenDir: tmp})
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(path)
	if !os.IsNotExist(err) {
		t.Fatalf("expected stale legacy wire file removed, got %v", err)
	}
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		var rel string

		rel, err = filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		var data []byte

		data, err = os.ReadFile(path)
		if err != nil {
			return err
		}
		err = os.MkdirAll(filepath.Dir(target), 0o755)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func compareTree(gotRoot, goldenRoot string) error {
	return filepath.WalkDir(goldenRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		var rel string

		rel, err = filepath.Rel(goldenRoot, path)
		if err != nil {
			return err
		}
		gotPath := filepath.Join(gotRoot, rel)
		var want []byte

		want, err = os.ReadFile(path)
		if err != nil {
			return err
		}
		var got []byte

		got, err = os.ReadFile(gotPath)
		if err != nil {
			return os.ErrNotExist
		}
		if !bytes.Equal(want, got) {
			return &goldenDiffError{file: rel}
		}
		return nil
	})
}

type goldenDiffError struct {
	file string
}

func (e *goldenDiffError) Error() string {
	return "generated output differs from golden: " + e.file + " (run UPDATE_GOLDEN=1 go test ./internal/versioning -run Golden)"
}
