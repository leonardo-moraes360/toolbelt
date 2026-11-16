package version

import (
	"encoding/json"
	"runtime"
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
