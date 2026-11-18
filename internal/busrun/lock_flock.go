//go:build linux || darwin || freebsd || netbsd || openbsd

package busrun

import (
	"os"
	"syscall"
)

// acquireLock obtém lock exclusivo (flock) sem esperar: falha se outro
// harness já segura o lock.
func acquireLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == syscall.EINTR {
			continue
		}
		break
	}
	if err != nil {
		_ = f.Close()
		return nil, errLocked
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
