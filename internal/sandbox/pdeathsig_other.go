//go:build !linux && !windows

package sandbox

import "syscall"

func setPdeathsig(*syscall.SysProcAttr) {}
