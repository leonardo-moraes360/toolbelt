//go:build !(linux || darwin || freebsd || netbsd || openbsd)

package bus

import (
	"errors"
	"os"
)

// Without flock the lock is an O_EXCL file; a file whose pid no longer
// exists is stale and is taken over.
func lockFile(path, kind string) (func(), error) {
	lockPath := path
	for range 2 {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = f.WriteString(holderRecord(kind))
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		pid, _ := readHolder(lockPath)
		if pid > 0 && ProcessAlive(pid) {
			return nil, errLockHeld
		}
		_ = os.Remove(lockPath)
	}
	return nil, errLockHeld
}

// ProcessAlive reports whether pid exists.
func ProcessAlive(pid int) bool {
	_, err := os.FindProcess(pid)
	return err == nil
}
