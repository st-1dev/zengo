package codegen

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

// File describes one generated file to render from a named template.
type File struct {
	// Template is the built-in template filename.
	Template string
	// Data is the template execution context.
	Data any
	// OutputPath is the target file path used for overrides and formatting behavior.
	OutputPath string
}

// Render executes a template, formats Go output when needed, and writes the file.
func Render(file File) error {
	src, err := executeTemplate(file.Template, file.Data, file.OutputPath)
	if err != nil {
		return err
	}
	if strings.HasSuffix(file.OutputPath, ".go") {
		src, err = FormatSource(src, file.OutputPath)
		if err != nil {
			return err
		}
	}
	err = os.MkdirAll(filepath.Dir(file.OutputPath), 0o755)
	if err != nil {
		return err
	}
	return os.WriteFile(file.OutputPath, src, 0o644)
}

func executeTemplate(name string, data any, outputPath string) ([]byte, error) {
	tmpl, err := parseTemplate(name, outputPath)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func parseTemplate(name, outputPath string) (*template.Template, error) {
	overridePath, ok, err := findOverrideTemplate(name, outputPath)
	if err != nil {
		return nil, err
	}
	base := template.New(name).Funcs(Funcs())
	if ok {
		src, err := os.ReadFile(overridePath)
		if err != nil {
			return nil, err
		}
		return base.Parse(string(src))
	}
	var tmpl *template.Template
	tmpl, err = template.New(name).Funcs(Funcs()).ParseFS(templatesFS, "templates/"+name)
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}

func findOverrideTemplate(name, outputPath string) (string, bool, error) {
	if outputPath == "" {
		return "", false, nil
	}
	dir := filepath.Dir(outputPath)
	for {
		candidate := filepath.Join(dir, ".zengo", "templates", name)
		_, err := os.Stat(candidate)
		if err == nil {
			return candidate, true, nil
		} else if err != nil && !os.IsNotExist(err) {
			return "", false, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false, nil
}

// FormatSource formats generated Go source and prefers goimports inside the repo.
func FormatSource(src []byte, filename string) ([]byte, error) {
	if shouldUseGoimports(filename) {
		formatted, err := imports.Process(filename, src, &imports.Options{
			Comments:  true,
			TabIndent: true,
			TabWidth:  8,
		})
		if err == nil {
			return formatted, nil
		}
	}

	gofmtSrc, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w", err)
	}
	return gofmtSrc, nil
}

func shouldUseGoimports(filename string) bool {
	if filename == "" {
		return true
	}
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return true
	}
	var wd string

	wd, err = os.Getwd()
	if err != nil {
		return true
	}
	var rel string

	rel, err = filepath.Rel(wd, absPath)
	if err != nil {
		return true
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}
