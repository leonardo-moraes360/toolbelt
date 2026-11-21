//go:build !linux || !(amd64 || arm64)

package sandbox

import "fmt"

// The filter is built only for amd64 and arm64 (386 multiplexes socket calls
// through socketcall(2), other arches are untested): DenyNetwork is
// unavailable here.
func seccompFilterAvailable() bool { return false }

func installNetworkFilter() error {
	return fmt.Errorf("%w: seccomp network filter not built for this architecture", ErrUnsupported)
}
