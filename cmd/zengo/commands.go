package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"zengo/platform/internal/manifest"
	"zengo/platform/internal/scaffold"
	"zengo/platform/internal/versioning"
)

type initArgs struct {
	name           string
	dir            string
	noGen          bool
	manifestFormat scaffold.ManifestFormat
}

// runVersion dispatches zengo version subcommands.
func runVersion(args []string) {
	if len(args) == 0 || args[0] != "freeze" {
		fail(1, "usage: zengo version freeze <vN>")
	}
	if len(args) < 2 {
		fail(1, "version required, e.g. v2")
	}
	err := freezeVersion(".", args[1])
	if err != nil {
		fail(1, "freeze failed: %v", err)
	}
	fmt.Printf("frozen api/hub -> api/%s\n", args[1])
}

// freezeVersion snapshots api/hub into a new legacy version and enables compatibility mode.
func freezeVersion(rootDir, version string) error {
	err := versioning.Freeze(
		filepath.Join(rootDir, "api"),
		version,
		versioning.ReadModuleAt(filepath.Join(rootDir, "go.mod")),
		filepath.Join(rootDir, "gen"),
	)
	if err != nil {
		return err
	}
	var manifestPath string

	manifestPath, err = manifest.FindPath(rootDir)
	if err != nil {
		return err
	}
	return manifest.Update(manifestPath, func(m *manifest.Manifest) error {
		if m.Compatibility == nil {
			m.Compatibility = &manifest.Compatibility{}
		}
		m.Compatibility.LegacyVersions = manifest.CompatibilityEnabled
		return nil
	})
}

func executeInit(ctx context.Context, parsed initArgs) {
	err := scaffold.InitService(parsed.name, parsed.dir, scaffold.InitOptions{ManifestFormat: parsed.manifestFormat})
	if err != nil {
		fail(1, "init failed: %v", err)
	}
	if parsed.noGen {
		fmt.Printf("service %q scaffolded at %s (run: cd %s && zengo gen)\n", parsed.name, parsed.dir, parsed.dir)
		return
	}
	err = finalizeInit(ctx, parsed.dir)
	if err != nil {
		fail(1, `init scaffold ok, finalize failed: %v
		try: cd %s && buf dep update && zengo gen`, err, parsed.dir)
	}
	fmt.Printf(`service %q ready at %s
	next: cd %s && mage up && mage dev
`, parsed.name, parsed.dir, parsed.dir)
}

// parseInitArgs parses positional and flag-like arguments for zengo init.
func parseInitArgs(args []string) (initArgs, error) {
	return parseInitArgsWithDefaults(args, initArgs{manifestFormat: scaffold.ManifestFormatTextproto})
}

// parseInitArgsWithDefaults parses zengo init args starting from pre-parsed defaults.
func parseInitArgsWithDefaults(args []string, defaults initArgs) (initArgs, error) {
	parsed := defaults
	if parsed.manifestFormat == "" {
		parsed.manifestFormat = scaffold.ManifestFormatTextproto
	}
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--no-gen" || arg == "--no-gen=true":
			parsed.noGen = true
		case arg == "--no-gen=false":
			parsed.noGen = false
		case arg == "--manifest-format":
			remaining := args[i+1:]
			if len(remaining) == 0 {
				return initArgs{}, fmt.Errorf("missing value for --manifest-format")
			}
			format, err := scaffold.ParseManifestFormat(remaining[0])
			if err != nil {
				return initArgs{}, err
			}
			parsed.manifestFormat = format
			i++
		case strings.HasPrefix(arg, "--manifest-format="):
			format, err := scaffold.ParseManifestFormat(strings.TrimPrefix(arg, "--manifest-format="))
			if err != nil {
				return initArgs{}, err
			}
			parsed.manifestFormat = format
		default:
			if strings.HasPrefix(arg, "-") {
				return initArgs{}, fmt.Errorf("unknown flag %q", arg)
			}
			positional = append(positional, arg)
		}
	}
	if len(positional) == 0 {
		return initArgs{}, fmt.Errorf("service name required")
	}
	if len(positional) > 2 {
		return initArgs{}, fmt.Errorf("too many arguments: %s", strings.Join(positional[2:], " "))
	}
	parsed.name = positional[0]
	parsed.dir = parsed.name
	if len(positional) == 2 {
		parsed.dir = positional[1]
	}
	return parsed, nil
}
