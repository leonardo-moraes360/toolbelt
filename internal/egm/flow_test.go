package egm

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const flowSpec = `# Spec: x

## Overview

Overview text that is long enough to read.

## Problem

The problem.

## Goals

- G1

## Non-Goals

- none

## Technical Plan

It changes ` + "`internal/missing.go`" + ` and keeps ` + "`internal/kept.go`" + `.
`

type flow struct {
	t    *testing.T
	root string
	doc  string
	cfg  string
}

// newFlow builds a committed repo with docs/specs/x.md and internal/kept.go,
// an isolated state dir and a user config file.
func newFlow(t *testing.T, userCfg string) *flow {
	t.Helper()
	root := newRepo(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeFile(t, filepath.Join(root, "docs", "specs", "x.md"), flowSpec)
	writeFile(t, filepath.Join(root, "internal", "kept.go"), "package internal\n")
	gitT(t, root, "add", ".")
	gitT(t, root, "commit", "-qm", "spec")
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfg, "egm:\n"+userCfg)
	return &flow{t: t, root: root, doc: "docs/specs/x.md", cfg: cfg}
}

func (f *flow) opts() Options {
	return Options{Doc: f.doc, Cwd: f.root, ConfigFile: f.cfg}
}

func (f *flow) optsP() *Options {
	o := f.opts()
	return &o
}

func (f *flow) edit(old, repl string) {
	f.t.Helper()
	p := filepath.Join(f.root, f.doc)
	b, err := os.ReadFile(p)
	if err != nil {
		f.t.Fatal(err)
	}
	s := strings.Replace(string(b), old, repl, 1)
	if s == string(b) {
		f.t.Fatalf("edit: %q not found", old)
	}
	writeFile(f.t, p, s)
}

func (f *flow) must(out *Outcome, err error) *Outcome {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
	return out
}

func lineageStatus(r *Report, id string) string {
	for i := range r.Findings {
		if r.Findings[i].ID == id {
			return r.Findings[i].Status
		}
	}
	return ""
}

// TestFlowDeterministic covers CUJ-2 and CUJ-3 offline: a checker finding,
// the no-change refusal, --fix with and without an edit, two clean rounds
// and PASS_PARTIAL, then STALE after an edit.
func TestFlowDeterministic(t *testing.T) {
	f := newFlow(t, "")
	ctx := context.Background()
	out := f.must(Review(ctx, f.optsP()))
	r := out.Report
	if r.Verdict != VerdictInProgress || r.Engine != EngineDeterministic || len(r.Findings) != 1 || r.Findings[0].Owner != checkRefs {
		t.Fatalf("round 1: %+v", r)
	}
	if !strings.Contains(r.EngineReason, "agent_cmd") || !r.Coverage.Partial {
		t.Fatalf("engine reason / coverage: %q %+v", r.EngineReason, r.Coverage)
	}
	_, err := LoopNext(ctx, f.optsP())
	wantReason(t, err, "no_change")
	_, err = Review(ctx, f.optsP())
	wantReason(t, err, "state_exists")
	_, err = Resolve(ctx, &ResolveOptions{Options: f.opts(), Fix: []string{"F-1"}})
	wantReason(t, err, "invalid_transition")

	f.edit("`internal/missing.go`", "`internal/kept.go` again")
	out = f.must(Resolve(ctx, &ResolveOptions{Options: f.opts(), Fix: []string{"F-1"}}))
	if lineageStatus(out.Report, "F-1") != StFixed {
		t.Fatalf("after --fix: %+v", out.Report.Findings)
	}
	r2 := f.must(LoopNext(ctx, f.optsP())).Report
	if r2.Findings[0].Status != StFixed || !r2.Findings[0].Confirmed || r2.Verdict != VerdictInProgress {
		t.Fatalf("round 2: %+v", r2)
	}
	r3 := f.must(LoopNext(ctx, f.optsP())).Report
	if r3.Verdict != VerdictPassPartial || r3.LoopState != "converged" {
		t.Fatalf("round 3: %s %s %v", r3.Verdict, r3.LoopState, r3.Downgrades)
	}
	if !containsStr(r3.Downgrades, "no_detailed_section") {
		t.Fatalf("downgrades %v", r3.Downgrades)
	}
	f.checkStatusAfterConvergence(ctx)
}

func (f *flow) checkStatusAfterConvergence(ctx context.Context) {
	t := f.t
	t.Helper()
	st := f.must(Status(ctx, f.optsP()))
	if st.Report.Verdict != VerdictPassPartial || st.ExitCode != ExitOK {
		t.Fatalf("status: %s %d", st.Report.Verdict, st.ExitCode)
	}
	o := f.opts()
	o.Strict = true
	if st := f.must(Status(ctx, &o)); st.ExitCode != ExitFail {
		t.Fatalf("strict exit %d", st.ExitCode)
	}
	f.edit("The problem.", "The problem, edited.")
	if st := f.must(Status(ctx, f.optsP())); st.Report.Verdict != VerdictStale || st.ExitCode != ExitNotPassing {
		t.Fatalf("after edit: %s %d", st.Report.Verdict, st.ExitCode)
	}
}

