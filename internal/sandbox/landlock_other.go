//go:build !linux

package sandbox

import "log/slog"

func CheckLandlockSupport() (bool, int, error) {
	return false, 0, ErrUnsupported
}

func applyLandlock(workDir string, allowed []PathAccess, forbidden []string) error {
	slog.Warn("Landlock indisponível fora do Linux")
	return nil
}
