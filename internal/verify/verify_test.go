package verify

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRunPass(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\ngo 1.22\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\nfunc Add(a,b int) int { return a+b }\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "x_test.go"), []byte("package x\nimport \"testing\"\nfunc TestAdd(t *testing.T){ if Add(1,2)!=3 { t.Fatal() } }\n"), 0o644)
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "c")

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = time.Minute
	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s", res.Verdict)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}
