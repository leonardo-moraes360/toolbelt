package vault

import (
	"errors"
	"io"
	"os"
)

var (
	errNotRegular = errors.New("not a regular file")
	errTooLarge   = errors.New("larger than vault.max_note_bytes")
)

// readNote reads a note without following a final symlink or blocking on a
// FIFO, checks with fstat that the open descriptor is the regular file the
// walk listed (a swap after the listing is refused), and reads at most limit
// bytes.
func readNote(path string, listed os.FileInfo, limit int64) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|openFlags, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	// The open descriptor must be the very file the walk listed: a swap for
	// a symlink (on Windows, where O_NOFOLLOW does not exist) or of an
	// intermediate directory after the listing is refused.
	if !fi.Mode().IsRegular() || (listed != nil && !os.SameFile(listed, fi)) {
		return nil, errNotRegular
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errTooLarge
	}
	return b, nil
}
