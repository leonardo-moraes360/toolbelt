//go:build linux

package sandbox

import "syscall"

func kernelLandlockPresent() bool {
	const (
		sysCreate   = 444
		flagVersion = 1
	)
	_, _, errno := syscall.Syscall(sysCreate, 0, 0, flagVersion)
	return errno == 0
}
