//go:build linux && amd64

package sandbox

import "golang.org/x/sys/unix"

const (
	seccompAuditArch = unix.AUDIT_ARCH_X86_64
	// x32 syscalls report AUDIT_ARCH_X86_64 with this bit set in nr; they are
	// a second syscall table, so they are rejected outright.
	seccompRejectX32 = true
	x32SyscallBit    = 0x40000000
)
