//go:build egmfake && unix

package bench

import "syscall"

// newSessionAttr makes the child a session leader (setsid) before exec.
func newSessionAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }
