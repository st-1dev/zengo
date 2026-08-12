package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type devCommandOptions struct {
	ctx          context.Context
	dir          string
	manifestPath string
	skipBuf      bool
	cmdPath      string
	interval     time.Duration
}

// executeDev watches service inputs, regenerates artifacts, and restarts go run on change.
func executeDev(opts devCommandOptions) {
	paths, err := resolveServicePaths(opts.dir, opts.manifestPath)
	if err != nil {
		fail(1, "zengo dev: run from a service directory or pass --dir/--manifest")
	}

	watchRoots := devWatchRoots(paths.rootDir)
	snap := snapshotModTimes(watchRoots)

	ctx := opts.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	var (
		mu      sync.Mutex
		runCmd  *exec.Cmd
		running bool
	)

	reload := func(reason string) {
		mu.Lock()
		defer mu.Unlock()
		if running {
			return
		}
		running = true
		defer func() { running = false }()

		fmt.Printf("\n>>> reload (%s)\n", reason)
		err = runGenerateAll(
			paths.rootDir,
			paths.manifestPath,
			genOptions{ctx: ctx, skipBuf: opts.skipBuf, quietMain: true},
		)
		if err != nil {
			fail(1, "gen failed: %v", err)
		}

		if runCmd != nil && runCmd.Process != nil {
			_ = runCmd.Process.Signal(os.Interrupt)
			_, _ = runCmd.Process.Wait()
		}

		runCmd = exec.CommandContext(ctx, "go", "run", opts.cmdPath)
		runCmd.Dir = paths.rootDir
		runCmd.Env = os.Environ()
		runCmd.Stdout = os.Stdout
		runCmd.Stderr = os.Stderr
		runCmd.Stdin = os.Stdin
		err = runCmd.Start()
		if err != nil {
			fail(1, "go run failed: %v", err)
			runCmd = nil
		}
	}

	fmt.Println("zengo dev: watching", strings.Join(watchRoots, ", "))
	reload("start")

	ticker := time.NewTicker(opts.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			mu.Lock()
			if runCmd != nil && runCmd.Process != nil {
				_ = runCmd.Process.Signal(os.Interrupt)
			}
			mu.Unlock()
			return
		case <-ticker.C:
			next := snapshotModTimes(watchRoots)
			changed := diffModTimes(snap, next)
			if changed != "" {
				snap = next
				reload(changed)
			}
		}
	}
}

// devWatchRoots lists the directories and files watched by zengo dev.
func devWatchRoots(rootDir string) []string {
	var roots []string
	for _, name := range []string{"api", "internal", "queries", "cql", "configs"} {
		path := filepath.Join(rootDir, name)
		if fileExists(path) {
			roots = append(roots, path)
		}
	}
	for _, name := range []string{
		"zengo.yaml",
		"zengo.yml",
		"zengo.textproto",
		"zengo.pbtxt",
		"sqlc.yaml",
		"cqlc.yaml",
		"buf.gen.yaml",
		"buf.yaml",
	} {
		path := filepath.Join(rootDir, name)
		if fileExists(path) {
			roots = append(roots, path)
		}
	}
	return roots
}

// snapshotModTimes records modification times for the current watch set.
func snapshotModTimes(roots []string) map[string]time.Time {
	out := map[string]time.Time{}
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			out[root] = info.ModTime()
			continue
		}

		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, _ error) error {
			if d == nil || d.IsDir() {
				return nil
			}

			var fi fs.FileInfo
			supportedExts := []string{
				".go",
				".proto",
				".yaml",
				".yml",
				".sql",
				".textproto",
				".pbtxt",
			}

			ext := filepath.Ext(path)
			if slices.Contains(supportedExts, ext) {
				fi, err = d.Info()
				if err == nil {
					out[path] = fi.ModTime()
				}
			}
			return nil
		})
	}
	return out
}

// diffModTimes reports the first detected file change between two snapshots.
func diffModTimes(before, after map[string]time.Time) string {
	for path, t := range after {
		prev, ok := before[path]
		if !ok || t.After(prev) {
			return path
		}
	}
	for path := range before {
		_, ok := after[path]
		if !ok {
			return path + " (removed)"
		}
	}
	return ""
}
