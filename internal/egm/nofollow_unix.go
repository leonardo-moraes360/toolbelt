//go:build unix

package egm

import "syscall"

// oNoFollow refuses to open the last path component through a symlink.
const oNoFollow = syscall.O_NOFOLLOW
