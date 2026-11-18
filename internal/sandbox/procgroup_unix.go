//go:build !windows

package sandbox

import (
	"os/exec"
	"syscall"
	"time"
)

// killProcessGroup makes the child lead its own process group and, when the
// context is canceled (timeout, watchdog, SIGINT), kills the whole group, so
// grandchildren holding the output pipe cannot hang Run.
func killProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	setPdeathsig(cmd.SysProcAttr)
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second
}

// reapProcessGroup sends SIGKILL to the child's group after Wait. ESRCH (group
// already empty) is the normal case and ignored. The pgid is the leader's pid
// (Setpgid); the pid-reuse window between Wait and this kill is negligible
// because the group only disappears once all members die.
func reapProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

// groupGone reports whether no process (not even a zombie) remains in pgid.
func groupGone(pgid int) bool {
	return syscall.Kill(-pgid, 0) == syscall.ESRCH
}

// settleGroup waits (up to ~1s) for the agent group to drain, reaping zombies
// reparented to the router (subreaper). Descendants that escaped the group stay
// in the escape registry and are delivered by Registry.Sweep. Once drained, the
// group entry is removed from the registry (see pruneGroupsLocked for a group
// that did not drain in time).
func settleGroup(reg *Registry, pgid int) {
	reg.finishGroup(pgid)
	deadline := time.Now().Add(time.Second)
	for {
		reg.mu.Lock()
		reg.sweepLocked()
		reg.pruneGroupsLocked() // group empty after the sweep: final reap, forget the entry
		reg.mu.Unlock()
		if groupGone(pgid) || time.Now().After(deadline) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
}
