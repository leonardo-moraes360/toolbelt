//go:build !linux

package sandbox

func kernelLandlockPresent() bool {
	return false
}
