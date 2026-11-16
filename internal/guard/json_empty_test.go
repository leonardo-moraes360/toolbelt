package guard

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCheckResultEmptyViolationsAreArray(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")
	git(t, dir, "config", "user.email", "t@t")
	git(t, dir, "config", "user.name", "t")
	write(t, filepath.Join(dir, "a.txt"), "a\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "base")
	write(t, filepath.Join(dir, "a.txt"), "b\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "head")

	res, err := NewChecker(dir).CheckDiff("HEAD~1", "HEAD", "")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	v, ok := payload["violations"]
	if !ok {
		t.Fatal("missing violations")
	}
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("violations=%T want [] (got null/other): %s", v, raw)
	}
	if arr == nil {
		t.Fatalf("violations null: %s", raw)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
