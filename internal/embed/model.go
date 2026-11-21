// Package embed runs the native embedding engine for tb graph (ADR-018,
// docs/specs/vector-index.md). V0 only checks a model and smoke-tests it;
// the sandboxed child protocol arrives in V1.
package embed

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrModelPath and ErrModelMismatch are the model refusals (spec,
// Configuration).
var (
	ErrModelPath     = errors.New("model_path")
	ErrModelMismatch = errors.New("model_mismatch")
)

// CheckModel verifies the model file before any native code sees it: an
// absolute path reached through no symlink, whose parent directories only
// the user or root can change (unix), a regular file owned by the current
// user (where ownership exists), whose sha256 is want. It returns the
// identity of the file it hashed, for SameModel after the native load.
func CheckModel(path, want string) (os.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("%w: %q must be a clean absolute path", ErrModelPath, path)
	}
	if len(want) != 64 || strings.ToLower(want) != want {
		return nil, fmt.Errorf("%w: the expected sha256 must be 64 lowercase hex characters", ErrModelMismatch)
	}
	if err := noSymlinks(path); err != nil {
		return nil, err
	}
	f, err := openNoFollow(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrModelPath, err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrModelPath, path)
	}
	if !ownedByUser(fi) {
		return nil, fmt.Errorf("%w: %s is not owned by the current user", ErrModelPath, path)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrModelPath, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return nil, fmt.Errorf("%w: %s has sha256 %s", ErrModelMismatch, path, got)
	}
	return fi, nil
}

// SameModel reports whether path still names the file CheckModel hashed:
// the native engine opens the path again, so a swap in between is refused.
func SameModel(path string, hashed os.FileInfo) error {
	fi, err := os.Lstat(path)
	if err != nil || !os.SameFile(fi, hashed) || fi.Size() != hashed.Size() || !fi.ModTime().Equal(hashed.ModTime()) {
		return fmt.Errorf("%w: %s changed while it was loaded", ErrModelMismatch, path)
	}
	return nil
}

// noSymlinks walks every component of an absolute path with Lstat.
func noSymlinks(path string) error {
	vol := filepath.VolumeName(path)
	cur := vol + string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(path[len(vol):], string(filepath.Separator)), string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrModelPath, err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s is a symlink", ErrModelPath, cur)
		}
		if cur != path && !safeDir(fi) {
			return fmt.Errorf("%w: %s can be changed by other users", ErrModelPath, cur)
		}
	}
	return nil
}
