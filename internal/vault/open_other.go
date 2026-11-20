//go:build !unix

package vault

// openFlags is 0 where O_NOFOLLOW/O_NONBLOCK do not exist; the Lstat of the
// scan and the fstat after opening still refuse anything but a regular file.
const openFlags = 0

func isLoop(error) bool { return false }
