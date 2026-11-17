//go:build !linux

package sandbox

import "fmt"

func CheckLandlockSupport() (bool, int, error) {
	return false, 0, ErrUnsupported
}

func applyLandlock(workDir string, allowed []PathAccess, forbidden []string) error {
	return fmt.Errorf("%w", ErrUnsupported)
}

func prepareChildEnv(cfg *Config, command string, args []string) ([]string, error) {
	return nil, fmt.Errorf("%w", ErrUnsupported)
}
