//go:build unix

package router

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

var errRouterBusy = errors.New("outro tb router run ativo neste repositório")

// acquireLock takes an exclusive, non-blocking flock on path. The lock lives as
// long as the descriptor is open and the kernel releases it if the process dies,
// so a reused pid in state.json never blocks a new router.
func acquireLock(path string) (release func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // .toolbelt/router/lock
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil { //nolint:gosec // small fd
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errRouterBusy
		}
		return nil, fmt.Errorf("lock do router: %w", err)
	}
	_ = f.Truncate(0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid()) // informational only
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:gosec // small fd
		_ = f.Close()
	}, nil
}
