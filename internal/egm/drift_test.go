package egm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
)

func (f *flow) drift(ctx context.Context, at string, docs ...string) *DriftReport {
	f.t.Helper()
	r, err := Drift(ctx, &DriftOptions{Cwd: f.root, Docs: docs, At: at, ConfigFile: f.cfg})
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func driftOfDoc(r *DriftReport, doc string) *DriftDoc {
	for i := range r.Documents {
		if r.Documents[i].Doc == doc {
			return &r.Documents[i]
		}
	}
	return nil
}

// AC1–AC4, AC6, AC7: receipt baseline, changed/missing/added, a document
// that changed is no_baseline, --at reads blobs, an ungated doc is an error.
func TestDriftReport(t *testing.T) {
	f, ctx := convergedAtHead(t)
	f.commitReceipt(ctx)
	if d := driftOfDoc(f.drift(ctx, ""), f.doc); d.Status != DriftFresh || d.Baseline != BaselineReceipt {
		t.Fatalf("fresh: %+v", d)
	}
	writeFile(t, filepath.Join(f.root, "internal", "kept.go"), "package internal // changed\n")
	gitT(t, f.root, "commit", "-qam", "source")
	r := f.drift(ctx, "")
	if d := driftOfDoc(r, f.doc); d.Status != DriftDrifted || len(d.Changed) != 1 || d.Changed[0] != "internal/kept.go" || r.ExitCode() != ExitFail {
		t.Fatalf("drifted: %+v exit %d", d, r.ExitCode())
	}
	if d := driftOfDoc(f.drift(ctx, "HEAD~1"), f.doc); d.Status != DriftFresh {
		t.Fatalf("--at HEAD~1: %+v", d)
	}
	gitT(t, f.root, "rm", "-q", "internal/kept.go")
	gitT(t, f.root, "commit", "-qm", "rm source")
	if d := driftOfDoc(f.drift(ctx, ""), f.doc); d.Status != DriftSourceMissing || d.Missing[0].Reason != "deleted" {
		t.Fatalf("missing: %+v", d)
	}
	f.edit("The problem.", "The problem, edited.")
	gitT(t, f.root, "commit", "-qam", "doc")
	if r := f.drift(ctx, ""); driftOfDoc(r, f.doc).Status != DriftNoBaseline {
		t.Fatalf("doc changed: %+v", driftOfDoc(r, f.doc))
	}
	if _, err := Drift(ctx, &DriftOptions{Cwd: f.root, Docs: []string{"README.md"}, ConfigFile: f.cfg}); err == nil {
		t.Fatal("ungated document accepted")
	}
}

// AC2 (added) and AC3: a newly resolving cited path is sources_differ; size
// limits never decide drift.
func TestDriftAddedAndSize(t *testing.T) {
	f, ctx := convergedAtHead(t)
	f.edit("`internal/kept.go` too", "`internal/kept.go` too and `internal/later.go`")
	gitT(t, f.root, "commit", "-qam", "cite later")
	// A receipt for this version of the document, certifying kept.go only.
	doc, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(f.doc)))
	if err != nil {
		t.Fatal(err)
	}
	kept, err := os.ReadFile(filepath.Join(f.root, "internal", "kept.go"))
	if err != nil {
		t.Fatal(err)
	}
	rc := Receipt{Version: receiptVersion, Doc: f.doc, DocSHA256: sha256Hex(doc),
		Sources: map[string]string{"internal/kept.go": sha256Hex(kept)}, Claims: []Claim{}}
	writeFile(t, filepath.Join(f.root, filepath.FromSlash(ReceiptPath(f.doc))), string(rc.Encode()))
	gitT(t, f.root, "add", "-f", ".toolbelt")
	gitT(t, f.root, "commit", "-qm", "receipt")
	if d := driftOfDoc(f.drift(ctx, ""), f.doc); d.Status != DriftFresh {
		t.Fatalf("fresh: %+v", d)
	}
	// A repository limit far below kept.go's size never makes it missing.
	writeFile(t, filepath.Join(f.root, ".toolbelt", "config.yaml"), "egm:\n  max_source_bytes: 1\n")
	gitT(t, f.root, "add", "-f", ".toolbelt/config.yaml")
	gitT(t, f.root, "commit", "-qm", "tiny limit")
	if d := driftOfDoc(f.drift(ctx, ""), f.doc); d.Status != DriftFresh {
		t.Fatalf("size decided drift: %+v", d)
	}
	writeFile(t, filepath.Join(f.root, "internal", "later.go"), "package internal\n")
	gitT(t, f.root, "rm", "-q", "--cached", ".toolbelt/config.yaml")
	gitT(t, f.root, "add", "internal/later.go")
	gitT(t, f.root, "commit", "-qm", "later")
	if d := driftOfDoc(f.drift(ctx, ""), f.doc); d.Status != DriftSourcesDiffer || len(d.Added) != 1 || d.Added[0] != "internal/later.go" {
		t.Fatalf("added: %+v", d)
	}
}

