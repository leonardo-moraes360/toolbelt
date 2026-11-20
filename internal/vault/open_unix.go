//go:build unix

package vault

import (
	"errors"
	"syscall"
)

// openFlags never follow a final symlink and never block on a FIFO (R3).
const openFlags = syscall.O_NOFOLLOW | syscall.O_NONBLOCK

// isLoop is the error of opening a symlink with O_NOFOLLOW.
func isLoop(err error) bool { return errors.Is(err, syscall.ELOOP) }
