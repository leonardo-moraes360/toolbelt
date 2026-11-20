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
// FIFO, checks with fstat that the open descriptor is a regular file (a file
// swapped after the scan's Lstat is refused), and reads at most limit bytes.
func readNote(path string, limit int64) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|openFlags, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
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
