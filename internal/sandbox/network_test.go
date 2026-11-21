package sandbox

import (
	"errors"
	"testing"
)

// Without a usable seccomp filter, DenyNetwork must refuse to build a sandbox
// unless the caller opted in to running unsandboxed.
func TestNewDenyNetworkUnavailableFailsClosed(t *testing.T) {
	restore := OverrideNetworkDenyProbe(func() bool { return false })
	defer restore()
	if NetworkDenyAvailable() {
		t.Fatal("NetworkDenyAvailable() = true with the probe forced off")
	}
	_, err := New(Config{WorkDir: t.TempDir(), DenyNetwork: true})
	if err == nil {
		t.Fatal("New(DenyNetwork) succeeded without network denial")
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("error %v does not wrap ErrUnsupported", err)
	}
	sb, err := New(Config{WorkDir: t.TempDir(), DenyNetwork: true, AllowUnsandboxed: true})
	if err != nil {
		t.Fatalf("New(DenyNetwork, AllowUnsandboxed): %v", err)
	}
	if sb.netDenyOK {
		t.Fatal("netDenyOK set although network denial is unavailable")
	}
}

func TestNewDenyNetworkUnavailableWithoutLandlock(t *testing.T) {
	restoreLL := OverrideLandlockProbe(func() (bool, int, error) { return false, 0, ErrUnsupported })
	defer restoreLL()
	restore := OverrideNetworkDenyProbe(func() bool { return true })
	defer restore()
	if NetworkDenyAvailable() {
		t.Fatal("NetworkDenyAvailable() = true without Landlock")
	}
}
