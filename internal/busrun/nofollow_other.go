//go:build !unix

package busrun

// openNoFollow is 0 where the flags do not exist; the fstat after opening
// still refuses anything but a regular file.
const openNoFollow = 0
