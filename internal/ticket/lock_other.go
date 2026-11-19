//go:build !(linux || darwin || freebsd || netbsd || openbsd)

package ticket

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	lockStaleAfter = 30 * time.Second
	lockTimeout    = 10 * time.Second
)

// acquireLock uses an O_EXCL lockfile with stale-lock detection (fallback
// for systems without flock).
func acquireLock(path string) (func(), error) {
	lockPath := path + ".excl"
	deadline := time.Now().Add(lockTimeout)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if st, serr := os.Stat(lockPath); serr == nil && time.Since(st.ModTime()) > lockStaleAfter {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout aguardando lock %s", lockPath)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
