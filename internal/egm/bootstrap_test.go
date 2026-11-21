//go:build unix

package egm

import (
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
	f := newFlow(t, "")
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
		"symlink path": {"docs/linked/y.md", []string{"internal/kept.go"}, ""},
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
}

// AC14, AC15: skipped files carry a reason; a file R7 would exclude refuses.
func TestBootstrapFromFiltering(t *testing.T) {
	f := newFlow(t, "")
	dir := filepath.Join(f.root, "internal")
	writeFile(t, filepath.Join(dir, "testdata", "fixture.go"), "package x\n")
	writeFile(t, filepath.Join(dir, ".env.local"), "A=1\n")
	gitT(t, f.root, "add", "internal")
	gitT(t, f.root, "commit", "-qm", "more files")
	writeFile(t, filepath.Join(dir, "untracked.go"), "package internal\n")
	res, err := f.bootstrap("docs/specs/filtered.md", "internal/", "internal/untracked.go")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range res.Skipped {
		got[s.Path] = s.Reason
	}
	if got["internal/testdata/fixture.go"] != "excluded" || got["internal/.env.local"] != "excluded" || got["internal/untracked.go"] != "untracked" {
		t.Fatalf("skipped: %+v", res.Skipped)
	}
	if len(res.Sources) != 1 || res.Sources[0] != "internal/kept.go" {
		t.Fatalf("sources: %+v", res.Sources)
	}
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
