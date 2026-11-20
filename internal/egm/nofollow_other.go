//go:build !unix

package egm

// oNoFollow is 0 where O_NOFOLLOW does not exist; os.Root still keeps the
// receipt write inside the repository.
const oNoFollow = 0
