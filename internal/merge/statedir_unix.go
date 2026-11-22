//go:build unix

package merge

import (
	"fmt"
	"os"
	"syscall"
)

// secureStateDir makes sure dir is private to the current user: a directory
// owned by someone else is refused, a looser mode is tightened to 0700.
func secureStateDir(dir string) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("merge state directory %s is not a directory", dir)
	}
	if sys, ok := fi.Sys().(*syscall.Stat_t); ok && int(sys.Uid) != os.Getuid() {
		return fmt.Errorf("merge state directory %s is owned by another user", dir)
	}
	if fi.Mode().Perm() != 0o700 {
		return os.Chmod(dir, 0o700)
	}
	return nil
}
