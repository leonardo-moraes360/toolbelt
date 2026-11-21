package embed

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/embed/native"
)

// tempDir resolves t.TempDir: on macOS it lives under /var, a symlink,
// which CheckModel refuses.
func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckModel(t *testing.T) {
	dir := tempDir(t)
	p := filepath.Join(dir, "m.gguf")
	if err := os.WriteFile(p, []byte("GGUF fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("GGUF fake"))
	good := hex.EncodeToString(sum[:])
	if _, err := CheckModel(p, good); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		path, sha string
		want      error
	}{
		"relative":  {"m.gguf", good, ErrModelPath},
		"unclean":   {dir + "/./m.gguf", good, ErrModelPath},
		"missing":   {filepath.Join(dir, "nope"), good, ErrModelPath},
		"directory": {dir, good, ErrModelPath},
		"mismatch":  {p, good[:63] + "0", ErrModelMismatch},
		"bad sha":   {p, "ABC", ErrModelMismatch},
	} {
		if _, err := CheckModel(tc.path, tc.sha); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", name, err, tc.want)
		}
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link.gguf")
		if err := os.Symlink(p, link); err != nil {
			t.Fatal(err)
		}
		if _, err := CheckModel(link, good); !errors.Is(err, ErrModelPath) {
			t.Fatalf("symlink: %v", err)
		}
	}
}

// fakeModel writes a fake model in a fresh real directory and returns its
// path and sha256.
func fakeModel(t *testing.T) (dir, path, sum string) {
	t.Helper()
	dir = tempDir(t)
	path = filepath.Join(dir, "m.gguf")
	if err := os.WriteFile(path, []byte("GGUF fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte("GGUF fake"))
	return dir, path, hex.EncodeToString(h[:])
}

func TestCheckModelParentWritable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no unix permissions")
	}
	dir, _, good := fakeModel(t)
	open := filepath.Join(dir, "open")
	if err := os.Mkdir(open, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(open, 0o777); err != nil {
		t.Fatal(err)
	}
	q := filepath.Join(open, "m.gguf")
	if err := os.WriteFile(q, []byte("GGUF fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckModel(q, good); !errors.Is(err, ErrModelPath) {
		t.Fatalf("world-writable parent: %v", err)
	}
}

func TestSameModelSwapped(t *testing.T) {
	dir, p, good := fakeModel(t)
	hashed, err := CheckModel(p, good)
	if err != nil {
		t.Fatal(err)
	}
	if err := SameModel(p, hashed); err != nil {
		t.Fatalf("same model: %v", err)
	}
	// The new file exists before the rename, so it cannot reuse the inode.
	swap := filepath.Join(dir, "swap.gguf")
	if err := os.WriteFile(swap, []byte("GGUF fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(swap, p); err != nil {
		t.Fatal(err)
	}
	if err := SameModel(p, hashed); !errors.Is(err, ErrModelMismatch) {
		t.Fatalf("swapped model: %v", err)
	}
}

func TestSmokeUnavailable(t *testing.T) {
	if native.Available {
		t.Skip("native engine built in")
	}
	if _, err := Smoke("/x", "", 1); !errors.Is(err, native.ErrUnavailable) {
		t.Fatalf("%v", err)
	}
}

func TestOutsideRepo(t *testing.T) {
	repo := t.TempDir()
	other := t.TempDir()
	inside := filepath.Join(repo, "models", "m.gguf")
	for _, c := range []struct {
		model, root string
		ok          bool
	}{
		{filepath.Join(other, "m.gguf"), repo, true},
		{inside, repo, false},
		{filepath.Join(repo, "m.gguf"), repo, false},
		{inside, "", true},
		// A sibling whose name only shares the prefix is outside.
		{repo + "-models/m.gguf", repo, true},
	} {
		err := OutsideRepo(c.model, c.root)
		if (err == nil) != c.ok || (err != nil && !errors.Is(err, ErrModelPath)) {
			t.Errorf("OutsideRepo(%q, %q) = %v, want ok=%v", c.model, c.root, err, c.ok)
		}
	}
}
