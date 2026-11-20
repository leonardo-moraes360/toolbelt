//go:build !unix

package busrun

import "errors"

// The desktop backend exists only on Linux and macOS (Validate refuses it
// elsewhere), so this is never reached in practice.
func rootOwned(string) error { return errors.New("ownership check not supported on this platform") }
