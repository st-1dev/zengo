package versioning

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateOptions configures compatibility code generation for a service.
type GenerateOptions struct {
	// Module is the Go module path of the target service.
	Module string
	// APIRoot is the service api/ directory containing hub and optional legacy versions.
	APIRoot string
	// GenDir is the service gen/ directory where generated code is written.
	GenDir string
	// Internal is the service internal/ directory used to locate manual converters.
	Internal string
	// HubMeta is populated during generation and reused across generator stages.
	HubMeta HubMeta
	// Legacy controls whether legacy compatibility generation is enabled.
	Legacy CompatibilityMode
}

// CompatibilityMode controls whether legacy version adapters are generated.
type CompatibilityMode string

const (
	// CompatibilityAuto keeps discovered legacy versions as-is.
	CompatibilityAuto CompatibilityMode = ""
	// CompatibilityDisabled disables legacy compatibility generation.
	CompatibilityDisabled CompatibilityMode = "disabled"
	// CompatibilityEnabled forces generation for discovered legacy versions.
	CompatibilityEnabled CompatibilityMode = "enabled"
)

// Generate produces legacy conversion, adapter, wire, and runtime code for a service.
func Generate(opts GenerateOptions) error {
	layout, err := Discover(opts.APIRoot)
	if err != nil {
		return err
	}
	layout.Legacy = legacyVersionsForMode(layout.Legacy, opts.Legacy)
	err = cleanupStaleVersions(layout, opts)
	if err != nil {
		return err
	}
	var loader *Loader

	loader, err = NewLoader(opts.Module, opts.GenDir, layout.Legacy)
	if err != nil {
		return err
	}
	activeLoader = loader
	defer func() { activeLoader = nil }()
	var hubMeta HubMeta

	hubMeta, err = loader.HubMeta()
	if err != nil {
		return err
	}
	opts.HubMeta = hubMeta
	var hubSchema *Schema

	hubSchema, err = loader.Schema("hub")
	if err != nil {
		return err
	}

	for _, version := range layout.Legacy {
		legacySchema, err := loader.Schema(version)
		if err != nil {
			return err
		}
		plan := BuildPlan(version, hubSchema, legacySchema)
		err = checkManualRequired(plan, opts)
		if err != nil {
			return err
		}
		err = generateConvert(version, plan, opts)
		if err != nil {
			return err
		}
		err = generateAdapters(version, plan, legacySchema, opts)
		if err != nil {
			return err
		}
		err = generateEventAdapters(version, legacySchema, opts)
		if err != nil {
			return err
		}
		err = cleanupStaleAdapterFiles(version, opts)
		if err != nil {
			return err
		}
	}
	err = generateWire(layout, opts)
	if err != nil {
		return err
	}
	return generateRuntime(layout, opts)
}

func legacyVersionsForMode(legacy []string, mode CompatibilityMode) []string {
	switch mode {
	case CompatibilityDisabled:
		return nil
	case CompatibilityEnabled, CompatibilityAuto:
		return legacy
	default:
		return legacy
	}
}

func cleanupStaleVersions(layout Layout, opts GenerateOptions) error {
	legacySet := map[string]struct{}{}
	for _, v := range layout.Legacy {
		legacySet[v] = struct{}{}
	}
	for _, sub := range []string{"convert", "adapters"} {
		root := filepath.Join(opts.GenDir, "zengo", sub)
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, e := range entries {
			if !e.IsDir() || !legacyDirPattern.MatchString(e.Name()) {
				continue
			}
			_, ok := legacySet[e.Name()]
			if ok {
				continue
			}
			err := os.RemoveAll(filepath.Join(root, e.Name()))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func cleanupStaleAdapterFiles(version string, opts GenerateOptions) error {
	dir := filepath.Join(opts.GenDir, "zengo", "adapters", version)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	meta := opts.HubMeta
	keep := map[string]struct{}{
		adapterFileName(meta.PrimaryService): {},
	}
	if meta.EventService != "" {
		keep[eventAdapterFileName(meta.EventService)] = struct{}{}
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_gen.go") {
			continue
		}
		_, ok := keep[e.Name()]
		if ok {
			continue
		}
		err := os.Remove(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

func checkManualRequired(plan ConversionPlan, opts GenerateOptions) error {
	var pending []ManualRequired
	for _, e := range plan.Errors {
		if fileExists(e.ManualPath) || hasManualConverter(opts.Internal, plan.Version, e.LegacyMessage) {
			continue
		}
		pending = append(pending, e)
	}
	if len(pending) > 0 {
		return formatManualErrors(pending)
	}
	return nil
}

func formatManualErrors(errs []ManualRequired) error {
	var b strings.Builder
	b.WriteString("version conversion requires manual implementation:\n")
	for _, e := range errs {
		fmt.Fprintf(
			&b,
			"\n- [%s] %s\n  reason: %s\n  action: create %s\n",
			e.Version,
			e.LegacyMessage,
			e.Reason,
			e.ManualPath,
		)
	}
	return fmt.Errorf("%s", b.String())
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
