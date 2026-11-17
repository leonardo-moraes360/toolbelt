//go:build !windows

package router

import (
	"errors"
	"syscall"
)

// PidAlive informa se o processo pid existe.
func PidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
