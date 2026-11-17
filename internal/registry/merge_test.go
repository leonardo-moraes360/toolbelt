package registry

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

func mergeToolGit(t *testing.T, dir string, args ...string) string {
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

func TestTbMergeDryRunUnlessApply(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	mergeToolGit(t, dir, "init", "-q", "-b", "main")
	mergeToolGit(t, dir, "config", "user.name", "t")
	mergeToolGit(t, dir, "config", "user.email", "t@t")
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".toolbelt/\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
	mergeToolGit(t, dir, "add", "-A")
	mergeToolGit(t, dir, "commit", "-q", "-m", "base")

	s := ticket.Open(dir)
	tk, err := s.Create(&ticket.Ticket{Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	base := mergeToolGit(t, dir, "rev-parse", "main")
	mergeToolGit(t, dir, "checkout", "-q", "-b", "w/1", base)
	os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x\n"), 0o644)
	mergeToolGit(t, dir, "add", "-A")
	mergeToolGit(t, dir, "commit", "-q", "-m", "x")
	head := mergeToolGit(t, dir, "rev-parse", "HEAD")
	mergeToolGit(t, dir, "checkout", "-q", "main")
	s.Transition(tk.ID, ticket.StatusRunning)
	if _, err := s.Update(tk.ID, func(x *ticket.Ticket) error {
		x.Status = ticket.StatusDone
		x.Result = &ticket.Result{BaseCommit: base, HeadCommit: head}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	res, text := callTool(t, "tb_merge", map[string]any{"cwd": dir, "ids": []any{"TKT-001"}})
	if res.IsError || !strings.Contains(text, `"dry_run": true`) && !strings.Contains(text, `"dry_run":true`) {
		t.Fatalf("dry-run: %v %s", res.IsError, text)
	}
	if mergeToolGit(t, dir, "rev-parse", "main") != base {
		t.Fatal("dry-run moveu ref")
	}
	res, text = callTool(t, "tb_merge", map[string]any{"cwd": dir, "ids": []any{"TKT-001"}, "apply": true})
	if res.IsError {
		t.Fatal(text)
	}
	if mergeToolGit(t, dir, "rev-parse", "main") == base {
		t.Fatal("apply não avançou main")
	}
	res, text = callTool(t, "tb_merge", map[string]any{"cwd": dir, "ids": []any{"TKT-001"}, "strategy": "prompt"})
	if !res.IsError || !strings.Contains(text, "não implementado") {
		t.Fatalf("prompt: %v %s", res.IsError, text)
	}
}

func TestTbMergeConflictReturnsReport(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	mergeToolGit(t, dir, "init", "-q", "-b", "main")
	mergeToolGit(t, dir, "config", "user.name", "t")
	mergeToolGit(t, dir, "config", "user.email", "t@t")
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".toolbelt/\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
	mergeToolGit(t, dir, "add", "-A")
	mergeToolGit(t, dir, "commit", "-q", "-m", "base")
	base := mergeToolGit(t, dir, "rev-parse", "main")

	s := ticket.Open(dir)
	for i, content := range []string{"one\n", "two\n"} {
		tk, err := s.Create(&ticket.Ticket{Title: "t"})
		if err != nil {
			t.Fatal(err)
		}
		mergeToolGit(t, dir, "checkout", "-q", "-b", "w/"+tk.ID, base)
		os.WriteFile(filepath.Join(dir, "a.txt"), []byte(content), 0o644)
		mergeToolGit(t, dir, "add", "-A")
		mergeToolGit(t, dir, "commit", "-q", "-m", "c"+string(rune('0'+i)))
		head := mergeToolGit(t, dir, "rev-parse", "HEAD")
		mergeToolGit(t, dir, "checkout", "-q", "main")
		s.Transition(tk.ID, ticket.StatusRunning)
		if _, err := s.Update(tk.ID, func(x *ticket.Ticket) error {
			x.Status = ticket.StatusDone
			x.Result = &ticket.Result{BaseCommit: base, HeadCommit: head}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	res, text := callTool(t, "tb_merge", map[string]any{"cwd": dir, "ids": []any{"TKT-001", "TKT-002"}, "apply": true})
	var payload struct {
		OK        bool   `json:"ok"`
		Error     string `json:"error"`
		Conflicts []struct {
			File string `json:"file"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("payload inválido (%v): %s", err, text)
	}
	if res.IsError && len(payload.Conflicts) == 0 {
		t.Fatalf("relatório descartado: %s", text)
	}
	if payload.OK || !strings.Contains(payload.Error, "conflito") || len(payload.Conflicts) != 1 || payload.Conflicts[0].File != "a.txt" {
		t.Fatalf("payload: %s", text)
	}
	if mergeToolGit(t, dir, "rev-parse", "main") != base {
		t.Fatal("ref mudou")
	}
}

func TestTbMergeSensitiveRefusedUnlessAllowed(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	mergeToolGit(t, dir, "init", "-q", "-b", "main")
	mergeToolGit(t, dir, "config", "user.name", "t")
	mergeToolGit(t, dir, "config", "user.email", "t@t")
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".toolbelt/\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
	mergeToolGit(t, dir, "add", "-A")
	mergeToolGit(t, dir, "commit", "-q", "-m", "base")
	base := mergeToolGit(t, dir, "rev-parse", "main")
	s := ticket.Open(dir)
	tk, _ := s.Create(&ticket.Ticket{Title: "t"})
	mergeToolGit(t, dir, "checkout", "-q", "-b", "w/1", base)
	os.MkdirAll(filepath.Join(dir, ".githooks"), 0o755)
	os.WriteFile(filepath.Join(dir, ".githooks", "post-merge"), []byte("#!/bin/sh\n"), 0o755)
	mergeToolGit(t, dir, "add", "-A")
	mergeToolGit(t, dir, "commit", "-q", "-m", "h")
	head := mergeToolGit(t, dir, "rev-parse", "HEAD")
	mergeToolGit(t, dir, "checkout", "-q", "main")
	s.Transition(tk.ID, ticket.StatusRunning)
	if _, err := s.Update(tk.ID, func(x *ticket.Ticket) error {
		x.Status = ticket.StatusDone
		x.Result = &ticket.Result{BaseCommit: base, HeadCommit: head}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	_, text := callTool(t, "tb_merge", map[string]any{"cwd": dir, "ids": []any{"TKT-001"}, "apply": true})
	if !strings.Contains(text, `"ok":false`) && !strings.Contains(text, `"ok": false`) || !strings.Contains(text, ".githooks/post-merge") {
		t.Fatalf("esperava recusa com sensitive: %s", text)
	}
	if mergeToolGit(t, dir, "rev-parse", "main") != base {
		t.Fatal("ref mudou")
	}
	res, text := callTool(t, "tb_merge", map[string]any{"cwd": dir, "ids": []any{"TKT-001"}, "apply": true, "allow_sensitive": true})
	if res.IsError || mergeToolGit(t, dir, "rev-parse", "main") == base {
		t.Fatalf("allow_sensitive: %v %s", res.IsError, text)
	}
}
