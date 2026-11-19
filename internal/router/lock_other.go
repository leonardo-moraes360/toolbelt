//go:build !unix

package router

import (
	"errors"
	"fmt"
	"os"
)

var errRouterBusy = errors.New("another tb router run is active in this repository")

// acquireLock (no flock): an O_EXCL lockfile removed on exit. If the process
// dies without cleaning up, the file remains and the user must remove it.
func acquireLock(path string) (release func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // .toolbelt/router/lock
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("%w (if there is no other router, remove %s)", errRouterBusy, path)
		}
		return nil, err
	}
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Close()
	return func() { _ = os.Remove(path) }, nil
}
