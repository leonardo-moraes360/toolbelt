//go:build windows

package bus

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockOffset: LockFileEx locks are mandatory on Windows, so the byte locked
// lies far past the holder record and a contender can still read the record.
const lockOffset = 1 << 30

func lockFile(path, kind string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	h := windows.Handle(f.Fd())
	ol := &windows.Overlapped{Offset: lockOffset}
	err = windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return nil, errLockHeld
		}
		return nil, err
	}
	if err := f.Truncate(0); err == nil {
		_, _ = f.WriteAt([]byte(holderRecord(kind)), 0)
	}
	return func() {
		_ = windows.UnlockFileEx(h, 0, 1, 0, &windows.Overlapped{Offset: lockOffset})
		_ = f.Close()
	}, nil
}
