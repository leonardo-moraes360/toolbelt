package sandbox

import "sync"

// NetworkDenyAvailable reports whether Config.DenyNetwork can be enforced:
// Linux on amd64/arm64 with usable Landlock (the trampoline that installs the
// filter only runs under Landlock) and the seccomp filter mode. Elsewhere it is
// false.
func NetworkDenyAvailable() bool {
	return landlockAvailable() && networkDenyProbe()
}

var (
	seccompOnce sync.Once
	seccompOK   bool
)

// cachedSeccompAvailable runs the (cheap, non-mutating) kernel probe once.
func cachedSeccompAvailable() bool {
	seccompOnce.Do(func() { seccompOK = seccompFilterAvailable() })
	return seccompOK
}

// networkDenyProbe is a variable so tests can simulate a host without seccomp
// (see OverrideNetworkDenyProbe); in production it is the cached kernel probe.
var networkDenyProbe = cachedSeccompAvailable

// OverrideNetworkDenyProbe replaces the seccomp probe used by
// NetworkDenyAvailable and New and returns a function that restores it. Tests
// ONLY: it exercises the fail-closed path regardless of the CI kernel.
func OverrideNetworkDenyProbe(fn func() bool) (restore func()) {
	prev := networkDenyProbe
	networkDenyProbe = fn
	return func() { networkDenyProbe = prev }
}
