//go:build windows

package sandbox

import (
	"os/exec"
	"time"
)

func killProcessGroup(cmd *exec.Cmd) {
	cmd.WaitDelay = 2 * time.Second
}

// reapProcessGroup não tem equivalente sem Job Objects; no-op no Windows.
func reapProcessGroup(*exec.Cmd) {}

// settleGroup é no-op no Windows (sem subreaper).
func settleGroup(*Registry, int) {}

// groupGone: sem grupos de processos no Windows; nada a aguardar.
func groupGone(int) bool { return true }
