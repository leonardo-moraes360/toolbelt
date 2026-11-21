package vecsearch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/graph"
)

// Vectors are built from root-relative units, so only the root may be
// indexed with vectors.
func TestCheckVectorRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".toolbelt"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	s := graph.NewStore(graph.DefaultPath(filepath.Join(root, ".toolbelt")))
	if err := CheckVectorRoot(s, root); err != nil {
		t.Fatalf("root refused: %v", err)
	}
	if err := CheckVectorRoot(s, filepath.Join(root, "sub")); err == nil {
		t.Fatal("subtree accepted")
	}
}
