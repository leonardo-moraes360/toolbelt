// Package version exposes build metadata.
package version

import (
	"fmt"
	"runtime"

	"github.com/leonardo-moraes360/toolbelt/internal/embed/native"
)

// Overwritten by goreleaser via ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
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
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Embed:     native.Engine(),
	}
}

func (i Info) String() string {
	return fmt.Sprintf("toolbelt %s\n  commit: %s\n  built:  %s\n  go:     %s\n  os:     %s/%s\n  embed:  %s",
		i.Version, i.Commit, i.BuildTime, i.GoVersion, i.OS, i.Arch, i.Embed)
}

func (i Info) Short() string { //nolint:gocritic // small struct, value receiver is fine
	return i.Version
}