func TestFlowNoState(t *testing.T) {
	f := newFlow(t, "")
	out := f.must(Status(context.Background(), f.optsP()))
	if out.Report.Verdict != VerdictNoState || out.ExitCode != ExitNotPassing {
		t.Fatalf("%s %d", out.Report.Verdict, out.ExitCode)
	}
	_, err := LoopNext(context.Background(), f.optsP())
	wantReason(t, err, "no_state")
}

// TestFlowReportDeterministic: the same inputs give the same report.json
// byte for byte (NFR determinism).
func TestFlowReportDeterministic(t *testing.T) {
	f := newFlow(t, "")
	ctx := context.Background()
	a := f.must(Review(ctx, f.optsP())).Report.JSON()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	b := f.must(Review(ctx, f.optsP())).Report.JSON()
	if !bytes.Equal(a, b) {
		t.Fatalf("reports differ:\n%s\n---\n%s", a, b)
	}
}

func TestFlowResolveRules(t *testing.T) {
	f := newFlow(t, "")
	ctx := context.Background()
	f.must(Review(ctx, f.optsP()))
	_, err := Resolve(ctx, &ResolveOptions{Options: f.opts(), Contest: "F-1", Evidence: "docs/specs/x.md:1"})
	wantReason(t, err, "contest_refused")
	_, err = Resolve(ctx, &ResolveOptions{Options: f.opts(), ForceSettle: "F-1"})
	wantReason(t, err, "usage")
	out := f.must(Resolve(ctx, &ResolveOptions{Options: f.opts(), Decline: []string{"F-1"}}))
	if lineageStatus(out.Report, "F-1") != StDeclined || out.Report.Findings[0].Downgrade != "declined_major" {
		t.Fatalf("decline: %+v", out.Report.Findings[0])
	}
	// No-op on the current status.
	f.must(Resolve(ctx, &ResolveOptions{Options: f.opts(), Decline: []string{"F-1"}}))
	_, err = Resolve(ctx, &ResolveOptions{Options: f.opts(), ForceSettle: "F-1", Reason: "x"})
	wantReason(t, err, "invalid_transition")
	_, err = Resolve(ctx, &ResolveOptions{Options: f.opts(), Fix: []string{"F-9"}})
	wantReason(t, err, "invalid_transition")
	// A declined major blocks nothing: two clean rounds converge, partially.
	f.edit("The problem.", "The problem, edited.")
	f.must(LoopNext(ctx, f.optsP()))
	r := f.must(LoopNext(ctx, f.optsP())).Report
	if r.Verdict != VerdictPassPartial || !containsStr(r.Downgrades, "F-1:declined_major") {
		t.Fatalf("%s %v", r.Verdict, r.Downgrades)
	}
}

func TestFlowForceSettleBlocksPass(t *testing.T) {
	f := newFlow(t, "")
	ctx := context.Background()
	f.must(Review(ctx, f.optsP()))
	out := f.must(Resolve(ctx, &ResolveOptions{Options: f.opts(), ForceSettle: "F-1", Reason: "accepted"}))
	if out.Report.Findings[0].Status != StSettled || out.Report.Findings[0].Downgrade != "forced" {
		t.Fatalf("%+v", out.Report.Findings[0])
	}
}

// TestFlowCapAndRestart: a cycle that hits max_rounds without converging is
// capped (FAIL); --restart needs a new content.
func TestFlowCapAndRestart(t *testing.T) {
	f := newFlow(t, "  max_rounds: 2\n")
	ctx := context.Background()
	f.must(Review(ctx, f.optsP()))
	f.edit("The problem.", "The problem, v2.")
	out := f.must(LoopNext(ctx, f.optsP()))
	if out.Report.Verdict != VerdictFail || out.ExitCode != ExitFail || out.Report.LoopState != "capped" {
		t.Fatalf("round 2: %s %d %s", out.Report.Verdict, out.ExitCode, out.Report.LoopState)
	}
	if out := f.must(LoopNext(ctx, f.optsP())); out.ExitCode != ExitFail {
		t.Fatalf("loop next on a capped cycle: %d", out.ExitCode)
	}
	o := f.opts()
	o.Restart = true
	_, err := Review(ctx, &o)
	wantReason(t, err, "restart_needs_change")
	f.edit("The problem, v2.", "The problem, v3.")
	out = f.must(Review(ctx, &o))
	if out.Report.Cycle != 2 || out.Report.Verdict != VerdictInProgress {
		t.Fatalf("restart: cycle %d %s", out.Report.Cycle, out.Report.Verdict)
	}
}

