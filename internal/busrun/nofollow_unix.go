//go:build unix

package busrun

import "syscall"

// openNoFollow refuses a final symlink and never blocks on a FIFO.
const openNoFollow = syscall.O_NOFOLLOW | syscall.O_NONBLOCK
