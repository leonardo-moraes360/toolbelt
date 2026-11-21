//go:build linux && (amd64 || arm64)

package sandbox

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Offsets in struct seccomp_data (include/uapi/linux/seccomp.h).
const (
	seccompDataNr   = 0
	seccompDataArch = 4
)

const seccompRetEPERM = unix.SECCOMP_RET_ERRNO | (uint32(unix.EPERM) & unix.SECCOMP_RET_DATA)

// networkDeniedSyscalls fail with EPERM. io_uring is included because
// IORING_OP_SOCKET creates sockets without going through socket(2); the Go
// runtime and the tools run under DenyNetwork do not use io_uring.
var networkDeniedSyscalls = []uint32{
	unix.SYS_SOCKET,
	unix.SYS_SOCKETPAIR,
	unix.SYS_IO_URING_SETUP,
	unix.SYS_IO_URING_ENTER,
	unix.SYS_IO_URING_REGISTER,
}

func bpfStmt(code uint16, k uint32) unix.SockFilter {
	return unix.SockFilter{Code: code, K: k}
}

func bpfJump(code uint16, k uint32, jt, jf uint8) unix.SockFilter {
	return unix.SockFilter{Code: code, Jt: jt, Jf: jf, K: k}
}

// networkFilter builds the classic-BPF program. A foreign arch (e.g. the
// i386 ABI through int 0x80 on x86-64, or AArch32 compat on arm64) has another
// syscall table, where the numbers below mean something else, so it kills the
// process instead of being matched against them.
func networkFilter() []unix.SockFilter {
	const (
		ld   = unix.BPF_LD | unix.BPF_W | unix.BPF_ABS
		jeq  = unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K
		jset = unix.BPF_JMP | unix.BPF_JSET | unix.BPF_K
		ret  = unix.BPF_RET | unix.BPF_K
	)
	prog := []unix.SockFilter{
		bpfStmt(ld, seccompDataArch),
		bpfJump(jeq, seccompAuditArch, 1, 0),
		bpfStmt(ret, unix.SECCOMP_RET_KILL_PROCESS),
		bpfStmt(ld, seccompDataNr),
	}
	if seccompRejectX32 {
		prog = append(prog,
			bpfJump(jset, x32SyscallBit, 0, 1),
			bpfStmt(ret, unix.SECCOMP_RET_KILL_PROCESS),
		)
	}
	for _, nr := range networkDeniedSyscalls {
		prog = append(prog,
			bpfJump(jeq, nr, 0, 1),
			bpfStmt(ret, seccompRetEPERM),
		)
	}
	return append(prog, bpfStmt(ret, unix.SECCOMP_RET_ALLOW))
}

// installNetworkFilter applies the filter to the calling thread, which must be
// locked (runtime.LockOSThread) and must exec right after: the filter is
// inherited across execve, and execve ends the other threads. It requires
// no_new_privs, which it sets (idempotent; Landlock already set it).
func installNetworkFilter() error {
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("prctl(PR_SET_NO_NEW_PRIVS): %w", err)
	}
	filter := networkFilter()
	prog := unix.SockFprog{Len: uint16(len(filter)), Filter: &filter[0]} //nolint:gosec // a few dozen instructions
	_, _, errno := unix.Syscall(unix.SYS_SECCOMP, unix.SECCOMP_SET_MODE_FILTER, 0, uintptr(unsafe.Pointer(&prog)))
	runtime.KeepAlive(filter)
	if errno != 0 {
		return fmt.Errorf("seccomp(SECCOMP_SET_MODE_FILTER): %w", errno)
	}
	return nil
}

// seccompFilterAvailable asks the kernel whether every action the filter
// returns is supported. SECCOMP_GET_ACTION_AVAIL changes nothing, needs
// CONFIG_SECCOMP_FILTER and fails where seccomp(2) itself is blocked (e.g. by a
// container profile), which is what matters here.
func seccompFilterAvailable() bool {
	for _, action := range []uint32{unix.SECCOMP_RET_KILL_PROCESS, unix.SECCOMP_RET_ERRNO, unix.SECCOMP_RET_ALLOW} {
		a := action
		_, _, errno := unix.Syscall(unix.SYS_SECCOMP, unix.SECCOMP_GET_ACTION_AVAIL, 0, uintptr(unsafe.Pointer(&a)))
		if errno != 0 {
			return false
		}
	}
	return true
}
