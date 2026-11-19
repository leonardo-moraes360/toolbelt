package egm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func (f *flow) statusAt(ctx context.Context, rev string) *Report {
	f.t.Helper()
	o := f.opts()
	o.At = rev
	return f.must(Status(ctx, &o)).Report
}

// convergedAtHead converges the flow spec (without the missing ref) in one
// clean round and commits it.
func convergedAtHead(t *testing.T) (*flow, context.Context) {
	t.Helper()
	f := newFlow(t, "  clean_rounds: 1\n")
	f.edit("`internal/missing.go`", "`internal/kept.go` too")
	gitT(t, f.root, "commit", "-qam", "fix ref")
	ctx := context.Background()
	if r := f.must(Review(ctx, f.optsP())).Report; r.Verdict != VerdictPassPartial {
		t.Fatalf("round 1: %s %+v", r.Verdict, r.Findings)
	}
	return f, ctx
}

// TestGateStatusAt is R7a/R25: the gate content after the commit equals the
// interactive content, a worktree edit does not move the gate, and an older
// commit or a source changed at head is STALE.
func TestGateStatusAt(t *testing.T) {
	f, ctx := convergedAtHead(t)
	r := f.statusAt(ctx, "HEAD")
	if r.Verdict != VerdictPassPartial || r.Target != "HEAD" {
		t.Fatalf("at HEAD: %s %s", r.Verdict, r.Target)
	}
	wt := f.must(Status(ctx, f.optsP())).Report
	if wt.Content != r.Content || wt.Target != "worktree" {
		t.Fatalf("content worktree %s vs gate %s", wt.Content, r.Content)
	}
	f.edit("The problem.", "The problem, not committed.")
	if r := f.statusAt(ctx, "HEAD"); r.Verdict != VerdictPassPartial {
		t.Fatalf("worktree edit moved the gate: %s", r.Verdict)
	}
	if r := f.must(Status(ctx, f.optsP())).Report; r.Verdict != VerdictStale {
		t.Fatalf("worktree: %s", r.Verdict)
	}
	if r := f.statusAt(ctx, "HEAD~1"); r.Verdict != VerdictStale {
		t.Fatalf("at HEAD~1: %s", r.Verdict)
	}
	gitT(t, f.root, "checkout", "-q", "--", f.doc)
	writeFile(t, filepath.Join(f.root, "internal", "kept.go"), "package internal\n\nvar X = 1\n")
	gitT(t, f.root, "commit", "-qam", "source")
	if r := f.statusAt(ctx, "HEAD"); r.Verdict != VerdictStale {
		t.Fatalf("source changed at head: %s", r.Verdict)
	}
}

// TestGateDocOnlyInCommit: the document may be gone from the worktree.
func TestGateDocOnlyInCommit(t *testing.T) {
	f, ctx := convergedAtHead(t)
	if err := os.Remove(filepath.Join(f.root, f.doc)); err != nil {
		t.Fatal(err)
	}
	if r := f.statusAt(ctx, "HEAD"); r.Verdict != VerdictPassPartial {
		t.Fatalf("at HEAD: %s", r.Verdict)
	}
	o := f.opts()
	o.At = "HEAD~2"
	_, err := Status(ctx, &o)
	wantReason(t, err, "usage")
	o.At = "no-such-rev"
	_, err = Status(ctx, &o)
	wantReason(t, err, "bad_revision")
}

// TestGateTreeRules: sources behind a versioned symlink are excluded, a
// symlinked document is refused and the repository config comes from the
// commit's blob (R7a, R34).
func TestGateTreeRules(t *testing.T) {
	f := newFlow(t, "")
	ctx := context.Background()
	if err := os.Symlink("internal", filepath.Join(f.root, "lnk")); err != nil {
		t.Fatal(err)
	}
	f.edit("`internal/missing.go`", "[k](../../lnk/kept.go)")
	writeFile(t, filepath.Join(f.root, ".toolbelt", "config.yaml"), "egm:\n  agent_cmd: [/bin/sh]\n")
	gitT(t, f.root, "add", ".")
	gitT(t, f.root, "commit", "-qm", "symlink")
	if err := os.RemoveAll(filepath.Join(f.root, ".toolbelt")); err != nil {
		t.Fatal(err)
	}
	r := f.statusAt(ctx, "HEAD")
	skipped := map[string]string{}
	for _, s := range r.SkippedSources {
		skipped[s.Path] = s.Reason
	}
	if skipped["lnk/kept.go"] != "missing" {
		t.Fatalf("skipped %+v", r.SkippedSources)
	}
	untrusted := false
	for _, w := range r.Warnings {
		untrusted = untrusted || w.Code == "config_untrusted"
	}
	if !untrusted {
		t.Fatalf("config at head not read: %+v", r.Warnings)
	}
	if err := os.Symlink("x.md", filepath.Join(f.root, "docs", "specs", "y.md")); err != nil {
		t.Fatal(err)
	}
	gitT(t, f.root, "add", ".")
	gitT(t, f.root, "commit", "-qm", "doc link")
	o := f.opts()
	o.Doc, o.At = "docs/specs/y.md", "HEAD"
	_, err := Status(ctx, &o)
	wantReason(t, err, "usage")
}

