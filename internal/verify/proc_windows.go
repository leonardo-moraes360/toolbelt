//go:build windows

package verify

import "os/exec"

// setProcessGroup is a no-op on Windows; WaitDelay still bounds the wait.
func setProcessGroup(*exec.Cmd) {}
