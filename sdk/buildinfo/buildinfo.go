package buildinfo

import (
	"encoding/json"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
)

// Version is optionally set at build time with -ldflags.
var Version string

// Branch is optionally set at build time with -ldflags.
var Branch string

// Info is the stable JSON contract for build metadata.
type Info struct {
	// Service is the logical service name passed in by the caller.
	Service string `json:"service"`
	// Module is the main Go module path recorded in build metadata.
	Module string `json:"module"`
	// Version is the release version from linker flags or module metadata.
	Version string `json:"version"`
	// Branch is the source control branch injected at build time.
	Branch string `json:"branch"`
	// Commit is the VCS revision reported by the Go toolchain.
	Commit string `json:"commit"`
	// Time is the VCS commit time reported by the Go toolchain.
	Time string `json:"time"`
	// Dirty reports whether the working tree was modified at build time.
	Dirty bool `json:"dirty"`
	// GoVersion is the Go toolchain version that built the binary.
	GoVersion string `json:"go_version"`
}

// Current returns build metadata for the current process.
func Current(service string) Info {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return currentFrom(service, nil)
	}
	return currentFrom(service, bi)
}

// Handler serves build metadata as JSON.
func Handler(service string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = encode(w, Current(service))
	})
}

// Print writes the same JSON payload used by Handler.
func Print(w io.Writer, service string) error {
	return encode(w, Current(service))
}

func encode(w io.Writer, info Info) error {
	enc := json.NewEncoder(w)
	return enc.Encode(info)
}

func currentFrom(service string, bi *debug.BuildInfo) Info {
	info := Info{Service: service}
	if bi == nil {
		info.Version = Version
		info.Branch = Branch
		return info
	}

	info.Module = bi.Main.Path
	info.GoVersion = bi.GoVersion
	if Version != "" {
		info.Version = Version
	} else if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		info.Version = bi.Main.Version
	}
	info.Branch = Branch

	settings := buildSettings(bi.Settings)
	info.Commit = settings["vcs.revision"]
	info.Time = settings["vcs.time"]
	dirty, ok := settings["vcs.modified"]
	if ok {
		info.Dirty, _ = strconv.ParseBool(dirty)
	}

	return info
}

func buildSettings(settings []debug.BuildSetting) map[string]string {
	out := make(map[string]string, len(settings))
	for _, setting := range settings {
		out[setting.Key] = setting.Value
	}
	return out
}
