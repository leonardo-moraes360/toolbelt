//go:build unix

package router

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// errRouterBusy é devolvido quando outro `tb router run` detém o lock.
var errRouterBusy = errors.New("outro tb router run ativo neste repositório")

// acquireLock toma flock exclusivo e não bloqueante em path. O lock vive
// enquanto o descritor estiver aberto e o kernel o libera se o processo morrer,
// então pid reaproveitado em state.json nunca bloqueia um novo router.
func acquireLock(path string) (release func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // .toolbelt/router/lock
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil { //nolint:gosec // fd pequeno
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errRouterBusy
		}
		return nil, fmt.Errorf("lock do router: %w", err)
	}
	_ = f.Truncate(0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid()) // só info
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:gosec // fd pequeno
		_ = f.Close()
	}, nil
}
