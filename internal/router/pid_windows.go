//go:build windows

package router

import "os"

// On Windows PidAlive only recognizes the current process (no signal 0).
func PidAlive(pid int) bool { return pid == os.Getpid() }
