package main

import (
	"context"
	"fmt"
	"path/filepath"
	"zengo/platform/internal/scaffold"
)

type addCommandOptions struct {
	ctx     context.Context
	dir     string
	gen     bool
	targets []string
}

// executeAdd patches a service manifest and optional templates for one or more targets.
func executeAdd(opts addCommandOptions) {
	if len(opts.targets) == 0 {
		fail(1,
			`usage: zengo add <target> [target...]
targets: postgres, kafka, nats, rabbitmq, grpc, rest, observability, auth, redis, s3, cassandra`,
		)
	}
	for _, target := range opts.targets {
		err := scaffold.Add(opts.dir, target)
		if err != nil {
			fail(1, "add %s: %v", target, err)
		}
		fmt.Println("added", target)
	}
	if opts.gen {
		absDir, err := filepath.Abs(opts.dir)
		if err != nil {
			fail(1, "resolve dir: %v", err)
		}
		if isServiceRootIn(absDir) {
			var paths servicePaths
			paths, err = resolveServicePaths(absDir, "")
			if err != nil {
				fail(1, "resolve service: %v", err)
			}
			err = runGenerateAll(paths.rootDir, paths.manifestPath, genOptions{ctx: opts.ctx, quietMain: true})
			if err != nil {
				fail(1, "gen after add failed: %v", err)
			}
		}
	}
}

// isServiceRootIn reports whether dir looks like a Zengo service root.
func isServiceRootIn(dir string) bool {
	for _, name := range []string{"zengo.yaml", "zengo.yml", "zengo.textproto", "zengo.pbtxt"} {
		if fileExists(filepath.Join(dir, name)) {
			return true
		}
	}
	return fileExists(filepath.Join(dir, "buf.gen.yaml")) && fileExists(filepath.Join(dir, "api"))
}
