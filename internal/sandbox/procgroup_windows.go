//go:build windows

package sandbox

import (
	"os/exec"
	"time"
)

func killProcessGroup(cmd *exec.Cmd) {
	cmd.WaitDelay = 2 * time.Second
}

// reapProcessGroup has no equivalent without Job Objects; no-op on Windows.
func reapProcessGroup(*exec.Cmd) {}

// settleGroup is a no-op on Windows (no subreaper).
func settleGroup(*Registry, int) {}

// groupGone: no process groups on Windows; nothing to wait for.
func groupGone(int) bool { return true }
