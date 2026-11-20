//go:build unix

package busdiscover

import "syscall"

// openFlags never follow a final symlink and never block on a FIFO.
const openFlags = syscall.O_NOFOLLOW | syscall.O_NONBLOCK
