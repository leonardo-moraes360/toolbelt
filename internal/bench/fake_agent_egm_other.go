//go:build egmfake && !unix

package bench

import "syscall"

// newSessionAttr: no sessions here; the escape modes only matter on Linux.
func newSessionAttr() *syscall.SysProcAttr { return nil }