func (f *flow) gate(ctx context.Context, base string, strict bool, docs ...string) *GateResult {
	f.t.Helper()
	r, err := Gate(ctx, &GateOptions{RepoDir: f.root, Base: base, Head: "HEAD", Docs: docs, Strict: strict, ConfigFile: f.cfg})
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func hasGateWarning(r *GateResult, code string) bool {
	for _, w := range r.Warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}

// TestGateSelection is R33: the touched gated documents of the range are
// evaluated at head; PASS_PARTIAL warns, and fails with strict.
func TestGateSelection(t *testing.T) {
	f, ctx := convergedAtHead(t)
	r := f.gate(ctx, "HEAD~2", false)
	if !r.OK || !r.Warn || len(r.Docs) != 1 || r.Docs[0].Doc != f.doc || r.Docs[0].Verdict != VerdictPassPartial {
		t.Fatalf("gate: %+v", r)
	}
	if r := f.gate(ctx, "HEAD~2", true); r.OK {
		t.Fatalf("strict: %+v", r)
	}
	// A worktree edit after the commit does not change the gate's view.
	f.edit("The problem.", "The problem, not committed.")
	if r := f.gate(ctx, "HEAD~2", false); !r.OK {
		t.Fatalf("worktree edit moved the gate: %+v", r)
	}
	gitT(t, f.root, "commit", "-qam", "edit")
	r = f.gate(ctx, "HEAD~1", false)
	if r.OK || r.Docs[0].Verdict != VerdictStale {
		t.Fatalf("after commit: %+v", r)
	}
}

// TestGateEmptySelection: no gated document in the range warns, fails with
// strict, and fails when the worktree has uncommitted gated documents.
func TestGateEmptySelection(t *testing.T) {
	f, ctx := convergedAtHead(t)
	writeFile(t, filepath.Join(f.root, "README.md"), "# r\n")
	gitT(t, f.root, "add", ".")
	gitT(t, f.root, "commit", "-qm", "readme")
	r := f.gate(ctx, "HEAD~1", false)
	if !r.OK || !r.Warn || len(r.Docs) != 0 || !hasGateWarning(r, "no_gated_docs") {
		t.Fatalf("empty: %+v", r)
	}
	if r := f.gate(ctx, "HEAD~1", true); r.OK {
		t.Fatalf("strict empty: %+v", r)
	}
	writeFile(t, filepath.Join(f.root, "docs", "adr", "001-new.md"), "# ADR\n")
	if r := f.gate(ctx, "HEAD~1", false); r.OK || !hasGateWarning(r, "uncommitted_gated_docs") {
		t.Fatalf("unversioned: %+v", r)
	}
	if err := os.Remove(filepath.Join(f.root, "docs", "adr", "001-new.md")); err != nil {
		t.Fatal(err)
	}
	f.edit("The problem.", "The problem, modified.")
	if r := f.gate(ctx, "HEAD~1", false); r.OK || !hasGateWarning(r, "uncommitted_gated_docs") {
		t.Fatalf("modified: %+v", r)
	}
	// --egm-doc adds a document outside the range.
	gitT(t, f.root, "checkout", "-q", "--", f.doc)
	if r := f.gate(ctx, "HEAD~1", false, f.doc); len(r.Docs) != 1 || r.Docs[0].Verdict != VerdictPassPartial {
		t.Fatalf("--egm-doc: %+v", r)
	}
}

// TestGateSourcesChanged: a document with state whose source changed in the
// range, but that was not touched, only warns.
func TestGateSourcesChanged(t *testing.T) {
	f, ctx := convergedAtHead(t)
	writeFile(t, filepath.Join(f.root, "internal", "kept.go"), "package internal\n\nvar Y = 2\n")
	gitT(t, f.root, "commit", "-qam", "source")
	r := f.gate(ctx, "HEAD~1", false)
	if len(r.Docs) != 0 || !hasGateWarning(r, "sources_changed") {
		t.Fatalf("%+v", r)
	}
}
