// Package version exposes build metadata.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/embed/native"
)

// Overwritten by goreleaser via ldflags. A build without them (go build,
// go install) falls back to the build info the Go toolchain embeds, so a
// hook comparing version + commit does not see "dev"/"unknown" for a
// binary built from a tagged or committed tree.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	// Modified: the tree had uncommitted changes at build time (vcs.modified).
	// Only known from build info; ldflags builds come from a clean release tree.
	Modified bool
)

func init() {
	if bi, ok := debug.ReadBuildInfo(); ok {
		applyBuildInfo(bi)
	}
}

func applyBuildInfo(bi *debug.BuildInfo) {
	if Version == "dev" {
		// "(devel)" is what go build gives a tree outside a tagged module version.
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			Version = strings.TrimPrefix(v, "v")
		}
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if Commit == "unknown" && s.Value != "" {
				Commit = s.Value
			}
		case "vcs.time":
			if BuildTime == "unknown" && s.Value != "" {
				BuildTime = s.Value
			}
		case "vcs.modified":
			Modified = s.Value == "true"
		}
	}
}

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	Modified  bool   `json:"modified"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	// Embed is the native embedding engine compiled in (ADR-018).
	Embed string `json:"embed"`
}

func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		Modified:  Modified,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Embed:     native.Engine(),
	}
}

func (i Info) String() string {
	commit := i.Commit
	if i.Modified {
		commit += " (modified)"
	}
	return fmt.Sprintf("toolbelt %s\n  commit: %s\n  built:  %s\n  go:     %s\n  os:     %s/%s\n  embed:  %s",
		i.Version, commit, i.BuildTime, i.GoVersion, i.OS, i.Arch, i.Embed)
}

func (i Info) Short() string { //nolint:gocritic // small struct, value receiver is fine
	return i.Version
}
