package egm

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
)

func (f *flow) ciGate(ctx context.Context, base string, offline bool) *GateResult {
	f.t.Helper()
	r, err := Gate(ctx, &GateOptions{RepoDir: f.root, Base: base, Head: "HEAD", ConfigFile: f.cfg, Mode: GateModeCI, AllowOffline: offline})
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func (f *flow) commitReceipt(ctx context.Context) {
	f.t.Helper()
	if out := f.must(WriteReceipt(ctx, f.optsP())); out.ExitCode != ExitOK {
		f.t.Fatalf("receipt: %+v", out)
	}
	gitT(f.t, f.root, "add", ".toolbelt")
	gitT(f.t, f.root, "commit", "-qm", "receipt")
}

func docReason(r *GateResult) string {
	if len(r.Docs) == 0 {
		return ""
	}
	return r.Docs[0].Reason
}

// TestCIReceiptChecks is T-R2 steps 1–2 and T-R4 with the deterministic
// engine (no agent_cmd): no_receipt, ci_offline, the offline opt-in,
// receipt_stale and receipt_invalid.
func TestCIReceiptChecks(t *testing.T) {
	f, ctx := convergedAtHead(t)
	if _, err := Gate(ctx, &GateOptions{RepoDir: f.root, Base: "HEAD~1", Head: "HEAD", ConfigFile: f.cfg, Mode: GateModeCI, AllowOffline: true}); err == nil {
		t.Fatal("ci without agent_cmd is a broken setup, not offline")
	}
	// A credential named in egm.env.pass but unset is offline (T-R4).
	writeFile(t, f.cfg, "egm:\n  clean_rounds: 1\n  env: {pass: [TB_EGM_TEST_KEY]}\n")
	t.Setenv("TB_EGM_TEST_KEY", "")
	if r := f.ciGate(ctx, "HEAD~1", false); r.OK || docReason(r) != "no_receipt" || r.Trust != GateModeCI {
		t.Fatalf("no receipt: %+v", r)
	}
	f.commitReceipt(ctx)
	if r := f.ciGate(ctx, "HEAD~2", false); r.OK || !strings.HasPrefix(docReason(r), "ci_offline") {
		t.Fatalf("offline: %+v", r)
	}
	r := f.ciGate(ctx, "HEAD~2", true)
	if !r.OK || !r.Warn || r.Docs[0].Verdict != VerdictPassPartial || !containsStr(r.Docs[0].Downgrades, "ci_offline") {
		t.Fatalf("offline allowed: %+v", r)
	}
	writeFile(t, filepath.Join(f.root, "internal", "kept.go"), "package internal // changed\n")
	gitT(t, f.root, "commit", "-qam", "source")
	if r := f.ciGate(ctx, "HEAD~3", true); r.OK || docReason(r) != "receipt_stale" {
		t.Fatalf("stale: %+v", r)
	}
	writeFile(t, filepath.Join(f.root, filepath.FromSlash(ReceiptPath(f.doc))), `{"version":1,"doc":"docs/specs/x.md","extra":true}`+"\n")
	gitT(t, f.root, "commit", "-qam", "bad receipt")
	if r := f.ciGate(ctx, "HEAD~4", true); r.OK || docReason(r) != "receipt_invalid" {
		t.Fatalf("invalid: %+v", r)
	}
}

func TestCIReceiptClaimValidation(t *testing.T) {
	ok := Receipt{Version: 1, Doc: "d.md", DocSHA256: "x", Sources: map[string]string{},
		Claims: []Claim{{ID: "F-1", Kind: ClaimDeclined, Severity: SevMajor, Lines: []int{2, 3}, Message: "m"}}}
	if !ok.valid("d.md", 10) {
		t.Fatal("valid receipt rejected")
	}
	for name, mut := range map[string]func(*Receipt){
		"no lines":  func(r *Receipt) { r.Claims[0].Lines = nil },
		"past end":  func(r *Receipt) { r.Claims[0].Lines = []int{2, 11} },
		"reversed":  func(r *Receipt) { r.Claims[0].Lines = []int{3, 2} },
		"nit":       func(r *Receipt) { r.Claims[0].Severity = SevNit },
		"kind":      func(r *Receipt) { r.Claims[0].Kind = "fixed" },
		"other doc": func(r *Receipt) { r.Doc = "e.md" },
		"version":   func(r *Receipt) { r.Version = 2 },
	} {
		r := ok
		r.Claims = []Claim{ok.Claims[0]}
		mut(&r)
		if r.valid("d.md", 10) {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCIUsageBudgetAndRemoved(t *testing.T) {
	f, ctx := convergedAtHead(t)
	if _, err := Gate(ctx, &GateOptions{RepoDir: f.root, Base: "HEAD~1", Head: "HEAD", Mode: GateModeCI}); err == nil {
		t.Fatal("ci mode without -c accepted")
	}
	if _, err := Gate(ctx, &GateOptions{RepoDir: f.root, Base: "HEAD~1", Head: "HEAD", ConfigFile: f.cfg, Mode: "remote"}); err == nil {
		t.Fatal("unknown mode accepted")
	}
	writeFile(t, f.cfg, "egm:\n  clean_rounds: 1\n  env: {pass: [TB_EGM_TEST_KEY]}\n  ci: {max_docs: 1}\n")
	t.Setenv("TB_EGM_TEST_KEY", "")
	r, err := Gate(ctx, &GateOptions{RepoDir: f.root, Base: "HEAD~1", Head: "HEAD", ConfigFile: f.cfg, Mode: GateModeCI, Docs: []string{"docs/specs/x.md", "docs/adr/001-y.md"}})
	if err != nil || r.OK || !hasGateWarning(r, "ci_budget") {
		t.Fatalf("max_docs: %+v", r)
	}
	gitT(t, f.root, "rm", "-q", f.doc)
	gitT(t, f.root, "commit", "-qm", "remove")
	r = f.ciGate(ctx, "HEAD~1", true)
	if r.OK || !hasGateWarning(r, "gated_doc_removed") {
		t.Fatalf("removed: %+v", r)
	}
	if r := f.gate(ctx, "HEAD~1", false); !r.OK || !hasGateWarning(r, "gated_doc_removed") {
		t.Fatalf("removed, local: %+v", r)
	}
}

// TestGateLocalReceiptWarnings is T-R8.
func TestGateLocalReceiptWarnings(t *testing.T) {
	f, ctx := convergedAtHead(t)
	r := f.gate(ctx, "HEAD~1", false)
	if !r.OK || r.Trust != GateModeLocal || !hasGateWarning(r, "receipt_missing") {
		t.Fatalf("missing: %+v", r)
	}
	f.commitReceipt(ctx)
	if r := f.gate(ctx, "HEAD~2", false); hasGateWarning(r, "receipt_missing") || hasGateWarning(r, "receipt_stale") {
		t.Fatalf("fresh: %+v", r.Warnings)
	}
}

func TestCIMatchClaim(t *testing.T) {
	set := &Settings{EgmConfig: mustDefault()}
	msg := "the design reviewer finds this statement unverifiable as written"
	f := &ReportFinding{ID: "F-1", Severity: SevMajor, Owner: "design", Scope: Scope{Heading: "## Goals", Lines: []int{10, 10}}, Message: msg}
	arb := Claim{ID: "F-1", Kind: ClaimArbiterWithdrawn, Severity: SevMajor, Heading: "## Goals", Lines: []int{9, 11}, Message: msg}
	dec := arb
	dec.ID, dec.Kind = "F-2", ClaimDeclined
	if c := matchClaim(f, []Claim{arb, dec}, set); c == nil || c.ID != "F-2" {
		t.Fatalf("declined must win over arbiter_withdrawn: %+v", c)
	}
	if c := matchClaim(f, []Claim{arb}, set); c == nil || c.ID != "F-1" {
		t.Fatalf("arbiter claim: %+v", c)
	}
	blocker := *f
	blocker.Severity = SevBlocker
	if c := matchClaim(&blocker, []Claim{dec}, set); c != nil {
		t.Fatalf("more severe finding matched: %+v", c)
	}
	lineless := *f
	lineless.Scope.Lines = nil
	if c := matchClaim(&lineless, []Claim{dec}, set); c != nil {
		t.Fatalf("finding without lines matched: %+v", c)
	}
	far := dec
	far.Lines = []int{40, 50}
	if c := matchClaim(f, []Claim{far}, set); c != nil {
		t.Fatalf("far claim matched: %+v", c)
	}
	if !contestable(f) || contestable(&ReportFinding{Owner: injectionChecker}) || contestable(&ReportFinding{Owner: "comprehension"}) {
		t.Fatal("contestable")
	}
	r := Receipt{Version: 1, Doc: "d.md", DocSHA256: "x", Sources: map[string]string{}, Claims: []Claim{dec, dec}}
	if r.valid("d.md", 60) {
		t.Fatal("duplicate claim ids accepted")
	}
}

func mustDefault() config.EgmConfig { return config.DefaultEgmConfig() }

// TestCIRenameInsideGate: a gated document moved to another gated path
// without an edit is not gated_doc_removed.
func TestCIRenameInsideGate(t *testing.T) {
	f, ctx := convergedAtHead(t)
	gitT(t, f.root, "mv", f.doc, "docs/specs/y.md")
	gitT(t, f.root, "commit", "-qm", "move")
	if r := f.gate(ctx, "HEAD~1", false); hasGateWarning(r, "gated_doc_removed") {
		t.Fatalf("rename inside the gate: %+v", r.Warnings)
	}
}
