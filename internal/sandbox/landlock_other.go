//go:build !linux

package sandbox

import "fmt"

func CheckLandlockSupport() (bool, int, error) {
	return false, 0, ErrUnsupported
}

type childPayload struct{}

func applyLandlock(p *childPayload) error {
	return fmt.Errorf("%w", ErrUnsupported)
}

func prepareChildEnv(cfg *Config, command string, args []string) ([]string, error) {
	return nil, fmt.Errorf("%w", ErrUnsupported)
}