// TestFlowCheckerReopens: a confirmed fixed checker lineage reopens when the
// re-run reproduces it (R20).
func TestFlowCheckerReopens(t *testing.T) {
	f := newFlow(t, "")
	ctx := context.Background()
	f.must(Review(ctx, f.optsP()))
	f.edit("`internal/missing.go`", "nothing")
	f.must(LoopNext(ctx, f.optsP()))
	f.edit("nothing", "`internal/missing.go`")
	r := f.must(LoopNext(ctx, f.optsP())).Report
	if lineageStatus(r, "F-1") != StReopened || len(r.Findings) != 1 {
		t.Fatalf("%+v", r.Findings)
	}
}

// TestFlowUntrustedRepoConfig is CUJ-5: execution keys in the repository's
// config are ignored and reported.
func TestFlowUntrustedRepoConfig(t *testing.T) {
	f := newFlow(t, "")
	writeFile(t, filepath.Join(f.root, ".toolbelt", "config.yaml"), "egm:\n  agent_cmd: [/bin/sh, -c, 'touch /tmp/pwned']\n  env: {pass: [AWS_SECRET_ACCESS_KEY]}\n")
	r := f.must(Review(context.Background(), f.optsP())).Report
	if r.Engine != EngineDeterministic || !strings.Contains(r.EngineReason, "agent_cmd") {
		t.Fatalf("engine %s %q", r.Engine, r.EngineReason)
	}
	n := 0
	for _, w := range r.Warnings {
		if w.Code == "config_untrusted" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("warnings %+v", r.Warnings)
	}
}

// TestFlowUnstagedSource: an unversioned cited source makes the round not
// clean (R7).
func TestFlowUnstagedSource(t *testing.T) {
	f := newFlow(t, "  clean_rounds: 1\n")
	f.edit("`internal/missing.go`", "[new](../../internal/new.go)")
	gitT(t, f.root, "commit", "-qam", "link")
	writeFile(t, filepath.Join(f.root, "internal", "new.go"), "package internal\n")
	r := f.must(Review(context.Background(), f.optsP())).Report
	found := false
	for _, w := range r.Warnings {
		found = found || (w.Code == "unstaged_source" && w.Path == "internal/new.go")
	}
	if !found || r.Verdict == VerdictPassPartial {
		t.Fatalf("%s %+v", r.Verdict, r.Warnings)
	}
	gitT(t, f.root, "add", "internal/new.go")
	r = f.must(LoopNext(context.Background(), f.optsP())).Report
	if r.Verdict != VerdictPassPartial {
		t.Fatalf("after git add: %s %+v", r.Verdict, r.Warnings)
	}
}

func TestFlowEngineFlag(t *testing.T) {
	f := newFlow(t, "")
	o := f.opts()
	o.Engine = EngineLLM
	_, err := Review(context.Background(), &o)
	wantReason(t, err, "usage")
	o.Engine = "x"
	_, err = Review(context.Background(), &o)
	wantReason(t, err, "usage")
	o.Engine = ""
	o.Reviewers = []string{"nope"}
	_, err = Review(context.Background(), &o)
	wantReason(t, err, "usage")
}

// TestFlowFIFODocument: a named pipe as the document exits 2 instead of
// blocking in open(2) (review finding 1).
func TestFlowFIFODocument(t *testing.T) {
	f := newFlow(t, "")
	fifo := filepath.Join(f.root, "docs", "specs", "pipe.md")
	mkfifo(t, fifo)
	o := f.opts()
	o.Doc = "docs/specs/pipe.md"
	done := make(chan error, 1)
	go func() { _, err := Status(context.Background(), &o); done <- err }()
	select {
	case err := <-done:
		wantReason(t, err, "usage")
	case <-time.After(5 * time.Second):
		t.Fatal("tb egm blocked on a FIFO document")
	}
}

// TestFlowDuplicatePlanPath: the same missing path twice in the plan keeps a
// single lineage across rounds (review finding 2).
func TestFlowDuplicatePlanPath(t *testing.T) {
	f := newFlow(t, "")
	f.edit("## Technical Plan\n", "## Technical Plan\n\nPlan.\n\n## Detailed Implementation Plan\n\n| File | Action | Reason |\n|---|---|---|\n| `src/a.go` | Modify | x |\n| `src/a.go` | Modify | y |\n\n")
	ctx := context.Background()
	n1 := len(f.must(Review(ctx, f.optsP())).Report.Findings)
	f.edit("Plan.", "Plan, v2.")
	n2 := len(f.must(LoopNext(ctx, f.optsP())).Report.Findings)
	f.edit("Plan, v2.", "Plan, v3.")
	n3 := len(f.must(LoopNext(ctx, f.optsP())).Report.Findings)
	if n1 != n2 || n2 != n3 {
		t.Fatalf("lineages grew: %d %d %d", n1, n2, n3)
	}
}
