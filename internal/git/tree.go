package git

import (
	"fmt"
	"strings"
)

// ModeSymlink is the git tree mode of a symbolic link.
const ModeSymlink = "120000"

// TreeMode returns the tree mode of path (relative to the repository root)
// at ref, or "" when ref has no entry at path. ref must be a commit or tree.
func TreeMode(dir, ref, path string) (string, error) {
	out, err := Output(dir, "ls-tree", "--full-tree", ref, "--", path)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return "", nil
	}
	mode, _, ok := strings.Cut(line, " ")
	if !ok {
		return "", fmt.Errorf("git ls-tree %s -- %s: unexpected output %q", ref, path, line)
	}
	return mode, nil
}
