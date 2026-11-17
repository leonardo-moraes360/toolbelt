package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

func mergeGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func mergeRepo(t *testing.T) string {
	t.Helper()
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	mergeGit(t, dir, "init", "-q", "-b", "main")
	mergeGit(t, dir, "config", "user.name", "t")
	mergeGit(t, dir, "config", "user.email", "t@t")
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".toolbelt/\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
	mergeGit(t, dir, "add", "-A")
	mergeGit(t, dir, "commit", "-q", "-m", "base")
	return dir
}

func mergeTicket(t *testing.T, dir, file, content string) {
	t.Helper()
	s := ticket.Open(dir)
	tk, err := s.Create(&ticket.Ticket{Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	base := mergeGit(t, dir, "rev-parse", "main")
	mergeGit(t, dir, "checkout", "-q", "-b", "w/"+strings.ToLower(tk.ID), base)
	os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644)
	mergeGit(t, dir, "add", "-A")
	mergeGit(t, dir, "commit", "-q", "-m", tk.ID)
	head := mergeGit(t, dir, "rev-parse", "HEAD")
	mergeGit(t, dir, "checkout", "-q", "main")
	if _, err := s.Transition(tk.ID, ticket.StatusRunning); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(tk.ID, func(x *ticket.Ticket) error {
		x.Status = ticket.StatusDone
		x.Result = &ticket.Result{BaseCommit: base, HeadCommit: head}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMergeCLI(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	bin := buildTB(t)
	dir := mergeRepo(t)
	mergeTicket(t, dir, "x.txt", "um\n")
	mergeTicket(t, dir, "x.txt", "dois\n")
	start := mergeGit(t, dir, "rev-parse", "main")

	mergeCLIRejections(t, bin, dir, start)
	mergeCLIApplyLastWins(t, bin, dir)
	if out, err := runTB(t, bin, dir, "merge", "1"); err == nil || !strings.Contains(out, "--force") {
		t.Fatalf("já mergeado: %v %s", err, out)
	}
}

// mergeCLIRejections: estratégia inválida e conflito com fail não movem refs.
func mergeCLIRejections(t *testing.T, bin, dir, start string) {
	t.Helper()
	if out, err := runTB(t, bin, dir, "merge", "--strategy", "semantic", "1"); err == nil || !strings.Contains(out, "não implementado") {
		t.Fatalf("semantic: %v %s", err, out)
	}
	out, err := runTB(t, bin, dir, "merge", "--json", "1", "2")
	if err == nil || !strings.Contains(out, "conflict") {
		t.Fatalf("fail deveria reportar conflito: %v %s", err, out)
	}
	if mergeGit(t, dir, "rev-parse", "main") != start {
		t.Fatal("ref mudou")
	}
}

// mergeCLIApplyLastWins: dry-run e aplicação real com last_wins.
func mergeCLIApplyLastWins(t *testing.T, bin, dir string) {
	t.Helper()
	out, err := runTB(t, bin, dir, "merge", "--dry-run", "--strategy", "last_wins", "1", "2")
	if err != nil || !strings.Contains(out, "last_wins") {
		t.Fatalf("dry-run: %v %s", err, out)
	}
	jout, err := runTB(t, bin, dir, "merge", "--strategy", "last_wins", "--json", "tkt-1", "2")
	if err != nil {
		t.Fatalf("%v %s", err, jout)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(jout), &res); err != nil || res["applied"] != true {
		t.Fatalf("json: %v %s", err, jout)
	}
	if mergeGit(t, dir, "rev-parse", "main") != res["result_sha"] {
		t.Fatal("main não avançou")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "x.txt")); string(b) != "dois\n" {
		t.Fatalf("x.txt = %q", b)
	}
}
