package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		if err := c.Run(); err != nil {
			t.Fatal(err)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "t@t")
	run("git", "config", "user.name", "t")
	return dir
}

func TestDetectTestDeletion(t *testing.T) {
	dir := setupRepo(t)
	_ = os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("package x\n"), 0o644)
	run := exec.Command("git", "add", "a_test.go")
	run.Dir = dir
	_ = run.Run()
	run = exec.Command("git", "commit", "-m", "a")
	run.Dir = dir
	_ = run.Run()
	baseOut, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	base := string(baseOut[:len(baseOut)-1])
	_ = os.Remove(filepath.Join(dir, "a_test.go"))
	run = exec.Command("git", "add", "-A")
	run.Dir = dir
	_ = run.Run()
	run = exec.Command("git", "commit", "-m", "del")
	run.Dir = dir
	_ = run.Run()
	headOut, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	head := string(headOut[:len(headOut)-1])

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Violations) == 0 {
		t.Fatal("esperava violação")
	}
}
