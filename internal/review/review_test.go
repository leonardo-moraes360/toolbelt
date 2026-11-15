package review

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/mem"
)

func TestLoadTicketYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rd.md")
	content := `---
title: Fix auth
base_commit: abc123
scope:
  - src/auth.go
  - src/handlers/
forbidden:
  - tests/hidden/**
acceptance:
  - login returns 200
---

Detalhes do ticket.
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ticket, err := LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.BaseCommit != "abc123" {
		t.Fatalf("base=%q", ticket.BaseCommit)
	}
	if len(ticket.Scope) != 2 {
		t.Fatalf("scope=%v", ticket.Scope)
	}
}

func TestDetectDriftOutOfScope(t *testing.T) {
	ticket := &TicketScope{
		BaseCommit: "base",
		Scope:      []string{"src/auth.go"},
		Forbidden:  []string{"tests/hidden/**"},
	}
	rep := DetectDrift(ticket, []string{"src/auth.go", "README.md", "tests/hidden/x_test.go"}, "other")
	if !rep.BaseMismatch {
		t.Fatal("expected base mismatch")
	}
	if len(rep.OutOfScope) != 2 {
		t.Fatalf("out of scope=%v", rep.OutOfScope)
	}
	if len(rep.ForbiddenHit) != 1 {
		t.Fatalf("forbidden=%v", rep.ForbiddenHit)
	}
}

func TestLoadTicketMarkdownHeadingScope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rd.md")
	content := "# Fix ErrPrefix\n\n## Scope\n\n- command.go\n- command_test.go\n- `rd.md`\n\n## Notes\n\nOther section.\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ticket, err := LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"command.go", "command_test.go", "rd.md"}
	if len(ticket.Scope) != len(want) {
		t.Fatalf("scope=%v want %v", ticket.Scope, want)
	}
	for i, s := range want {
		if ticket.Scope[i] != s {
			t.Fatalf("scope[%d]=%q want %q", i, ticket.Scope[i], s)
		}
	}
}

func TestLoadTicketMarkdownEscopoLists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rd.md")
	content := `## Escopo

* scripts/check.sh
1. ` + "`src/main.go`" + `
2) internal/review/review.go

base_commit: abc
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ticket, err := LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.BaseCommit != "abc" {
		t.Fatalf("base=%q", ticket.BaseCommit)
	}
	want := []string{"scripts/check.sh", "src/main.go", "internal/review/review.go"}
	if len(ticket.Scope) != len(want) {
		t.Fatalf("scope=%v want %v", ticket.Scope, want)
	}
	for i, s := range want {
		if ticket.Scope[i] != s {
			t.Fatalf("scope[%d]=%q want %q", i, ticket.Scope[i], s)
		}
	}
}

func TestLoadTicketMarkdownScopeMultilineLabel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rd.md")
	content := `Scope:
- command.go
- command_test.go
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ticket, err := LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ticket.Scope) != 2 || ticket.Scope[0] != "command.go" || ticket.Scope[1] != "command_test.go" {
		t.Fatalf("scope=%v", ticket.Scope)
	}
}

func TestLoadTicketMarkdownScopeSameLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rd.md")
	content := "Scope: command.go, command_test.go, rd.md\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ticket, err := LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ticket.Scope) != 3 {
		t.Fatalf("scope=%v", ticket.Scope)
	}
}

func TestInScopeMidDoubleStar(t *testing.T) {
	cases := []struct {
		file, pattern string
		want          bool
	}{
		{"src/foo.go", "src/**/foo.go", true},
		{"src/a/foo.go", "src/**/foo.go", true},
		{"src/a/b/foo.go", "src/**/foo.go", true},
		{"pkg/foo.go", "src/**/foo.go", false},
		{"src/a/b.go", "src/**/*.go", true},
		{"src/x.go", "src/*.go", true},
		{"src/a/x.go", "src/*.go", false},
		{"src/a/b.go", "src/**", true},
		{"src", "src/**", true},
	}
	for _, tc := range cases {
		got := inScope(tc.file, []string{tc.pattern})
		if got != tc.want {
			t.Fatalf("inScope(%q, %q)=%v want %v", tc.file, tc.pattern, got, tc.want)
		}
	}
}

func TestDetectDriftWithMarkdownScopeGlobs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rd.md")
	content := `## Scope

- src/**/*.go
- scripts/*.sh
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ticket, err := LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	rep := DetectDrift(ticket, []string{"src/pkg/a.go", "scripts/check.sh", "README.md"}, "HEAD")
	if len(rep.OutOfScope) != 1 || rep.OutOfScope[0] != "README.md" {
		t.Fatalf("out of scope=%v", rep.OutOfScope)
	}
}

func TestReviewFreshInvariants(t *testing.T) {
	ctx := FreshContext{
		Reviewer: "goldfish-1",
		Ticket: &TicketScope{
			Scope: []string{"src/"},
		},
		Changed: []string{"src/a.go", "tests/hidden/t.go"},
		Diff:    "diff --git a/tests/hidden/t.go",
		Invariants: []mem.Invariant{
			{ID: "1", Rule: "NUNCA modifique arquivos em tests/hidden/"},
		},
		Standards: []string{"scope", "invariants"},
	}
	findings := Fresh(&ctx)
	if len(findings) == 0 {
		t.Fatal("expected findings")
	}
}

func TestRunOnRepo(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
	if err := os.MkdirAll(filepath.Join(repo, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "b.go"), []byte("package src\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rd := `---
title: add b
base_commit: HEAD~1
scope:
  - src/
---
`
	if err := os.WriteFile(filepath.Join(repo, "rd.md"), []byte(rd), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "add b")

	cfg := DefaultConfig(repo)
	cfg.TicketPath = filepath.Join(repo, "rd.md")
	cfg.BaseRef = "HEAD~1"
	cfg.HeadRef = "HEAD"
	cfg.Reviewers = 2
	rep, err := Run(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.FreshOK {
		t.Fatal("fresh context required")
	}
	if rep.Drift == nil {
		t.Fatal("expected drift report")
	}
}
