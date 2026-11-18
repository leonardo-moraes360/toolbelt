package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureLocalGitExclude(t *testing.T) {
	dir := initGitRepo(t)

	path, err := EnsureLocalGitExclude(dir)
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("exclude path vazio")
	}
	assertExcludeContents(t, path)

	again, err := EnsureLocalGitExclude(dir)
	if err != nil || again != path {
		t.Fatalf("idempotente: %v %q", err, again)
	}
	data2, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data2), gitExcludeBegin) != 1 {
		t.Fatalf("seção duplicada:\n%s", data2)
	}

	assertIgnoreSplit(t, dir)
}

func TestEnsureLocalGitExcludeNoGit(t *testing.T) {
	path, err := EnsureLocalGitExclude(t.TempDir())
	if err != nil || path != "" {
		t.Fatalf("sem .git: path=%q err=%v", path, err)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "t@t")
	run("git", "config", "user.name", "t")
	return dir
}

func assertExcludeContents(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, p := range LocalCacheExcludePatterns {
		if !strings.Contains(text, p) {
			t.Fatalf("faltou %s em:\n%s", p, text)
		}
	}
	if strings.Contains(text, ".toolbelt/invariants.yaml") {
		t.Fatal("invariants não deve ser excluído")
	}
	if strings.Contains(text, ".toolbelt/\n") || strings.Contains(text, ".toolbelt/\r") {
		t.Fatal("não deve ignorar .toolbelt/ inteiro")
	}
}

func assertIgnoreSplit(t *testing.T, dir string) {
	t.Helper()
	toolbelt := filepath.Join(dir, ".toolbelt")
	if err := os.MkdirAll(filepath.Join(toolbelt, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"config.yaml":     "x: 1\n",
		"invariants.yaml": "version: 1\n",
		"graph.json":      "{}\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(toolbelt, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	check := exec.Command("git", "check-ignore", "-v",
		".toolbelt/graph.json",
		".toolbelt/logs/",
		".toolbelt/config.yaml",
		".toolbelt/invariants.yaml",
	)
	check.Dir = dir
	out, _ := check.CombinedOutput()
	got := string(out)
	if !strings.Contains(got, ".toolbelt/graph.json") {
		t.Fatalf("graph deveria ser ignorado:\n%s", got)
	}
	if strings.Contains(got, "config.yaml") || strings.Contains(got, "invariants.yaml") {
		t.Fatalf("config/invariants não deveriam ser ignorados:\n%s", got)
	}
}

// Dogfood #17: numa git worktree o exclude vai para $GIT_COMMON_DIR/info/exclude,
// o único que o git lê; o info/ do gitdir da worktree é ignorado.
func TestEnsureLocalGitExcludeWorktree(t *testing.T) {
	main := initGitRepo(t)
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	run(main, "git", "commit", "--allow-empty", "-m", "init")
	wt := filepath.Join(t.TempDir(), "wt")
	run(main, "git", "worktree", "add", "-q", wt)

	path, err := EnsureLocalGitExclude(wt)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(main, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("exclude em %q, want %q", got, want)
	}
	assertIgnoreSplit(t, wt)
}
