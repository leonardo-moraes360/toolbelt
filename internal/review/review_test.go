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
	if rep.ActualBase != "other" || rep.DeclaredBase != "base" {
		t.Fatalf("bases actual=%q declared=%q", rep.ActualBase, rep.DeclaredBase)
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

func TestLoadTicketMarkdownForbiddenHeading(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rd.md")
	content := `## Scope

- src/a.py

## Forbidden

- tests/hidden/**
- secrets/**
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ticket, err := LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"tests/hidden/**", "secrets/**"}
	if len(ticket.Forbidden) != len(want) {
		t.Fatalf("forbidden=%v want %v", ticket.Forbidden, want)
	}
	for i, s := range want {
		if ticket.Forbidden[i] != s {
			t.Fatalf("forbidden[%d]=%q want %q", i, ticket.Forbidden[i], s)
		}
	}
}

func TestLoadTicketMarkdownProibidoLists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rd.md")
	content := `## Proibido

* tests/hidden/**
1. ` + "`thresholds.yaml`" + `
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ticket, err := LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ticket.Forbidden) != 2 || ticket.Forbidden[0] != "tests/hidden/**" || ticket.Forbidden[1] != "thresholds.yaml" {
		t.Fatalf("forbidden=%v", ticket.Forbidden)
	}
}

func TestLoadTicketMarkdownForbiddenMultilineLabel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rd.md")
	content := `Forbidden:
- tests/hidden/**
- .env
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ticket, err := LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ticket.Forbidden) != 2 || ticket.Forbidden[0] != "tests/hidden/**" || ticket.Forbidden[1] != ".env" {
		t.Fatalf("forbidden=%v", ticket.Forbidden)
	}
}

func TestDetectDriftForbiddenHitFromMarkdownGlob(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rd.md")
	content := `## Scope

- src/a.py

## Forbidden

- tests/hidden/**
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	ticket, err := LoadTicket(path)
	if err != nil {
		t.Fatal(err)
	}
	rep := DetectDrift(ticket, []string{"src/a.py", "tests/hidden/cheat.py"}, "HEAD")
	if len(rep.OutOfScope) != 1 || rep.OutOfScope[0] != "tests/hidden/cheat.py" {
		t.Fatalf("out of scope=%v", rep.OutOfScope)
	}
	if len(rep.ForbiddenHit) != 1 || rep.ForbiddenHit[0] != "tests/hidden/cheat.py" {
		t.Fatalf("forbidden_hit=%v", rep.ForbiddenHit)
	}
}

func TestReviewForbiddenAndHiddenInvariant(t *testing.T) {
	ctx := FreshContext{
		Reviewer: "goldfish-1",
		Ticket: &TicketScope{
			Scope:     []string{"src/"},
			Forbidden: []string{"tests/hidden/**"},
		},
		Changed: []string{"src/a.py", "tests/hidden/cheat.py"},
		Diff:    "diff --git a/tests/hidden/cheat.py\n+cheat\n",
		Invariants: []mem.Invariant{
			{ID: "1", Rule: "NUNCA modifique arquivos em tests/hidden/"},
		},
		Standards: []string{"scope", "invariants"},
	}
	findings := Fresh(&ctx)
	var forbidden, invariant bool
	for i := range findings {
		f := &findings[i]
		if f.ID == "forbidden-tests/hidden/cheat.py" && f.Severity == SeverityCritical {
			forbidden = true
		}
		if f.ID == "inv-1" && f.Severity == SeverityCritical {
			invariant = true
		}
	}
	if !forbidden {
		t.Fatalf("expected forbidden finding, got %#v", findings)
	}
	if !invariant {
		t.Fatalf("expected hidden invariant finding, got %#v", findings)
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
	if raw.ActualBase != "HEAD^" || raw.DeclaredBase != baseSHA {
		t.Fatalf("DetectDrift sem enrich: actual=%q declared=%q", raw.ActualBase, raw.DeclaredBase)
	}

	rep, err := Run(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Drift == nil {
		t.Fatal("expected drift")
	}
	if rep.Drift.BaseMismatch {
		t.Fatalf("HEAD^ ancestral não deve mismatch; actual=%q declared=%q",
			rep.Drift.ActualBase, rep.Drift.DeclaredBase)
	}
	if rep.Drift.ActualBase != baseSHA {
		t.Fatalf("actual_base=%q want resolved SHA %q", rep.Drift.ActualBase, baseSHA)
	}
	if rep.Drift.MergeBase == "" {
		t.Fatal("merge_base deve ser preenchido")
	}
}

func TestDetectDriftAncestorBaseNoMismatch(t *testing.T) {
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
	if rep.Drift == nil {
		t.Fatal("expected drift")
	}
	if rep.Drift.BaseMismatch {
		t.Fatal("HEAD^ é ancestral do head; base_mismatch deve ser false mesmo diferindo do ticket")
	}
	if rep.Drift.MergeBase == "" {
		t.Fatal("merge_base vazio")
	}
}

func TestBuildReviewerRosterDistinct(t *testing.T) {
	roster := buildReviewerRoster([]string{"scope", "quality", "acceptance"}, 5)
	if len(roster) != 3 {
		t.Fatalf("expected 3 roles, got %d", len(roster))
	}
	want := []string{"goldfish-scope", "goldfish-quality", "goldfish-acceptance"}
	for i, name := range want {
		if roster[i].Name != name {
			t.Fatalf("roster[%d]=%q want %q", i, roster[i].Name, name)
		}
	}
	if n := len(buildReviewerRoster([]string{"scope", "quality", "acceptance"}, 2)); n != 2 {
		t.Fatalf("limit 2 → got %d", n)
	}
	alias := buildReviewerRoster([]string{"completeness"}, 3)
	if len(alias) != 1 || alias[0].ID != "acceptance" {
		t.Fatalf("completeness alias → %#v", alias)
	}
}

func TestRunDistinctReviewersNoClones(t *testing.T) {
	repo := setupDistinctReviewRepo(t)
	cfg := DefaultConfig(repo)
	cfg.TicketPath = filepath.Join(repo, "rd.md")
	cfg.BaseRef = "HEAD~1"
	cfg.HeadRef = "HEAD"
	cfg.Reviewers = 5
	cfg.Standards = []string{"scope", "quality", "acceptance"}
	rep, err := Run(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Reviewers != 3 {
		t.Fatalf("expected 3 distinct reviewers (not clones), got %d", rep.Reviewers)
	}
	assertDistinctRoleFindings(t, rep.Findings)
}

func setupDistinctReviewRepo(t *testing.T) string {
	t.Helper()
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
	if err := os.MkdirAll(filepath.Join(repo, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "a.go"), []byte("package src\n\nfunc OK() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")

	var big strings.Builder
	big.WriteString("package src\n\nfunc Huge() {\n")
	for i := 0; i < 90; i++ {
		big.WriteString("\t_ = 1\n")
	}
	big.WriteString("}\n\nfunc Other() {\n\t// TODO: finish later\n}\n")
	if err := os.WriteFile(filepath.Join(repo, "src", "a.go"), []byte(big.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	rd := `---
title: quality and acceptance
base_commit: HEAD~1
scope:
  - src/
acceptance:
  - login returns 200 on success
  - session cookie is http-only
---
`
	if err := os.WriteFile(filepath.Join(repo, "rd.md"), []byte(rd), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "change")
	return repo
}

func assertDistinctRoleFindings(t *testing.T, findings []Finding) {
	t.Helper()
	names := map[string]bool{}
	hasQuality, hasAcceptance := false, false
	for i := range findings {
		f := &findings[i]
		names[f.Reviewer] = true
		if f.Reviewer == "goldfish-quality" && f.Category == "quality" {
			hasQuality = true
		}
		if f.Reviewer == "goldfish-acceptance" && f.Category == "acceptance" {
			hasAcceptance = true
		}
	}
	if names["goldfish-1"] || names["goldfish-2"] || names["goldfish-3"] {
		t.Fatalf("identical numbered goldfish still present: %#v", names)
	}
	if !hasQuality || !hasAcceptance {
		t.Fatalf("expected quality+acceptance role findings, got %#v", findings)
	}
}

func TestReviewQualityHeuristics(t *testing.T) {
	ctx := FreshContext{
		Reviewer: "goldfish-quality",
		Ticket:   &TicketScope{Scope: []string{"src/"}},
		Changed:  []string{"src/a.go"},
		Diff: `diff --git a/src/a.go b/src/a.go
--- a/src/a.go
+++ b/src/a.go
@@ -1,3 +1,95 @@
 package src
+
+func Huge() {
` + strings.Repeat("+	x := 1\n", 85) + `+}
+
+func Other() {
+	// TODO: wire this up
+}
`,
		Standards: []string{"quality"},
	}
	findings := Fresh(&ctx)
	var todo, large, noTest bool
	for i := range findings {
		f := &findings[i]
		switch {
		case strings.Contains(f.Message, "TODO") || strings.Contains(f.Message, "FIXME"):
			todo = true
		case strings.Contains(strings.ToLower(f.Message), "grande") || strings.Contains(strings.ToLower(f.Message), "large") || f.ID == "quality-large-func":
			large = true
		case strings.Contains(strings.ToLower(f.Message), "teste") || f.ID == "quality-no-tests":
			noTest = true
		}
		if f.Reviewer != "goldfish-quality" {
			t.Fatalf("quality findings must come from goldfish-quality, got %q", f.Reviewer)
		}
		if f.Category != "quality" {
			t.Fatalf("category=%q want quality", f.Category)
		}
	}
	if !todo || !large || !noTest {
		t.Fatalf("expected TODO, large-func and no-tests findings, got %#v (todo=%v large=%v noTest=%v)", findings, todo, large, noTest)
	}
}

func TestReviewAcceptanceUsesChangedTestsOrFiles(t *testing.T) {
	ctx := FreshContext{
		Reviewer: "goldfish-acceptance",
		Ticket: &TicketScope{
			Scope:      []string{"src/"},
			Acceptance: []string{"login returns 200 on success"},
		},
		Changed:   []string{"src/auth.go"},
		Diff:      "diff --git a/src/auth.go\n+func Auth() {}\n",
		Standards: []string{"acceptance"},
	}
	findings := Fresh(&ctx)
	if len(findings) == 0 {
		t.Fatal("expected uncovered acceptance criterion finding")
	}
	if findings[0].Reviewer != "goldfish-acceptance" || findings[0].Category != "acceptance" {
		t.Fatalf("got %#v", findings[0])
	}
	if findings[0].Criterion == "" || findings[0].Evidence == "" {
		t.Fatalf("acceptance finding must link criterion and evidence, got %#v", findings[0])
	}
	if findings[0].Role != "acceptance" {
		t.Fatalf("role=%q want acceptance", findings[0].Role)
	}

	ctx.Changed = []string{"src/auth.go", "src/auth_test.go"}
	ctx.Diff = "diff --git a/src/auth_test.go b/src/auth_test.go\n--- a/src/auth_test.go\n+++ b/src/auth_test.go\n@@ -0,0 +1,1 @@\n+func TestLoginReturns200OnSuccess(t *testing.T) {}\n"
	findings = Fresh(&ctx)
	for i := range findings {
		f := &findings[i]
		if f.ID == "accept-0" {
			t.Fatalf("criterion evidenced in changed test should pass, got %#v", findings)
		}
	}
}

func TestReviewQualityTODOIgnoresPortugueseTodo(t *testing.T) {
	ctx := FreshContext{
		Reviewer: "goldfish-quality",
		Changed:  []string{"src/a.go", "src/a_test.go"},
		Diff: `diff --git a/src/a.go b/src/a.go
--- a/src/a.go
+++ b/src/a.go
@@ -1,3 +1,5 @@
 package src
+// preciso fazer todo o trabalho amanhã
+return "todo mundo"
`,
		Standards: []string{"quality"},
	}
	findings := Fresh(&ctx)
	for i := range findings {
		f := &findings[i]
		if strings.Contains(f.ID, "todo") || strings.Contains(f.Message, "TODO") {
			t.Fatalf("palavra 'todo' em português não deve gerar finding: %#v", f)
		}
	}
}

func TestReviewQualityTODOUppercaseMarkersInComments(t *testing.T) {
	ctx := FreshContext{
		Reviewer: "goldfish-quality",
		Changed:  []string{"src/a.go", "src/a_test.go"},
		Diff: `diff --git a/src/a.go b/src/a.go
+++ b/src/a.go
+// TODO: wire
+// FIXME: later
+// XXX: risky
+// HACK: temp
+msg := "todo lowercase stays"
`,
		Standards: []string{"quality"},
	}
	findings := Fresh(&ctx)
	var todo bool
	for i := range findings {
		f := &findings[i]
		if strings.Contains(f.ID, "quality-todo") {
			todo = true
			if f.Role != "quality" {
				t.Fatalf("role=%q", f.Role)
			}
		}
	}
	if !todo {
		t.Fatal("expected TODO/FIXME/XXX/HACK finding in comments")
	}
}

func TestReviewAcceptanceLinksCriterionAndEvidence(t *testing.T) {
	ctx := FreshContext{
		Reviewer: "goldfish-acceptance",
		Ticket: &TicketScope{
			Scope:      []string{"src/"},
			Acceptance: []string{"session cookie is http-only"},
		},
		Changed: []string{"src/greet.go", "src/greet_test.go"},
		Diff: `diff --git a/src/greet_test.go b/src/greet_test.go
+++ b/src/greet_test.go
+func TestGreet(t *testing.T) {}
`,
		Standards: []string{"acceptance"},
	}
	findings := Fresh(&ctx)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for unrelated criterion, got %#v", findings)
	}
	f := findings[0]
	if f.Criterion != "session cookie is http-only" {
		t.Fatalf("criterion=%q", f.Criterion)
	}
	if !strings.Contains(f.Evidence, "TestGreet") {
		t.Fatalf("evidence should list tests in diff: %q", f.Evidence)
	}
	if !strings.Contains(f.Message, "session cookie is http-only") {
		t.Fatalf("message should name criterion: %q", f.Message)
	}
}

func TestReviewInvariantsApplyToAddedLines(t *testing.T) {
	ctx := FreshContext{
		Reviewer: "goldfish-scope",
		Ticket:   &TicketScope{Scope: []string{"src/"}},
		Changed:  []string{"src/a.js"},
		Diff: `diff --git a/src/a.js b/src/a.js
+++ b/src/a.js
+  // temporary note
+  console.log("debug")
+  return 1
`,
		Invariants: []mem.Invariant{
			{ID: "1", Rule: "NUNCA use console.log"},
			{ID: "2", Rule: "NUNCA adicione comentários no código"},
		},
		Standards: []string{"invariants"},
	}
	findings := Fresh(&ctx)
	var consoleHit, commentHit bool
	for i := range findings {
		f := &findings[i]
		if f.ID == "inv-1" && strings.Contains(f.Evidence, "console.log") {
			consoleHit = true
		}
		if f.ID == "inv-2" && strings.Contains(strings.ToLower(f.Evidence), "coment") {
			commentHit = true
		}
	}
	if !consoleHit || !commentHit {
		t.Fatalf("expected console.log and comment invariant hits on added lines, got %#v", findings)
	}
}

func TestReviewAcceptanceSeesTestGreet(t *testing.T) {
	ctx := FreshContext{
		Reviewer: "goldfish-acceptance",
		Ticket: &TicketScope{
			Scope:      []string{"greet/"},
			Acceptance: []string{"Greet personalizes the message for the user"},
		},
		Changed: []string{"greet/greet.go", "greet/greet_test.go"},
		Diff: `diff --git a/greet/greet.go b/greet/greet.go
+++ b/greet/greet.go
+func Greet(n string) string { return "hello, " + n }
diff --git a/greet/greet_test.go b/greet/greet_test.go
+++ b/greet/greet_test.go
+func TestGreet(t *testing.T) {
+	if Greet("x") != "hello, x" { t.Fatal(1) }
+}
`,
		Standards: []string{"acceptance"},
	}
	findings := Fresh(&ctx)
	for i := range findings {
		f := &findings[i]
		if f.Category == "acceptance" {
			t.Fatalf("TestGreet should cover Greet criterion, got %#v", f)
		}
	}
}

func TestReviewAcceptanceSeesJSTSTests(t *testing.T) {
	ctx := FreshContext{
		Reviewer: "goldfish-acceptance",
		Ticket: &TicketScope{
			Scope:      []string{"src/"},
			Acceptance: []string{"Greet returns hello"},
		},
		Changed: []string{"src/greet.ts", "src/greet.test.ts"},
		Diff: `diff --git a/src/greet.test.ts b/src/greet.test.ts
+++ b/src/greet.test.ts
+import { greet } from './greet'
+test('Greet returns hello', () => {
+  expect(greet('Ada')).toBe('hello, Ada')
+})
+it('covers edge', () => {})
`,
		Standards: []string{"acceptance"},
	}
	findings := Fresh(&ctx)
	for i := range findings {
		f := &findings[i]
		if f.Category == "acceptance" {
			t.Fatalf("JS/TS test title should cover Greet criterion, got %#v", f)
		}
	}
}

func TestFindingRoleInJSONAndAgreementSemantics(t *testing.T) {
	f := Finding{
		ID: "quality-todo-x", Category: "quality", Reviewer: "goldfish-quality", Agreement: 1,
	}
	normalizeFinding(&f)
	if f.Role != "quality" {
		t.Fatalf("role=%q want quality", f.Role)
	}
	all := aggregateFindings([]Finding{
		{ID: "x", Category: "quality", Role: "quality", Message: "m", Reviewer: "a", Agreement: 1},
		{ID: "x", Category: "quality", Role: "acceptance", Message: "m", Reviewer: "b", Agreement: 1},
	})
	if len(all) != 1 || all[0].Agreement != 2 {
		t.Fatalf("expected role_agreement=2, got %#v", all)
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

func initGitRepo(t *testing.T, repo string) func(args ...string) {
	t.Helper()
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
	return run
}

func writeTicket(t *testing.T, repo, scopeFile string) string {
	t.Helper()
	path := filepath.Join(repo, "rd.md")
	rd := "---\ntitle: t\nscope:\n  - " + scopeFile + "\n---\n"
	if err := os.WriteFile(path, []byte(rd), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckPinnedApprovalEmptyRangeClearError(t *testing.T) {
	repo := t.TempDir()
	run := initGitRepo(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "base")
	base := gitRevParse(t, repo, "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\nfunc X() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "head")

	_, err := CheckPinnedApproval(repo, base, "", base, "HEAD")
	if err == nil {
		t.Fatal("approved-sha=base deve falhar com mensagem clara")
	}
	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "usage: git range-diff") {
		t.Fatalf("não deve despejar usage do range-diff:\n%s", msg)
	}
	if !strings.Contains(msg, "coincide com a base") && !strings.Contains(msg, "vazio") {
		t.Fatalf("mensagem deve explicar intervalo vazio:\n%s", msg)
	}
}

func TestCheckPinnedApprovalMatch(t *testing.T) {
	repo, base, approved, run := pinnedFixture(t, "package a\n\nfunc OK() {}\n")
	_ = run
	pinned, err := CheckPinnedApproval(repo, base, "", approved, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if pinned == nil || !pinned.Checked || pinned.Match == nil || !*pinned.Match {
		t.Fatalf("head igual ao approved deveria match; got %#v", pinned)
	}
	if pinned.ApprovedSHA != approved || pinned.PatchID == "" || pinned.PatchID != pinned.HeadPatchID {
		t.Fatalf("pinned=%#v", pinned)
	}
	if pinned.ApprovedBase == "" || pinned.HeadBase == "" {
		t.Fatalf("bases devem ser expostas; pinned=%#v", pinned)
	}
	if pinned.RangeDiffSummary != "" {
		t.Fatalf("match não deve ter range-diff: %q", pinned.RangeDiffSummary)
	}
}

func TestCheckPinnedApprovalMismatch(t *testing.T) {
	repo, base, approved, run := pinnedFixture(t, "package a\n\nfunc OK() {}\n")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n\nfunc Changed() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "diverged")

	pinned, err := CheckPinnedApproval(repo, base, "", approved, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if pinned == nil || !pinned.Diverged() {
		t.Fatalf("head divergente deveria mismatch; got %#v", pinned)
	}
	if pinned.PatchID == pinned.HeadPatchID {
		t.Fatal("patch-ids deveriam diferir")
	}
	if pinned.RangeDiffSummary == "" {
		t.Fatal("mismatch deve incluir range-diff")
	}
}

func pinnedFixture(t *testing.T, approvedContent string) (repo, base, approved string, run func(...string)) {
	t.Helper()
	repo = t.TempDir()
	run = initGitRepo(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "base")
	base = gitRevParse(t, repo, "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte(approvedContent), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "approved")
	approved = gitRevParse(t, repo, "HEAD")
	return repo, base, approved, run
}

func TestRunPinnedApprovalInReportJSON(t *testing.T) {
	repo := t.TempDir()
	run := initGitRepo(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "base")
	base := gitRevParse(t, repo, "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\nx\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "approved")
	approved := gitRevParse(t, repo, "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\ny\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "head")

	ticket := writeTicket(t, repo, "a.go")
	cfg := DefaultConfig(repo)
	cfg.TicketPath = ticket
	cfg.BaseRef = base
	cfg.HeadRef = "HEAD"
	cfg.ApprovedSHA = approved
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	rep, err := Run(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.PinnedApproval == nil {
		t.Fatal("expected pinned_approval no report")
	}
	if !rep.PinnedApproval.Diverged() {
		t.Fatal("expected mismatch")
	}
	if rep.Drift == nil || rep.Drift.PinnedApproval == nil || !rep.Drift.PinnedApproval.Diverged() {
		t.Fatalf("drift deve espelhar pinned_approval; drift=%#v", rep.Drift)
	}
	if rep.PinnedApproval.ApprovedSHA != approved {
		t.Fatalf("approved_sha=%q want %q", rep.PinnedApproval.ApprovedSHA, approved)
	}
	if rep.PinnedApproval.RangeDiffSummary == "" {
		t.Fatal("expected range-diff summary")
	}
}

func TestCheckPinnedApprovalIdenticalContentDifferentCommit(t *testing.T) {
	repo := t.TempDir()
	run := initGitRepo(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "base")
	base := gitRevParse(t, repo, "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\nfunc X() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "approved")
	approved := gitRevParse(t, repo, "HEAD")

	run("checkout", "-b", "copy", base)
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\nfunc X() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "copy-same-patch")
	copySHA := gitRevParse(t, repo, "HEAD")

	pinned, err := CheckPinnedApproval(repo, base, "", approved, copySHA)
	if err != nil {
		t.Fatal(err)
	}
	if pinned == nil || pinned.Match == nil || !*pinned.Match {
		t.Fatalf("mesmo patch em commits distintos deve match; got %#v", pinned)
	}
}

func TestCheckPinnedApprovalCleanRebaseMatch(t *testing.T) {
	repo := t.TempDir()
	run := initGitRepo(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "base")
	m1 := gitRevParse(t, repo, "HEAD")

	run("checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(repo, "feat.go"), []byte("package feat\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "approved")
	approved := gitRevParse(t, repo, "HEAD")

	run("checkout", "master")
	if err := os.WriteFile(filepath.Join(repo, "other.txt"), []byte("main2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "main2")
	m2 := gitRevParse(t, repo, "HEAD")

	run("checkout", "feature")
	run("rebase", "master")
	head := gitRevParse(t, repo, "HEAD")

	pinned, err := CheckPinnedApproval(repo, m2, "", approved, head)
	if err != nil {
		t.Fatal(err)
	}
	if pinned == nil || pinned.Match == nil || !*pinned.Match {
		t.Fatalf("rebase limpo deve match; got %#v", pinned)
	}
	if pinned.ApprovedBase != m1 {
		t.Fatalf("approved_base=%s want %s", pinned.ApprovedBase, m1)
	}
	if pinned.HeadBase != m2 {
		t.Fatalf("head_base=%s want %s", pinned.HeadBase, m2)
	}
	if pinned.PatchID == "" || pinned.PatchID != pinned.HeadPatchID {
		t.Fatalf("patch-ids devem coincidir; pinned=%#v", pinned)
	}
}

func TestCheckPinnedApprovalRebasePatchChangeDiverges(t *testing.T) {
	repo := t.TempDir()
	run := initGitRepo(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "base")

	run("checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(repo, "feat.go"), []byte("package feat\n\nfunc A() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "approved")
	approved := gitRevParse(t, repo, "HEAD")

	run("checkout", "master")
	if err := os.WriteFile(filepath.Join(repo, "other.txt"), []byte("main2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "main2")
	m2 := gitRevParse(t, repo, "HEAD")

	run("checkout", "feature")
	run("rebase", "master")
	if err := os.WriteFile(filepath.Join(repo, "feat.go"), []byte("package feat\n\nfunc Changed() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "--amend", "--no-edit")
	head := gitRevParse(t, repo, "HEAD")

	pinned, err := CheckPinnedApproval(repo, m2, "", approved, head)
	if err != nil {
		t.Fatal(err)
	}
	if pinned == nil || !pinned.Diverged() {
		t.Fatalf("rebase com patch alterado deve divergir; got %#v", pinned)
	}
	if pinned.RangeDiffSummary == "" {
		t.Fatal("mismatch deve incluir range-diff")
	}
}

func TestCheckPinnedApprovalAmendMatch(t *testing.T) {
	repo, base, approved, run := pinnedFixture(t, "package a\n\nfunc OK() {}\n")
	run("commit", "--amend", "-m", "approved-amended")
	head := gitRevParse(t, repo, "HEAD")
	if head == approved {
		t.Fatal("amend deveria gerar SHA novo")
	}

	pinned, err := CheckPinnedApproval(repo, base, "", approved, head)
	if err != nil {
		t.Fatal(err)
	}
	if pinned == nil || pinned.Match == nil || !*pinned.Match {
		t.Fatalf("amend com mesmo patch deve match; got %#v", pinned)
	}
}

func TestCheckPinnedApprovalHeadAdvancesDiverges(t *testing.T) {
	repo, base, approved, run := pinnedFixture(t, "package a\n\nfunc OK() {}\n")
	if err := os.WriteFile(filepath.Join(repo, "extra.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "extra")

	pinned, err := CheckPinnedApproval(repo, base, "", approved, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if pinned == nil || !pinned.Diverged() {
		t.Fatalf("head que avança deve divergir; got %#v", pinned)
	}
}

func TestCheckPinnedApprovalExplicitApprovedBase(t *testing.T) {
	repo := t.TempDir()
	run := initGitRepo(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "base")
	m1 := gitRevParse(t, repo, "HEAD")

	run("checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(repo, "feat.go"), []byte("package feat\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "approved")
	approved := gitRevParse(t, repo, "HEAD")

	run("checkout", "master")
	if err := os.WriteFile(filepath.Join(repo, "other.txt"), []byte("main2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "main2")
	m2 := gitRevParse(t, repo, "HEAD")

	run("checkout", "feature")
	run("rebase", "master")
	head := gitRevParse(t, repo, "HEAD")

	pinned, err := CheckPinnedApproval(repo, m2, m1, approved, head)
	if err != nil {
		t.Fatal(err)
	}
	if pinned == nil || pinned.Match == nil || !*pinned.Match {
		t.Fatalf("--approved-base explícito em rebase limpo deve match; got %#v", pinned)
	}
	if pinned.ApprovedBase != m1 || pinned.HeadBase != m2 {
		t.Fatalf("bases=%s/%s want %s/%s", pinned.ApprovedBase, pinned.HeadBase, m1, m2)
	}
}

func TestRangeDiffAllEqual(t *testing.T) {
	if !rangeDiffAllEqual("1:  abcdef0 = 1:  1234567 subject") {
		t.Fatal("linha = deve ser equal")
	}
	if rangeDiffAllEqual("1:  abcdef0 ! 1:  1234567 subject") {
		t.Fatal("linha ! não deve ser equal")
	}
	if !rangeDiffAllEqual("") {
		t.Fatal("summary vazio deve ser equal")
	}
}

func TestCheckPinnedApprovalWhitespaceSensitive(t *testing.T) {
	repo := t.TempDir()
	run := initGitRepo(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n\nfunc OK() {\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "base")
	base := gitRevParse(t, repo, "HEAD")

	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n\nfunc OK() {\n\tx := 1\n\t_ = x\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "approved")
	approved := gitRevParse(t, repo, "HEAD")

	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n\nfunc OK() {\n    x := 1\n    _ = x\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "whitespace-only")

	pinned, err := CheckPinnedApproval(repo, base, "", approved, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if pinned == nil || !pinned.Checked {
		t.Fatalf("esperava checagem; got %#v", pinned)
	}
	if !pinned.Diverged() {
		t.Fatalf("mudança só de whitespace deve divergir com patch-id verbatim; got %#v", pinned)
	}
	if pinned.PatchID == pinned.HeadPatchID {
		t.Fatal("patch-ids verbatim deveriam diferir para tabs vs espaços")
	}
}

func TestCheckPinnedApprovalUncheckedExplicit(t *testing.T) {
	repo, base, _, _ := pinnedFixture(t, "package a\n\nfunc OK() {}\n")
	pinned, err := CheckPinnedApproval(repo, base, "", "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if pinned == nil {
		t.Fatal("sem approved-sha deve retornar pinned_approval explícito")
	}
	if pinned.Checked {
		t.Fatalf("checked deve ser false; got %#v", pinned)
	}
}

func TestRunRequireApprovedSHA(t *testing.T) {
	repo, base, _, _ := pinnedFixture(t, "package a\n\nfunc OK() {}\n")
	ticket := writeTicket(t, repo, "a.go")
	cfg := DefaultConfig(repo)
	cfg.TicketPath = ticket
	cfg.BaseRef = base
	cfg.HeadRef = "HEAD"
	cfg.RequireApprovedSHA = true
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	_, err := Run(&cfg)
	if err == nil {
		t.Fatal("require_approved_sha sem --approved-sha deve falhar")
	}
	if !strings.Contains(err.Error(), "approved-sha") {
		t.Fatalf("mensagem deve citar approved-sha: %v", err)
	}
}

func TestRunPinsHeadSHAOnce(t *testing.T) {
	repo, base, _, run := pinnedFixture(t, "package a\n\nfunc OK() {}\n")
	ticket := writeTicket(t, repo, "a.go")
	headBefore := gitRevParse(t, repo, "HEAD")

	cfg := DefaultConfig(repo)
	cfg.TicketPath = ticket
	cfg.BaseRef = base
	cfg.HeadRef = "HEAD"
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	rep, err := Run(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.HeadRef != headBefore {
		t.Fatalf("head_ref deve ser SHA pinado no início; got %q want %q", rep.HeadRef, headBefore)
	}
	if rep.BaseRef != base {
		t.Fatalf("base_ref deve ser SHA; got %q want %q", rep.BaseRef, base)
	}
	if rep.PinnedApproval == nil || rep.PinnedApproval.Checked {
		t.Fatalf("sem approved-sha: pinned_approval.checked=false; got %#v", rep.PinnedApproval)
	}

	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n\nfunc Moved() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "after-run-should-not-matter")
	if rep.HeadRef == gitRevParse(t, repo, "HEAD") {
		t.Fatal("relatório não deve refletir HEAD movido depois do pin")
	}
	_ = run
}

func TestDriftThreeDotIgnoresSquashOnBase(t *testing.T) {
	repo := t.TempDir()
	run := initGitRepo(t, repo)
	run("checkout", "-b", "main")
	mustWrite(t, filepath.Join(repo, "root.txt"), "root\n")
	run("add", ".")
	run("commit", "-m", "root")
	mustWrite(t, filepath.Join(repo, "base.txt"), "base\n")
	run("add", ".")
	run("commit", "-m", "base")
	fork := gitRevParse(t, repo, "HEAD")

	run("checkout", "-b", "feature")
	mustWrite(t, filepath.Join(repo, "pr.txt"), "pr\n")
	run("add", ".")
	run("commit", "-m", "pr change")

	run("checkout", "main")
	for _, n := range []string{"squash1.txt", "squash2.txt", "squash3.txt", "squash4.txt", "squash5.txt", "squash6.txt"} {
		mustWrite(t, filepath.Join(repo, n), "sq\n")
	}
	run("add", ".")
	run("commit", "-m", "squash other PR")
	mainTip := gitRevParse(t, repo, "HEAD")

	run("checkout", "feature")
	rd := "---\ntitle: pr\nbase_commit: " + fork + "\nscope:\n  - pr.txt\n  - rd.md\n---\n"
	mustWrite(t, filepath.Join(repo, "rd.md"), rd)
	run("add", ".")
	run("commit", "-m", "ticket")

	cfg := DefaultConfig(repo)
	cfg.TicketPath = filepath.Join(repo, "rd.md")
	cfg.BaseRef = "main"
	cfg.HeadRef = "HEAD"
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	rep, err := Run(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Drift == nil {
		t.Fatal("expected drift")
	}
	for _, f := range rep.Drift.OutOfScope {
		if strings.HasPrefix(f, "squash") {
			t.Fatalf("three-dot com --base main não deve incluir arquivos do squash: %v", rep.Drift.OutOfScope)
		}
	}
	if rep.Drift.ActualBase != mainTip {
		t.Fatalf("actual_base=%q want main tip %q", rep.Drift.ActualBase, mainTip)
	}
	if rep.Drift.MergeBase == "" {
		t.Fatal("merge_base deve ser preenchido")
	}
}

func TestDriftThreeDotAfterMergeMain(t *testing.T) {
	repo := t.TempDir()
	run := initGitRepo(t, repo)
	run("checkout", "-b", "main")
	mustWrite(t, filepath.Join(repo, "root.txt"), "root\n")
	run("add", ".")
	run("commit", "-m", "root")
	mustWrite(t, filepath.Join(repo, "base.txt"), "base\n")
	run("add", ".")
	run("commit", "-m", "base")
	fork := gitRevParse(t, repo, "HEAD")

	run("checkout", "-b", "feature")
	mustWrite(t, filepath.Join(repo, "pr.txt"), "pr\n")
	run("add", ".")
	run("commit", "-m", "pr")

	run("checkout", "main")
	mustWrite(t, filepath.Join(repo, "from_main.txt"), "m\n")
	run("add", ".")
	run("commit", "-m", "main advance")

	run("checkout", "feature")
	run("merge", "main", "-m", "merge main")
	rd := "---\ntitle: pr\nbase_commit: " + fork + "\nscope:\n  - pr.txt\n  - rd.md\n---\n"
	mustWrite(t, filepath.Join(repo, "rd.md"), rd)
	run("add", ".")
	run("commit", "-m", "ticket")

	cfg := DefaultConfig(repo)
	cfg.TicketPath = filepath.Join(repo, "rd.md")
	cfg.BaseRef = "main"
	cfg.HeadRef = "HEAD"
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	rep, err := Run(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Drift == nil {
		t.Fatal("expected drift")
	}
	for _, f := range rep.Drift.OutOfScope {
		if f == "from_main.txt" {
			t.Fatalf("após merge da main, --base main three-dot não deve marcar from_main.txt: %v", rep.Drift.OutOfScope)
		}
	}
	if rep.Drift.MergeBase == "" {
		t.Fatal("merge_base vazio")
	}
}

func TestDriftBaseMismatchNonAncestor(t *testing.T) {
	repo := t.TempDir()
	run := initGitRepo(t, repo)
	run("checkout", "-b", "main")
	mustWrite(t, filepath.Join(repo, "a.txt"), "a\n")
	run("add", ".")
	run("commit", "-m", "a")
	mustWrite(t, filepath.Join(repo, "b.txt"), "b\n")
	run("add", ".")
	run("commit", "-m", "b")

	run("checkout", "-b", "feature")
	mustWrite(t, filepath.Join(repo, "pr.txt"), "pr\n")
	run("add", ".")
	run("commit", "-m", "pr")

	run("checkout", "main")
	run("checkout", "-b", "side")
	mustWrite(t, filepath.Join(repo, "side.txt"), "s\n")
	run("add", ".")
	run("commit", "-m", "side")
	side := gitRevParse(t, repo, "HEAD")

	run("checkout", "feature")
	rd := "---\ntitle: t\nbase_commit: " + side + "\nscope:\n  - pr.txt\n---\n"
	mustWrite(t, filepath.Join(repo, "rd.md"), rd)

	cfg := DefaultConfig(repo)
	cfg.TicketPath = filepath.Join(repo, "rd.md")
	cfg.BaseRef = side
	cfg.HeadRef = "HEAD"
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	rep, err := Run(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Drift == nil || !rep.Drift.BaseMismatch {
		t.Fatalf("base não ancestral deve setar base_mismatch; drift=%#v", rep.Drift)
	}
	if rep.Drift.ActualBase != side {
		t.Fatalf("actual_base=%q want %q", rep.Drift.ActualBase, side)
	}
	if rep.Drift.MergeBase == "" {
		t.Fatal("merge_base deve existir mesmo sem ancestralidade direta")
	}
}

func TestDriftExplicitBaseUsedForDiff(t *testing.T) {
	repo := t.TempDir()
	run := initGitRepo(t, repo)
	mustWrite(t, filepath.Join(repo, "a.txt"), "a\n")
	run("add", ".")
	run("commit", "-m", "a")
	c1 := gitRevParse(t, repo, "HEAD")
	mustWrite(t, filepath.Join(repo, "b.txt"), "b\n")
	run("add", ".")
	run("commit", "-m", "b")
	c2 := gitRevParse(t, repo, "HEAD")
	mustWrite(t, filepath.Join(repo, "c.txt"), "c\n")
	run("add", ".")
	run("commit", "-m", "c")

	rd := "---\ntitle: t\nbase_commit: " + c1 + "\nscope:\n  - c.txt\n  - rd.md\n---\n"
	mustWrite(t, filepath.Join(repo, "rd.md"), rd)

	cfg := DefaultConfig(repo)
	cfg.TicketPath = filepath.Join(repo, "rd.md")
	cfg.BaseRef = c2
	cfg.HeadRef = "HEAD"
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	rep, err := Run(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Drift == nil {
		t.Fatal("expected drift")
	}
	if rep.Drift.ActualBase != c2 {
		t.Fatalf("actual_base=%q want explicit base %q", rep.Drift.ActualBase, c2)
	}
	for _, f := range rep.Drift.OutOfScope {
		if f == "b.txt" {
			t.Fatalf("--base explícito (c2) não deve incluir b.txt no diff: oos=%v", rep.Drift.OutOfScope)
		}
	}
	if rep.Drift.BaseMismatch {
		t.Fatal("base explícita ancestral não deve marcar base_mismatch só porque difere do ticket")
	}
	if rep.Drift.MergeBase == "" {
		t.Fatal("merge_base vazio")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
