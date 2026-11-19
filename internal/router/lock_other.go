//go:build !unix

package router

import (
	"errors"
	"fmt"
	"os"
)

var errRouterBusy = errors.New("outro tb router run ativo neste repositório")

// acquireLock (no flock): an O_EXCL lockfile removed on exit. If the process
// dies without cleaning up, the file remains and the user must remove it.
func acquireLock(path string) (release func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // .toolbelt/router/lock
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("%w (se não houver outro router, remova %s)", errRouterBusy, path)
		}
		return nil, err
	}
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Close()
	return func() { _ = os.Remove(path) }, nil
}
