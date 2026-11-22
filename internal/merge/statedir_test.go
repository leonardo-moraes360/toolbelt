//go:build unix

package merge

import (
	"os"
	"testing"
)

func TestWriteReceiptSecuresStateDir(t *testing.T) {
	t.Setenv(EnvStateDir, t.TempDir())
	top := t.TempDir()
	dir, err := receiptDir(top)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // a deliberately loose directory
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil { //nolint:gosec // idem
		t.Fatal(err)
	}
	if _, err := writeReceipt(top, &Receipt{}); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
	if os.Getuid() == 0 {
		if err := os.Chown(dir, 12345, 12345); err != nil {
			t.Fatal(err)
		}
		if _, err := writeReceipt(top, &Receipt{}); err == nil {
			t.Fatal("a directory owned by another user must be refused")
		}
	}
}
