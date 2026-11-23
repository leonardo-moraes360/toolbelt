package verify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInsideDirWalk(t *testing.T) {
	top := t.TempDir()
	nested := filepath.Join(top, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if !insideDir(top, filepath.Join(nested, "c.yaml")) {
		t.Fatal("nested file must be inside")
	}
	if !insideDir(top, filepath.Join(top, "c.yaml")) {
		t.Fatal("top-level file must be inside")
	}
	if insideDir(top, filepath.Join(t.TempDir(), "c.yaml")) {
		t.Fatal("file in another directory must be outside")
	}
}
