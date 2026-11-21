//go:build unix

package embed

import (
	"os"
	"syscall"
)

func ownedByUser(fi os.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}

// safeDir: a parent directory of the model must belong to the user or
// root and must not be writable by other users, unless it is sticky (like
// /tmp), where others cannot rename or remove entries they do not own.
// Group write is allowed only for the user's own primary group (the
// per-user group of a umask 002 setup).
func safeDir(fi os.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || (int(st.Uid) != os.Getuid() && st.Uid != 0) {
		return false
	}
	if fi.Mode()&os.ModeSticky != 0 {
		return true
	}
	perm := fi.Mode().Perm()
	if perm&0o002 != 0 {
		return false
	}
	return perm&0o020 == 0 || int(st.Gid) == os.Getgid()
}

// openNoFollow opens the model refusing a symlink in the last component.
func openNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0) //nolint:gosec // checked by CheckModel
}
