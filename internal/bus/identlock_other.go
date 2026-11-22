//go:build !(linux || darwin || freebsd || netbsd || openbsd || windows)

package bus

import (
	"errors"
	"os"
)

// Without flock or LockFileEx the lock is an O_EXCL file removed on release.
// It cannot tell a live holder from a dead one, so a file left by a crashed
// holder must be removed by hand; a lock is never taken over.
func lockFile(path, kind string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil, errLockHeld
	}
	if err != nil {
		return nil, err
	}
	_, _ = f.WriteString(holderRecord(kind))
	_ = f.Close()
	return func() { _ = os.Remove(path) }, nil
}
