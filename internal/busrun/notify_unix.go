//go:build unix

package busrun

import (
	"errors"
	"os"
	"syscall"
)

// rootUID is who, besides uid 0, may own a desktop notifier and its
// ancestors (tests set it to their own uid).
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
	if sys.Uid != 0 && sys.Uid != rootUID {
		return errors.New("not owned by root")
	}
	// A sticky root-owned directory (/tmp) lets nobody rename others' entries.
	if st.Mode().Perm()&0o022 != 0 && (st.Mode()&os.ModeSticky == 0 || sys.Uid != 0) {
		return errors.New("group- or world-writable")
	}
	return nil
}
