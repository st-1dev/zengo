//go:build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	apiDir     = "api"
	binDir     = ".bin"
	exampleDir = "examples/user-service"
)

func All() error {
	err := Gen()
	if err != nil {
		return err
	}
	return Build()
}

func ZengoProto() error {
	err := buildProtocGenGoZZ()
	if err != nil {
		return err
	}
	return generateAt(apiDir, "zengo/buf.gen.yaml", "zengo")
}

func ProtocGenGoZz() error {
	return buildProtocGenGoZZ()
}

func ConfigModel() error {
	err := buildProtocGenGoZZ()
	if err != nil {
		return err
	}
	return generateAt(apiDir, "config/buf.gen.yaml", "config")
}

func ProtocGenZengo() error {
	err := ZengoProto()
	if err != nil {
		return err
	}
	err = ensureDir(binDir)
	if err != nil {
		return err
	}
	return run(".", pathEnv(filepath.Join(".", binDir)), "go", "build", "-o", "./.bin/protoc-gen-zengo", "./cmd/protoc-gen-zengo")
}

func ProtocGenOpenAPIV2() error {
	err := ensureDir(binDir)
	if err != nil {
		return err
	}
	return run(".", pathEnv(filepath.Join(".", binDir)), "go", "build", "-o", "./.bin/protoc-gen-openapiv2", "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2")
}

func Zengo() error {
	err := ProtocGenZengo()
	if err != nil {
		return err
	}
	err = ProtocGenOpenAPIV2()
	if err != nil {
		return err
	}
	err = ensureDir(binDir)
	if err != nil {
		return err
	}
	return run(".", pathEnv(filepath.Join(".", binDir)), "go", "build", "-o", "./.bin/zengo", "./cmd/zengo")
}

func GenPlatform() error {
	err := ConfigModel()
	if err != nil {
		return err
	}
	return Zengo()
}

func GenService() error {
	err := GenPlatform()
	if err != nil {
		return err
	}
	err = run(exampleDir, pathEnv(filepath.Join(".", binDir)), "buf", "dep", "update")
	if err != nil {
		return err
	}
	return run(exampleDir, pathEnv(filepath.Join(".", binDir)), filepath.FromSlash("../../.bin/zengo"), "gen")
}

func Gen() error {
	err := GenPlatform()
	if err != nil {
		return err
	}
	return GenService()
}

func Build() error {
	err := run(".", pathEnv(filepath.Join(".", binDir)), "go", "build", "./...")
	if err != nil {
		return err
	}
	err = ensureDir(filepath.Join(exampleDir, "bin"))
	if err != nil {
		return err
	}
	return run(exampleDir, pathEnv(filepath.Join(".", binDir)), "go", "build", "-ldflags", buildInfoLDFlags(exampleDir), "-o", "bin/user-service", "./cmd")
}

func Test() error {
	err := run(".", pathEnv(filepath.Join(".", binDir)), "go", "test", "./...")
	if err != nil {
		return err
	}
	return run(exampleDir, pathEnv(filepath.Join(".", binDir)), "go", "test", "./...")
}

func Tidy() error {
	err := run(".", pathEnv(filepath.Join(".", binDir)), "go", "mod", "tidy")
	if err != nil {
		return err
	}
	return run(exampleDir, pathEnv(filepath.Join(".", binDir)), "go", "mod", "tidy")
}

func Check() error {
	err := Zengo()
	if err != nil {
		return err
	}
	err = run(".", pathEnv(filepath.Join(".", binDir)), "./.bin/zengo", "check", "--breaking")
	if err != nil {
		return err
	}
	return run(exampleDir, pathEnv(filepath.Join(".", binDir)), filepath.FromSlash("../../.bin/zengo"), "check", "--breaking")
}

func Dev() error {
	err := Zengo()
	if err != nil {
		return err
	}
	return run(exampleDir, pathEnv(filepath.Join(".", binDir)), filepath.FromSlash("../../.bin/zengo"), "dev")
}

func buildProtocGenGoZZ() error {
	err := ensureDir(binDir)
	if err != nil {
		return err
	}
	return run(".", pathEnv(filepath.Join(".", binDir)), "go", "build", "-o", "./.bin/protoc-gen-go-zz", "./cmd/protoc-gen-go-zz")
}

func generateAt(dir, template, path string) error {
	return run(dir, pathEnv(filepath.Join(".", binDir)), "buf", "generate", "--template", template, "--path", path)
}

func buildInfoLDFlags(dir string) string {
	version := gitValue(dir, "describe", "--tags", "--always", "--dirty")
	branch := gitValue(dir, "rev-parse", "--abbrev-ref", "HEAD")
	return fmt.Sprintf("-X zengo/platform/sdk/buildinfo.Version=%s -X zengo/platform/sdk/buildinfo.Branch=%s", version, branch)
}

func gitValue(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func pathEnv(entries ...string) []string {
	pathParts := make([]string, 0, len(entries)+2)
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		pathParts = append(pathParts, absPath(entry))
	}
	gopathBin := strings.TrimSpace(goEnv("GOPATH"))
	if gopathBin != "" {
		pathParts = append(pathParts, filepath.Join(gopathBin, "bin"))
	}
	pathParts = append(pathParts, os.Getenv("PATH"))
	env := os.Environ()
	pathKey := "PATH=" + strings.Join(pathParts, string(os.PathListSeparator))
	replaced := false
	for i, item := range env {
		if strings.HasPrefix(item, "PATH=") {
			env[i] = pathKey
			replaced = true
			break
		}
	}
	if !replaced {
		env = append(env, pathKey)
	}
	return env
}

func goEnv(name string) string {
	out, err := exec.Command("go", "env", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

func absPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func run(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
