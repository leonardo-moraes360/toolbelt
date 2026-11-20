package egm

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
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
	writeFile(t, f.cfg, "egm:\n  clean_rounds: 1\n  ci: {max_docs: 1}\n")
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
