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

func TestLoadTicketFrontmatterEmptyScopeUsesBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rd.md")
	content := `---
title: ValidateStruct
base_commit: abc123
forbidden:
  - tests/hidden/**
---

## Escopo

- binding/**/*.go
- binding/
- rd.md
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ticket, err := LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Title != "ValidateStruct" {
		t.Fatalf("title=%q", ticket.Title)
	}
	if ticket.BaseCommit != "abc123" {
		t.Fatalf("base=%q", ticket.BaseCommit)
	}
	if len(ticket.Forbidden) != 1 || ticket.Forbidden[0] != "tests/hidden/**" {
		t.Fatalf("forbidden=%v", ticket.Forbidden)
	}
	want := []string{"binding/**/*.go", "binding/", "rd.md"}
	if len(ticket.Scope) != len(want) {
		t.Fatalf("scope=%v want %v", ticket.Scope, want)
	}
	for i, s := range want {
		if ticket.Scope[i] != s {
			t.Fatalf("scope[%d]=%q want %q", i, ticket.Scope[i], s)
		}
	}
}

func TestLoadTicketFrontmatterScopeWinsOverBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rd.md")
	content := `---
title: Keep YAML scope
scope:
  - src/auth.go
---

## Scope

- other/path.go
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ticket, err := LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ticket.Scope) != 1 || ticket.Scope[0] != "src/auth.go" {
		t.Fatalf("scope=%v want [src/auth.go]", ticket.Scope)
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

func TestDetectDriftHeadCaretResolvesToSHA(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
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
	run("commit", "-m", "base")
	baseSHA := gitRevParse(t, repo, "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "head")

	ticket := &TicketScope{BaseCommit: baseSHA, Scope: []string{"a.go"}}
	cfg := DefaultConfig(repo)
	cfg.TicketPath = filepath.Join(repo, "rd.md")
	cfg.BaseRef = "HEAD^"
	cfg.HeadRef = "HEAD"
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	rd := "---\ntitle: t\nbase_commit: " + baseSHA + "\nscope:\n  - a.go\n---\n"
	if err := os.WriteFile(cfg.TicketPath, []byte(rd), 0o600); err != nil {
		t.Fatal(err)
	}

	raw := DetectDrift(ticket, []string{"a.go"}, "HEAD^")
	if !raw.BaseMismatch {
		t.Fatal("sem resolve, HEAD^ vs SHA deveria mismatch por string")
	}

	rep, err := Run(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Drift == nil {
		t.Fatal("expected drift")
	}
	if rep.Drift.BaseMismatch {
		t.Fatalf("HEAD^ deveria resolver para o mesmo SHA do ticket; actual=%q declared=%q",
			rep.Drift.ActualBase, rep.Drift.DeclaredBase)
	}
	if rep.Drift.ActualBase != baseSHA {
		t.Fatalf("actual_base=%q want resolved SHA %q", rep.Drift.ActualBase, baseSHA)
	}
}

func TestDetectDriftResolvedStillMismatch(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
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
	run("commit", "-m", "c1")
	first := gitRevParse(t, repo, "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\nx\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "c2")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\nxy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "c3")

	rd := "---\ntitle: t\nbase_commit: " + first + "\nscope:\n  - a.go\n---\n"
	path := filepath.Join(repo, "rd.md")
	if err := os.WriteFile(path, []byte(rd), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig(repo)
	cfg.TicketPath = path
	cfg.BaseRef = "HEAD^"
	cfg.HeadRef = "HEAD"
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	rep, err := Run(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Drift == nil || !rep.Drift.BaseMismatch {
		t.Fatal("bases diferentes após resolve ainda devem mismatch")
	}
}

func gitRevParse(t *testing.T, repo, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "rev-parse", ref).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}
