package version

import (
	"encoding/json"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

func TestDefaultVersion(t *testing.T) {
	if Version != "dev" {
		t.Errorf("Version default = %q, want %q", Version, "dev")
	}
	if Commit == "" {
		t.Error("Commit default should not be empty")
	}
	if BuildTime == "" {
		t.Error("BuildTime default should not be empty")
	}
}

func TestGet(t *testing.T) {
	info := Get()

	if info.Version == "" {
		t.Error("Version should not be empty")
	}
	if info.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, want %q", info.GoVersion, runtime.Version())
	}
	if info.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", info.OS, runtime.GOOS)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", info.Arch, runtime.GOARCH)
	}
}

func TestInfo_String(t *testing.T) {
	info := Get()
	s := info.String()

	if !strings.Contains(s, "toolbelt") {
		t.Error("String() should contain 'toolbelt'")
	}
	if !strings.Contains(s, info.Version) {
		t.Errorf("String() should contain version %q", info.Version)
	}
}

func TestInfo_Short(t *testing.T) {
	info := Get()
	if info.Short() != info.Version {
		t.Errorf("Short() = %q, want %q", info.Short(), info.Version)
	}
}

func TestInfo_JSON(t *testing.T) {
	info := Get()
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded Info
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Version != info.Version {
		t.Errorf("Decoded version = %q, want %q", decoded.Version, info.Version)
	}
}

func TestApplyBuildInfo(t *testing.T) {
	saveV, saveC, saveB, saveM := Version, Commit, BuildTime, Modified
	t.Cleanup(func() { Version, Commit, BuildTime, Modified = saveV, saveC, saveB, saveM })
	bi := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc123"},
			{Key: "vcs.time", Value: "2026-11-22T10:00:00Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	Version, Commit, BuildTime, Modified = "dev", "unknown", "unknown", false
	applyBuildInfo(bi)
	if Version != "1.2.3" || Commit != "abc123" || BuildTime != "2026-11-22T10:00:00Z" || !Modified {
		t.Fatalf("fallback: %q %q %q %v", Version, Commit, BuildTime, Modified)
	}
	if got := Get().String(); !strings.Contains(got, "abc123 (modified)") {
		t.Errorf("String() = %q", got)
	}

	Version, Commit, BuildTime, Modified = "9.9.9", "ldflag", "ldtime", false
	applyBuildInfo(bi)
	if Version != "9.9.9" || Commit != "ldflag" || BuildTime != "ldtime" {
		t.Fatalf("ldflags must win: %q %q %q", Version, Commit, BuildTime)
	}

	Version = "dev"
	applyBuildInfo(&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}})
	if Version != "dev" {
		t.Fatalf("(devel) must not replace dev: %q", Version)
	}
}

func TestInfoJSONHasModified(t *testing.T) {
	b, err := json.Marshal(Get())
	if err != nil || !strings.Contains(string(b), `"modified":`) {
		t.Fatalf("%s %v", b, err)
	}
}
