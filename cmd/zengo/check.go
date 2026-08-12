package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"zengo/platform/internal/manifest"
)

type checkCommandOptions struct {
	ctx          context.Context
	dir          string
	manifestPath string
	skipTests    bool
	skipGen      bool
	skipBuf      bool
	skipLint     bool
	breaking     bool
	against      string
}

// executeCheck runs service-level or platform-level validation checks.
func executeCheck(opts checkCommandOptions) {
	if opts.manifestPath != "" || isServiceRootAt(opts.dir) {
		runServiceCheck(
			opts.ctx,
			opts.dir,
			opts.manifestPath,
			opts.skipTests,
			opts.skipGen,
			opts.skipBuf,
			opts.skipLint,
			opts.breaking,
			opts.against,
		)
		return
	}
	runPlatformCheck(opts.ctx, opts.skipTests, opts.skipLint, opts.breaking, opts.against)
}

// runServiceCheck validates a service repository, including freshness and optional tests.
func runServiceCheck(
	ctx context.Context,
	dir, manifestPath string,
	skipTests, skipGen, skipBuf, skipLint, breaking bool,
	against string,
) {
	paths, err := resolveServicePaths(dir, manifestPath)
	if err != nil {
		fail(1, "manifest: %v", err)
	}
	fmt.Println("check: manifest")
	_, err = manifest.Load(paths.manifestPath)
	if err != nil {
		fail(1, "manifest: %v", err)
	}

	if !skipLint && fileExists(filepath.Join(paths.rootDir, "buf.yaml")) {
		fmt.Println("check: buf lint")
		err = runCommandDir(ctx, paths.rootDir, "buf", "lint")
		if err != nil {
			fail(1, "buf lint failed: %v", err)
		}
	}

	if breaking && fileExists(filepath.Join(paths.rootDir, "buf.yaml")) && gitAvailable(ctx) {
		fmt.Println("check: buf breaking", against)
		err = runCommandDir(ctx, paths.rootDir, "buf", "breaking", "--against", against)
		if err != nil {
			fail(1, "buf breaking failed: %v", err)
		}
	}

	if !skipGen {
		fmt.Println("check: generated code")
		err = checkGenFreshIn(ctx, paths.rootDir, paths.manifestPath, skipBuf)
		if err != nil {
			fail(1, "%v", err)
		}
	}

	fmt.Println("check: go vet ./...")
	err = runCommandDir(ctx, paths.rootDir, "go", "vet", "./...")
	if err != nil {
		fail(12, "go vet failed: %v", err)
	}

	if !skipTests {
		fmt.Println("check: go test ./...")
		err = runCommandDir(ctx, paths.rootDir, "go", "test", "./...")
		if err != nil {
			fail(1, "go test failed: %v", err)
		}
	}

	fmt.Println("check: ok")
}

// runPlatformCheck validates the platform repository outside a service context.
func runPlatformCheck(ctx context.Context, skipTests, skipLint, breaking bool, against string) {
	if fileExists("api/buf.yaml") && !skipLint {
		fmt.Println("check: buf lint (api/)")
		err := runCommandDir(ctx, ".", "buf", "lint", "--path", "api")
		if err != nil {
			fail(1, "buf lint failed")
		}
	}

	if breaking && fileExists("api/buf.yaml") && gitAvailable(ctx) {
		fmt.Println("check: buf breaking (api/)", against)
		err := runCommandDir(ctx, ".", "buf", "breaking", "--against", against, "--path", "api")
		if err != nil {
			fail(1, "buf breaking failed: %v", err)
		}
	}

	fmt.Println("check: go vet ./...")
	err := runCommand(ctx, "go", "vet", "./...")
	if err != nil {
		fail(1, "go vet failed: %v", err)
	}

	if !skipTests {
		fmt.Println("check: go test ./...")
		err = runCommand(ctx, "go", "test", "./...")
		if err != nil {
			fail(1, "go test failed: %v", err)
		}
	}

	fmt.Println("check: ok")
}

// gitAvailable reports whether git metadata is available for breaking checks.
func gitAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}
