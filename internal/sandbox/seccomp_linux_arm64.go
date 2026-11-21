//go:build linux && arm64

package sandbox

import "golang.org/x/sys/unix"

const (
	seccompAuditArch = unix.AUDIT_ARCH_AARCH64
	seccompRejectX32 = false
	x32SyscallBit    = 0
)
