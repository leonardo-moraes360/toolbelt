//go:build unix

package egm

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (f *flow) bootstrap(target string, from ...string) (*BootstrapResult, error) {
	return Bootstrap(context.Background(), &BootstrapOptions{Cwd: f.root, Target: target, From: from, ConfigFile: f.cfg})
}

func reason(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Reason
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

// AC12, AC16: the skeleton has the title, the status line, the required
// sections and linked sources that R7 includes exactly; nothing is staged.
func TestBootstrapSkeleton(t *testing.T) {
	f := newFlow(t, "")
	res, err := f.bootstrap("docs/specs/router-bus.md", "internal/")
	if err != nil {
		t.Fatal(err)
	}
	if res.Type != "spec" || len(res.Sources) != 1 || res.Sources[0] != "internal/kept.go" {
		t.Fatalf("result: %+v", res)
	}
	b, err := os.ReadFile(filepath.Join(f.root, "docs", "specs", "router-bus.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{"# Router bus\n", BootstrapStatus, "## Overview", "## Problem", "## Goals", "## Non-Goals",
		"## Technical Plan", "## Sources\n\n- [internal/kept.go](../../internal/kept.go)\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	ctx := context.Background()
	if d := driftOfDoc(f.drift(ctx, ""), "docs/specs/router-bus.md"); d != nil {
		t.Fatalf("an untracked draft is listed: %+v", d)
	}
	if out := gitT(t, f.root, "status", "--porcelain", "--untracked-files=all"); !strings.Contains(out, "?? docs/specs/router-bus.md") {
		t.Fatalf("draft staged or committed: %q", out)
	}
	if err := os.MkdirAll(filepath.Join(f.root, "docs", "adr"), 0o755); err != nil {
		t.Fatal(err)
	}
	adr, err := f.bootstrap("docs/adr/020-x.md", "internal/kept.go")
	if err != nil || adr.Type != "adr" {
		t.Fatalf("adr: %+v %v", adr, err)
	}
	if b, _ := os.ReadFile(filepath.Join(f.root, "docs", "adr", "020-x.md")); !strings.Contains(string(b), "## Alternatives Considered") {
		t.Fatalf("adr sections:\n%s", b)
	}
}

// AC13: refusals write nothing.
func TestBootstrapRefusals(t *testing.T) {
	f := newFlow(t, "  doc_globs: [\"docs/**\", \"**/*.md\"]\n")
	before, err := os.ReadFile(filepath.Join(f.root, "docs", "specs", "x.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.root, "docs", "specs"), filepath.Join(f.root, "docs", "linked")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "x.go")
	writeFile(t, outside, "package x\n")
	for name, c := range map[string]struct {
		target string
		from   []string
		want   string
	}{
		"exists":       {"docs/specs/x.md", []string{"internal/kept.go"}, "bootstrap_exists"},
		"not gated":    {"notes/x.md", []string{"internal/kept.go"}, "usage"},
		"symlink path": {"docs/linked/y.md", []string{"internal/kept.go"}, "bootstrap_path"},
		"inside .git":  {".git/x.md", []string{"internal/kept.go"}, "bootstrap_path"},
		"from root":    {"docs/specs/r.md", []string{"."}, "usage"},
		"from outside": {"docs/specs/z.md", []string{outside}, "usage"},
		"no from":      {"docs/specs/w.md", nil, "usage"},
	} {
		_, err := f.bootstrap(c.target, c.from...)
		if err == nil || (c.want != "" && reason(err) != c.want) {
			t.Errorf("%s: %v (want %s)", name, err, c.want)
		}
	}
	for _, p := range []string{"docs/linked/y.md", "docs/specs/z.md", "docs/specs/w.md"} {
		if _, err := os.Lstat(filepath.Join(f.root, filepath.FromSlash(p))); err == nil && p != "docs/linked/y.md" {
			t.Errorf("%s written", p)
		}
	}
	if _, err := os.Lstat(filepath.Join(f.root, "docs", "specs", "y.md")); err == nil {
		t.Error("written through the symlinked directory")
	}
	if _, err := os.Lstat(filepath.Join(f.root, ".git", "x.md")); err == nil {
		t.Error("written inside .git")
	}
	if after, _ := os.ReadFile(filepath.Join(f.root, "docs", "specs", "x.md")); !bytes.Equal(after, before) {
		t.Error("an existing document changed")
	}
}

// AC14, AC15: skipped files carry a reason; a file R7 would exclude refuses.
func TestBootstrapFromFiltering(t *testing.T) {
	f := newFlow(t, "")
	dir := filepath.Join(f.root, "internal")
	writeFile(t, filepath.Join(dir, "testdata", "fixture.go"), "package x\n")
	writeFile(t, filepath.Join(dir, ".env.local"), "A=1\n")
	gitT(t, f.root, "add", "internal")
	gitT(t, f.root, "commit", "-qm", "more files")
	writeFile(t, filepath.Join(dir, "p(1).go"), "package internal\n")
	gitT(t, f.root, "add", "internal/p(1).go")
	gitT(t, f.root, "commit", "-qm", "unlinkable name")
	target := t.TempDir()
	writeFile(t, filepath.Join(target, "inside.go"), "package x\n")
	if err := os.Symlink(target, filepath.Join(dir, "linked")); err != nil {
		t.Fatal(err)
	}
	gitT(t, f.root, "add", "internal/linked")
	gitT(t, f.root, "commit", "-qm", "symlinked dir")
	writeFile(t, filepath.Join(dir, "untracked.go"), "package internal\n")
	res, err := f.bootstrap("docs/specs/filtered.md", "internal/", "internal/untracked.go")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range res.Skipped {
		got[s.Path] = s.Reason
	}
	if got["internal/testdata/fixture.go"] != "excluded" || got["internal/.env.local"] != "excluded" || got["internal/untracked.go"] != "untracked" ||
		got["internal/p(1).go"] != "unlinkable" || got["internal/linked"] != "symlink" {
		t.Fatalf("skipped: %+v", res.Skipped)
	}
	if len(res.Sources) != 1 || res.Sources[0] != "internal/kept.go" {
		t.Fatalf("sources: %+v", res.Sources)
	}
	if src := r7Sources(t, f, "docs/specs/filtered.md"); len(src) != 1 || src[0] != "internal/kept.go" {
		t.Fatalf("R7 of the draft: %v", src)
	}
}

// r7Sources is AC12's check: the review's R7 sources of the written draft.
func r7Sources(t *testing.T, f *flow, doc string) []string {
	t.Helper()
	repo, err := DiscoverRepo(filepath.Join(f.root, filepath.FromSlash(doc)))
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(doc)))
	if err != nil {
		t.Fatal(err)
	}
	user, err := LoadUserConfig(f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	set := ApplyRepoConfig(&user, nil)
	got, err := CollectSources(context.Background(), repo, &set, doc, string(text))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, s := range got.Included {
		out = append(out, s.Path)
	}
	return out
}

// AC15: a file R7 would exclude (secret-like content) refuses the bootstrap.
func TestBootstrapSecretLike(t *testing.T) {
	f := newFlow(t, "")
	dir := filepath.Join(f.root, "internal")
	writeFile(t, filepath.Join(dir, "leak.go"), "package internal\n\nconst k = \"ghp_"+strings.Repeat("a", 36)+"\"\n")
	gitT(t, f.root, "add", "internal/leak.go")
	gitT(t, f.root, "commit", "-qm", "leak")
	if _, err := f.bootstrap("docs/specs/leaky.md", "internal/leak.go"); reason(err) != "bootstrap_sources" {
		t.Fatalf("secret-like source: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(f.root, "docs", "specs", "leaky.md")); err == nil {
		t.Fatal("refused bootstrap wrote the file")
	}
}
