package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSandboxRun(t *testing.T) {
	dir := t.TempDir()
	sb, err := New(Config{WorkDir: dir, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	res, err := sb.Run(context.Background(), "echo", "ok")
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit %d", res.ExitCode)
	}
}

func TestSandboxValidateForbidden(t *testing.T) {
	dir := t.TempDir()
	forbidden := filepath.Join(dir, "secret")
	_ = os.MkdirAll(forbidden, 0o755)
	sb, err := New(Config{WorkDir: dir, ForbiddenPaths: []string{forbidden}})
	if err != nil {
		t.Fatal(err)
	}
	if sb.ValidatePath(filepath.Join(forbidden, "x")) == nil {
		t.Fatal("deveria negar")
	}
}
