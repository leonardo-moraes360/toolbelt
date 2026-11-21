//go:build linux && (amd64 || arm64)

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const netProbeMode = "net-probe"

// The probe runs as the command inside the sandbox (the test binary execs
// itself through the trampoline) and prints name=OK or name=<errno>.
func init() {
	if os.Getenv(helperEnv) != netProbeMode {
		return
	}
	report := func(name string, err error) {
		if err == nil {
			fmt.Printf("%s=OK\n", name)
			return
		}
		var errno syscall.Errno
		if errors.As(err, &errno) {
			fmt.Printf("%s=%d\n", name, int(errno))
			return
		}
		fmt.Printf("%s=%v\n", name, err)
	}
	for _, c := range []struct {
		name   string
		domain int
	}{{"inet", unix.AF_INET}, {"inet6", unix.AF_INET6}, {"unix", unix.AF_UNIX}} {
		fd, err := unix.Socket(c.domain, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
		if err == nil {
			_ = unix.Close(fd)
		}
		report(c.name, err)
	}
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err == nil {
		_ = unix.Close(fds[0])
		_ = unix.Close(fds[1])
	}
	report("socketpair", err)
	var params [120]byte // struct io_uring_params
	r, _, errno := unix.Syscall(unix.SYS_IO_URING_SETUP, 1, uintptr(unsafe.Pointer(&params)), 0)
	if errno == 0 {
		_ = unix.Close(int(r))
		report("io_uring", nil)
	} else {
		report("io_uring", errno)
	}
	report("signal", unix.Kill(os.Getppid(), 0))
	os.Exit(0)
}

func runNetProbe(t *testing.T, deny bool) map[string]string {
	t.Helper()
	if !kernelLandlockPresent() {
		t.Skip("kernel without Landlock")
	}
	if ok, _, _ := CheckLandlockSupport(); !ok {
		t.Skip("Landlock unavailable")
	}
	if deny && !NetworkDenyAvailable() {
		t.Skip("seccomp filter mode unavailable (SECCOMP_GET_ACTION_AVAIL failed)")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	sb, err := New(Config{
		WorkDir:      t.TempDir(),
		Timeout:      20 * time.Second,
		Env:          append(os.Environ(), helperEnv+"="+netProbeMode),
		AllowedPaths: []PathAccess{{Path: exe, ReadOnly: true}},
		DenyNetwork:  deny,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := sb.Run(context.Background(), exe, "-test.run=^$")
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || !res.Sandboxed {
		t.Fatalf("probe: exit=%d sandboxed=%v output=%q", res.ExitCode, res.Sandboxed, res.Output)
	}
	got := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(res.Output), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			got[k] = v
		}
	}
	t.Logf("probe (deny=%v): %v", deny, got)
	return got
}

func TestDenyNetworkBlocksSockets(t *testing.T) {
	got := runNetProbe(t, true)
	eperm := fmt.Sprint(int(unix.EPERM))
	for _, name := range []string{"inet", "inet6", "unix", "socketpair", "io_uring"} {
		if got[name] != eperm {
			t.Errorf("%s under DenyNetwork = %q, want EPERM (%s); all=%v", name, got[name], eperm, got)
		}
	}
	// The parent (this test process) is outside the Landlock domain: with
	// signal scoping (ABI >= 6) even kill(ppid, 0) is refused.
	if _, abi, _ := CheckLandlockSupport(); abi >= landlockABIScoped && got["signal"] != eperm {
		t.Errorf("signal to the parent under DenyNetwork (Landlock ABI %d) = %q, want EPERM", abi, got["signal"])
	}
}

func TestWithoutDenyNetworkSocketsWork(t *testing.T) {
	got := runNetProbe(t, false)
	for _, name := range []string{"unix", "socketpair"} {
		if got[name] != "OK" {
			t.Errorf("%s without DenyNetwork = %q, want OK; all=%v", name, got[name], got)
		}
	}
}

// The x32 bit must not slip past the syscall-number checks. Verified on the
// program itself: running an x32 syscall would kill the test process.
func TestNetworkFilterShape(t *testing.T) {
	prog := networkFilter()
	if prog[0].K != seccompDataArch || prog[1].K != seccompAuditArch || prog[2].K != unix.SECCOMP_RET_KILL_PROCESS {
		t.Fatalf("filter does not start with the arch check: %+v", prog[:3])
	}
	if last := prog[len(prog)-1]; last.K != unix.SECCOMP_RET_ALLOW {
		t.Fatalf("filter does not end with ALLOW: %+v", last)
	}
	if seccompRejectX32 && (prog[4].Code != unix.BPF_JMP|unix.BPF_JSET|unix.BPF_K || prog[4].K != x32SyscallBit || prog[5].K != unix.SECCOMP_RET_KILL_PROCESS) {
		t.Fatalf("x32 check missing: %+v", prog[4:6])
	}
}
