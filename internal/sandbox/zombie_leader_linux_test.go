//go:build linux

package sandbox

import (
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// helperEnv selects a helper mode when the test binary re-executes itself.
const helperEnv = "TB_SANDBOX_TEST_HELPER"

func init() {
	if os.Getenv(helperEnv) == "zombie-leader" {
		// Keep the main goroutine on the main thread (the thread-group leader),
		// so the SYS_exit in TestMain ends the leader and nothing else.
		runtime.LockOSThread()
	}
}

func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) == "zombie-leader" {
		// exit(2), not exit_group(2): only the leader thread ends. The Go
		// runtime's other threads (sysmon, GC workers) keep the process alive,
		// so /proc/<pid>/stat shows Z while the thread group still runs.
		_, _, _ = syscall.RawSyscall(syscall.SYS_EXIT, 0, 0, 0)
	}
	os.Exit(m.Run())
}

// startZombieLeader starts this test binary as an untracked, detached child
// whose leader thread exits while its other threads stay alive, and waits until
// /proc shows that state.
func startZombieLeader(t *testing.T, extraEnv ...string) int {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^$") //nolint:gosec // the test binary itself
	cmd.Env = append(append(os.Environ(), helperEnv+"=zombie-leader"), extraEnv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		// Through os.Process (pidfd): never hits a reused pid once Sweep reaped
		// the helper; Wait then just reports ECHILD.
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	deadline := time.Now().Add(10 * time.Second)
	for {
		for _, c := range listChildren() {
			if c.pid == pid && leaderIsZombie(pid) && liveThread(pid) != 0 {
				return pid
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("helper %d never reached zombie-leader-with-live-threads", pid)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func leaderIsZombie(pid int) bool {
	st := readStat(pid)
	return len(st) > 0 && st[0] == "Z"
}

// An escaped process whose leader thread called exit(2) while other threads
// keep running shows state Z. It used to be taken for a zombie: never
// classified, never signalled, and wait4 cannot reap a leader with live
// threads, so every sweep spun for 2 s with the mutex held and the live escape
// was never killed or reported.
func TestRegistrySweepKillsZombieLeaderWithLiveThreads(t *testing.T) {
	pid := startZombieLeader(t, "TB_TEST_MARK=zl")
	reg := NewRegistry("TB_TEST_MARK")
	began := time.Now()
	esc := reg.Sweep()
	took := time.Since(began)
	if len(esc) != 1 || esc[0].PID != pid {
		t.Fatalf("escapes=%+v pid=%d", esc, pid)
	}
	if got := esc[0].Vars["TB_TEST_MARK"]; got != "zl" {
		t.Fatalf("Vars=%v: environment not read through a live thread", esc[0].Vars)
	}
	if took > time.Second {
		t.Fatalf("Sweep took %v: spun on an unreapable leader", took)
	}
	if err := syscall.Kill(pid, 0); err == nil {
		t.Fatal("escaped process still alive after Sweep")
	}
	if again := reg.Sweep(); len(again) != 0 {
		t.Fatalf("second sweep: %+v", again)
	}
}
