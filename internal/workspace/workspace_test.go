package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "a@b.c")
	run("git", "config", "user.name", "t")
	_ = os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644)
	run("git", "add", "f")
	run("git", "commit", "-m", "init")
	return dir
}

func TestCreateWorkspace(t *testing.T) {
	repo := initGitRepo(t)
	mgr, err := NewManager(repo)
	if err != nil {
		t.Fatal(err)
	}
	ws, err := mgr.Create("w1", "HEAD", Scope{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ws.Path); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mgr.Remove(ws.ID) }()
	list, err := mgr.List()
	if err != nil || len(list) == 0 {
		t.Fatalf("list: %v %d", err, len(list))
	}
}
