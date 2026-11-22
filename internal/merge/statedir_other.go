//go:build !unix

package merge

import "os"

// secureStateDir tightens dir to owner-only where the platform has modes.
func secureStateDir(dir string) error { return os.Chmod(dir, 0o700) }
