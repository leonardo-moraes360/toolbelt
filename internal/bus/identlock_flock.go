//go:build linux || darwin || freebsd || netbsd || openbsd

package bus

import (
	"os"
	"syscall"
)

// fileExistenceIsLock: false where the lock is a flock on an existing file.
const fileExistenceIsLock = false

func lockFile(path, kind string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err != syscall.EINTR {
			break
		}
	}
	if err != nil {
		_ = f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, errLockHeld
		}
		return nil, err
	}
	// Holder info is advisory (for the contender's message); the flock is the
	// lock. A probe leaves the real holder's record alone.
	if kind == probeKind {
		return func() {
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			_ = f.Close()
		}, nil
	}
	if err := f.Truncate(0); err == nil {
		_, _ = f.WriteAt([]byte(holderRecord(kind)), 0)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
