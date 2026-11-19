//go:build unix

package egm

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// lockState takes LOCK_EX|LOCK_NB on <state-dir>/lock. The kernel releases it
// if the process dies (R21). A busy lock is exit 2 with reason locked.
func lockState(dir string) (unlock func(), err error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errorf(ExitError, "locked", "another tb egm command holds %s", f.Name())
		}
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
