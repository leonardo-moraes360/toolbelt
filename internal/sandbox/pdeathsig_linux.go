//go:build linux

package sandbox

import "syscall"

// setPdeathsig asks the kernel to kill the child (SIGKILL) if the parent dies.
// Caveat: Pdeathsig is tied to the *thread* that forked, so finishRun locks the
// goroutine to its thread (runtime.LockOSThread) until the child exits. It
// covers abrupt router death (kill -9); the rest of the group is bounded by the
// dead child itself + reapProcessGroup on normal paths.
func setPdeathsig(a *syscall.SysProcAttr) { a.Pdeathsig = syscall.SIGKILL }
