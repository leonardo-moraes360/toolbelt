//go:build !windows

package verify

import (
	"os/exec"
	"syscall"
)

// setProcessGroup runs cmd in its own process group and kills the whole
// group on cancel, so a timeout also reaches the children sh spawned.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
