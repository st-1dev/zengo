package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
	"zengo/platform/internal/scaffold"

	"github.com/google/subcommands"
)

func newCommander() *subcommands.Commander {
	commander := subcommands.NewCommander(flag.CommandLine, "zengo")
	commander.Explain = func(w io.Writer) {
		writeUsage(w)
	}

	commander.Register(commander.HelpCommand(), "")
	commander.Register(commander.FlagsCommand(), "")
	commander.Register(commander.CommandsCommand(), "")
	commander.Register(&initCommand{}, "")
	commander.Register(&genCommand{}, "")
	commander.Register(&checkCommand{}, "")
	commander.Register(&devCommand{}, "")
	commander.Register(&addCommand{}, "")
	commander.Register(&versionCommand{}, "")

	return commander
}

func writeUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Usage:
  zengo init [--no-gen] [--manifest-format textproto|yaml] <service-name> [dir]
  zengo gen [--dir DIR] [--manifest PATH] [--skip-buf] [--proto-only|--wire-only] [--skip-sqlc] [--skip-main]
  zengo check [--dir DIR] [--breaking] [--against REF]
  zengo dev [--dir DIR] [--cmd ./cmd] [--skip-buf]
  zengo add <target>...   # postgres, kafka, grpc, observability, ...
  zengo version freeze <vN>
  zengo help [command]`)
}

type initCommand struct {
	noGen          bool
	manifestFormat string
}

func (*initCommand) Name() string {
	return "init"
}

func (*initCommand) Synopsis() string {
	return "Scaffold a new service."
}

func (*initCommand) Usage() string {
	return "init [--no-gen] [--manifest-format textproto|yaml] <service-name> [dir]\n"
}

func (c *initCommand) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&c.noGen, "no-gen", false, "skip post-scaffold generation")
	fs.StringVar(
		&c.manifestFormat,
		"manifest-format",
		string(scaffold.ManifestFormatTextproto),
		"service manifest format: textproto or yaml",
	)
}

func (c *initCommand) Execute(ctx context.Context, fs *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	defaultFormat, err := scaffold.ParseManifestFormat(c.manifestFormat)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return subcommands.ExitUsageError
	}
	var parsed initArgs

	parsed, err = parseInitArgsWithDefaults(fs.Args(), initArgs{
		noGen:          c.noGen,
		manifestFormat: defaultFormat,
	})
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return subcommands.ExitUsageError
	}

	executeInit(ctx, parsed)
	return subcommands.ExitSuccess
}

type genCommand struct {
	dir          string
	manifestPath string
	skipBuf      bool
	protoOnly    bool
	wireOnly     bool
	skipSQLC     bool
	skipCQL      bool
	skipMain     bool
}

func (*genCommand) Name() string {
	return "gen"
}

func (*genCommand) Synopsis() string {
	return "Run code generation for a service."
}

func (*genCommand) Usage() string {
	return "gen [--dir DIR] [--manifest PATH] [--skip-buf] [--proto-only|--wire-only] [--skip-sqlc] [--skip-main]\n"
}

func (c *genCommand) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.dir, "dir", ".", "service directory")
	fs.StringVar(&c.manifestPath, "manifest", "", "path to service manifest (auto-discover when empty)")
	fs.BoolVar(&c.skipBuf, "skip-buf", false, "skip buf generate")
	fs.BoolVar(&c.protoOnly, "proto-only", false, "run buf generate only")
	fs.BoolVar(
		&c.wireOnly,
		"wire-only",
		false,
		"run zengo runtime and legacy compatibility generation only (skip buf/sqlc/cql)",
	)
	fs.BoolVar(&c.skipSQLC, "skip-sqlc", false, "skip sqlc generate")
	fs.BoolVar(&c.skipCQL, "skip-cql", false, "skip cql generate")
	fs.BoolVar(&c.skipMain, "skip-main", false, "skip cmd/main.go generation")
}

func (c *genCommand) Execute(ctx context.Context, _ *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	executeGen(genCommandOptions{
		ctx:          ctx,
		dir:          c.dir,
		manifestPath: c.manifestPath,
		skipBuf:      c.skipBuf,
		protoOnly:    c.protoOnly,
		wireOnly:     c.wireOnly,
		skipSQLC:     c.skipSQLC,
		skipCQL:      c.skipCQL,
		skipMain:     c.skipMain,
	})
	return subcommands.ExitSuccess
}

type checkCommand struct {
	dir          string
	manifestPath string
	skipTests    bool
	skipGen      bool
	skipBuf      bool
	skipLint     bool
	breaking     bool
	against      string
}

func (*checkCommand) Name() string {
	return "check"
}

func (*checkCommand) Synopsis() string {
	return "Run service or platform validation checks."
}

func (*checkCommand) Usage() string {
	return "check [--dir DIR] [--manifest PATH] [--skip-test] [--skip-gen] [--skip-buf] [--skip-lint] [--breaking] [--against REF]\n"
}

func (c *checkCommand) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.dir, "dir", ".", "service directory")
	fs.StringVar(&c.manifestPath, "manifest", "", "path to service manifest (auto-discover when empty)")
	fs.BoolVar(&c.skipTests, "skip-test", false, "skip go test")
	fs.BoolVar(&c.skipGen, "skip-gen", false, "skip generated-code freshness check")
	fs.BoolVar(&c.skipBuf, "skip-buf", false, "pass --skip-buf to gen freshness check")
	fs.BoolVar(&c.skipLint, "skip-lint", false, "skip buf lint")
	fs.BoolVar(&c.breaking, "breaking", false, "run buf breaking against git ref")
	fs.StringVar(&c.against, "against", ".git#branch=main", "buf breaking comparison ref")
}

func (c *checkCommand) Execute(ctx context.Context, _ *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	executeCheck(checkCommandOptions{
		ctx:          ctx,
		dir:          c.dir,
		manifestPath: c.manifestPath,
		skipTests:    c.skipTests,
		skipGen:      c.skipGen,
		skipBuf:      c.skipBuf,
		skipLint:     c.skipLint,
		breaking:     c.breaking,
		against:      c.against,
	})
	return subcommands.ExitSuccess
}

type devCommand struct {
	dir          string
	manifestPath string
	skipBuf      bool
	cmdPath      string
	interval     string
}

func (*devCommand) Name() string {
	return "dev"
}

func (*devCommand) Synopsis() string {
	return "Watch service files, regenerate, and restart go run."
}

func (*devCommand) Usage() string {
	return "dev [--dir DIR] [--manifest PATH] [--cmd ./cmd] [--skip-buf] [--interval 1500ms]\n"
}

func (c *devCommand) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.dir, "dir", ".", "service directory")
	fs.StringVar(&c.manifestPath, "manifest", "", "path to service manifest (auto-discover when empty)")
	fs.BoolVar(&c.skipBuf, "skip-buf", false, "skip buf generate on reload")
	fs.StringVar(&c.cmdPath, "cmd", "./cmd", "main package to run")
	fs.StringVar(&c.interval, "interval", "1500ms", "poll interval for file changes")
}

func (c *devCommand) Execute(ctx context.Context, _ *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	executeDev(devCommandOptions{
		ctx:          ctx,
		dir:          c.dir,
		manifestPath: c.manifestPath,
		skipBuf:      c.skipBuf,
		cmdPath:      c.cmdPath,
		interval:     mustParseDuration(c.interval),
	})
	return subcommands.ExitSuccess
}

type addCommand struct {
	dir string
	gen bool
}

func (*addCommand) Name() string {
	return "add"
}

func (*addCommand) Synopsis() string {
	return "Patch a service manifest with new components."
}

func (*addCommand) Usage() string {
	return "add [--dir DIR] [--gen=true] <target> [target...]\n"
}

func (c *addCommand) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.dir, "dir", ".", "service directory")
	fs.BoolVar(&c.gen, "gen", true, "run zengo gen after adding")
}

func (c *addCommand) Execute(ctx context.Context, fs *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	executeAdd(addCommandOptions{
		ctx:     ctx,
		dir:     c.dir,
		gen:     c.gen,
		targets: fs.Args(),
	})
	return subcommands.ExitSuccess
}

type versionCommand struct{}

func (*versionCommand) Name() string {
	return "version"
}

func (*versionCommand) Synopsis() string {
	return "Run version-related commands."
}

func (*versionCommand) Usage() string {
	return "version freeze <vN>\n"
}

func (*versionCommand) SetFlags(*flag.FlagSet) {}

func (*versionCommand) Execute(_ context.Context, fs *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	runVersion(fs.Args())
	return subcommands.ExitSuccess
}

func mustParseDuration(raw string) time.Duration {
	value, err := time.ParseDuration(raw)
	if err != nil {
		fail(1, "invalid --interval: %v", err)
	}
	return value
}
