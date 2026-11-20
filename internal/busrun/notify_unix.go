//go:build unix

package busrun

import (
	"errors"
	"os"
	"syscall"
)

// rootUID is who must own a desktop notifier (tests override it).
var rootUID uint32

func rootOwned(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("cannot read the owner")
	}
	if sys.Uid != rootUID {
		return errors.New("not owned by root")
	}
	if st.Mode().Perm()&0o022 != 0 {
		return errors.New("group- or world-writable")
	}
	return nil
}
