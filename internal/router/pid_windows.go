//go:build windows

package router

import "os"

// PidAlive em Windows só reconhece o próprio processo (sem sinal 0).
func PidAlive(pid int) bool { return pid == os.Getpid() }
