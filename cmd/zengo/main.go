package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// main dispatches subcommands for the zengo CLI.
func main() {
	os.Exit(runCLI())
}

func runCLI() int {
	commander := newCommander()
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return int(commander.Execute(ctx))
}

type genCommandOptions struct {
	ctx          context.Context
	dir          string
	manifestPath string
	skipBuf      bool
	protoOnly    bool
	wireOnly     bool
	skipSQLC     bool
	skipCQL      bool
	skipMain     bool
}

// executeGen runs the high-level generation pipeline for a service.
func executeGen(opts genCommandOptions) {
	genOpts := genOptions{
		ctx:       opts.ctx,
		skipBuf:   opts.skipBuf,
		protoOnly: opts.protoOnly,
		wireOnly:  opts.wireOnly,
		skipSQLC:  opts.skipSQLC,
		skipCQL:   opts.skipCQL,
		skipMain:  opts.skipMain,
	}
	err := runGenerateAll(opts.dir, opts.manifestPath, genOpts)
	if err != nil {
		fail(1, "%v", err)
	}
	fmt.Println("generation complete")
}
