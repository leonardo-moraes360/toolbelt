//go:build !amd64

package embed

// CheckCPU accepts every CPU: arm64 kernels stay at the armv8.0-a floor.
func CheckCPU() error { return nil }
