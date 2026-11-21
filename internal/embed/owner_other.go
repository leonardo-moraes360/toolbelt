//go:build !unix

package embed

import "os"

// ownedByUser: Windows has no uid; the path and hash checks still apply.
func ownedByUser(os.FileInfo) bool { return true }

// safeDir: Windows directory ACLs are not checked.
func safeDir(os.FileInfo) bool { return true }

func openNoFollow(path string) (*os.File, error) {
	return os.Open(path) //nolint:gosec // checked by CheckModel
}
