//go:build !unix

package egm

// lockState needs flock (R21); tb egm commands that change state are not
// supported on this platform.
func lockState(string) (func(), error) {
	return nil, errorf(ExitError, "unsupported_platform", "tb egm needs flock (unix)")
}
