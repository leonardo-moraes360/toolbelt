package review

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestSuggestsViolationPathNotSubstring(t *testing.T) {
	rule := "nunca modifique arquivos em tests/hidden/"
	diffMentions := "diff --git a/rd.md\n+forbidden:\n+  - tests/hidden/**\n"

	if suggestsViolation(rule, strings.ToLower(diffMentions), []string{"rd.md", "src/main.go"}) {
		t.Fatal("mention of tests/hidden in ticket diff must not flag invariant")
	}
	if !suggestsViolation(rule, "diff --git a/tests/hidden/t.go", []string{"tests/hidden/t.go"}) {
		t.Fatal("real change under tests/hidden/ must flag invariant")
	}
	if suggestsViolation(rule, "", []string{"mytests/hidden/x.go"}) {
		t.Fatal("path that only contains substring tests/hidden must not flag")
	}
	if suggestsViolation(rule, "", []string{"docs/about-tests/hidden-guide.md"}) {
		t.Fatal("path with substring across segments must not flag")
	}
	if !suggestsViolation(rule, "", []string{"pkg/tests/hidden/extra.go"}) {
		t.Fatal("tests/hidden as path segments must flag")
	}
}

func TestPathMarkerHit(t *testing.T) {
	cases := []struct {
		file   string
		marker string
		want   bool
	}{
		{"tests/hidden/t.go", "tests/hidden", true},
		{"tests/hidden", "tests/hidden", true},
		{"mytests/hidden/x.go", "tests/hidden", false},
		{"docs/about-tests/hidden-guide.md", "tests/hidden", false},
		{"pkg/tests/hidden/extra.go", "tests/hidden", true},
		{".toolbelt/thresholds.yaml", "thresholds.yaml", true},
		{"docs/thresholds.yaml.bak", "thresholds.yaml", false},
	}
	for _, tc := range cases {
		if got := pathMarkerHit(tc.file, tc.marker); got != tc.want {
			t.Fatalf("pathMarkerHit(%q, %q)=%v want %v", tc.file, tc.marker, got, tc.want)
		}
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