// AC5: without a receipt only a converged round is a baseline.
func TestDriftStateBaseline(t *testing.T) {
	f, ctx := convergedAtHead(t)
	if d := driftOfDoc(f.drift(ctx, ""), f.doc); d.Status != DriftFresh || d.Baseline != BaselineState {
		t.Fatalf("state baseline: %+v", d)
	}
}

func ciOffline(t *testing.T, f *flow, extra string) {
	t.Helper()
	writeFile(t, f.cfg, "egm:\n  clean_rounds: 1\n  env: {pass: [TB_EGM_TEST_KEY]}\n"+extra)
	t.Setenv("TB_EGM_TEST_KEY", "")
}

// AC9, AC10: ci_source_drift warns by default, fails with drift.ci: fail,
// and does not blame a range for drift already at the merge-base.
func TestCISourceDrift(t *testing.T) {
	f, ctx := convergedAtHead(t)
	f.commitReceipt(ctx)
	ciOffline(t, f, "")
	writeFile(t, filepath.Join(f.root, "internal", "kept.go"), "package internal // changed\n")
	gitT(t, f.root, "commit", "-qam", "source")
	r := f.ciGate(ctx, "HEAD~1", false)
	if !hasGateWarning(r, "ci_source_drift") || !r.Warn || !r.OK {
		t.Fatalf("warn: %+v", r)
	}
	ciOffline(t, f, "  drift: {ci: fail}\n")
	if r := f.ciGate(ctx, "HEAD~1", false); r.OK || !hasGateWarning(r, "ci_source_drift") {
		t.Fatalf("fail: %+v", r)
	}
	writeFile(t, filepath.Join(f.root, "internal", "kept.go"), "package internal // changed again\n")
	gitT(t, f.root, "commit", "-qam", "source again")
	if r := f.ciGate(ctx, "HEAD~1", false); hasGateWarning(r, "ci_source_drift") {
		t.Fatalf("blamed an unrelated range: %+v", r)
	}
}

// AC8: a receipt-only edit selects its document; a removed receipt fails.
func TestCIReceiptEdits(t *testing.T) {
	f, ctx := convergedAtHead(t)
	f.commitReceipt(ctx)
	ciOffline(t, f, "")
	rp := filepath.Join(f.root, filepath.FromSlash(ReceiptPath(f.doc)))
	b, err := os.ReadFile(rp)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, rp, string(b)+"\n")
	gitT(t, f.root, "commit", "-qam", "receipt only")
	if r := f.ciGate(ctx, "HEAD~1", false); len(r.Docs) != 1 || r.Docs[0].Doc != f.doc {
		t.Fatalf("receipt edit did not select its document: %+v", r)
	}
	gitT(t, f.root, "rm", "-q", ReceiptPath(f.doc))
	gitT(t, f.root, "commit", "-qm", "drop receipt")
	if r := f.ciGate(ctx, "HEAD~1", false); r.OK || !hasGateWarning(r, "receipt_removed") {
		t.Fatalf("removed receipt: %+v", r)
	}
	if r := f.gate(ctx, "HEAD~1", false); !r.OK || !hasGateWarning(r, "receipt_removed") {
		t.Fatalf("local gate: %+v", r)
	}
}

// AC11: the local gate warns sources_changed from the receipt baseline.
func TestLocalGateSourcesChanged(t *testing.T) {
	f, ctx := convergedAtHead(t)
	f.commitReceipt(ctx)
	writeFile(t, filepath.Join(f.root, "internal", "kept.go"), "package internal // changed\n")
	gitT(t, f.root, "commit", "-qam", "source")
	if r := f.gate(ctx, "HEAD~1", false); !hasGateWarning(r, "sources_changed") {
		t.Fatalf("local: %+v", r)
	}
}

// AC9 (policy): the repository config can raise drift.ci to fail, never
// lower it.
func TestDriftCIPolicy(t *testing.T) {
	user := config.DefaultEgmConfig()
	if s := ApplyRepoConfig(&user, []byte("egm:\n  drift: {ci: fail}\n")); s.Drift.CI != DriftFail {
		t.Fatalf("raise: %+v", s.Drift)
	}
	user.Drift.CI = DriftFail
	if s := ApplyRepoConfig(&user, []byte("egm:\n  drift: {ci: warn}\n")); s.Drift.CI != DriftFail || len(s.Warnings) == 0 {
		t.Fatalf("lowered: %+v", s.Drift)
	}
}
