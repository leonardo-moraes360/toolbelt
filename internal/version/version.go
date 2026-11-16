// Package version fornece informações de versão do toolbelt.
package version

import (
	"fmt"
	"runtime"
)

var (
	// Version é a versão semântica do toolbelt.
	// Será sobrescrita pelo goreleaser via ldflags.
	Version = "dev"

	// Commit é o hash do commit git.
	// Será sobrescrito pelo goreleaser via ldflags.
	Commit = "unknown"

	// BuildTime é o timestamp da build.
	// Será sobrescrito pelo goreleaser via ldflags.
	BuildTime = "unknown"
)

// Info contém todas as informações de versão.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// Get retorna as informações de versão.
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// String retorna uma representação textual da versão.
func (i Info) String() string {
	return fmt.Sprintf("toolbelt %s\n  commit: %s\n  built:  %s\n  go:     %s\n  os:     %s/%s",
		i.Version, i.Commit, i.BuildTime, i.GoVersion, i.OS, i.Arch)
}

// Short retorna apenas a versão.
func (i Info) Short() string { //nolint:gocritic // small struct, value receiver is fine
	return i.Version
}
