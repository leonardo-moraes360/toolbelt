package egm

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// converge runs loop next until the cycle converges.
func (f *flow) converge(ctx context.Context) *Report {
	f.t.Helper()
	for i := 0; i < 6; i++ {
		r := f.must(LoopNext(ctx, f.optsP())).Report
		if r.LoopState == "converged" {
			return r
		}
	}
	f.t.Fatal("did not converge")
	return nil
}

func (f *flow) readReceipt() (*Receipt, []byte) {
	f.t.Helper()
	b, err := os.ReadFile(filepath.Join(f.root, ReceiptPath(f.doc)))
	if err != nil {
		f.t.Fatal(err)
	}
	var rc Receipt
	if err := json.Unmarshal(b, &rc); err != nil {
		f.t.Fatal(err)
	}
	return &rc, b
}

func TestReceiptRefusedWhenNotPassing(t *testing.T) {
	f := newFlow(t, "")
	ctx := context.Background()
	out := f.must(WriteReceipt(ctx, f.optsP()))
	if out.ExitCode != ExitNotPassing || out.Receipt != "" {
		t.Fatalf("no state: %+v", out)
	}
	f.must(Review(ctx, f.optsP()))
	out = f.must(WriteReceipt(ctx, f.optsP()))
	if out.ExitCode != ExitNotPassing || out.Receipt != "" {
		t.Fatalf("in progress: %+v", out)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".toolbelt", "egm")); !os.IsNotExist(err) {
		t.Fatalf("receipt dir created on refusal: %v", err)
	}
	o := f.opts()
	o.At = "HEAD"
	_, err := WriteReceipt(ctx, &o)
	wantReason(t, err, "usage")
}

func TestReceiptFixedLineageIsNoClaim(t *testing.T) {
	f := newFlow(t, "")
	ctx := context.Background()
	f.must(Review(ctx, f.optsP()))
	f.edit("`internal/missing.go`", "`internal/kept.go` again")
	f.must(Resolve(ctx, &ResolveOptions{Options: f.opts(), Fix: []string{"F-1"}}))
	f.converge(ctx)
	out := f.must(WriteReceipt(ctx, f.optsP()))
	if out.ExitCode != ExitOK || out.Receipt != ReceiptPath(f.doc) {
		t.Fatalf("%+v", out)
	}
	rc, b := f.readReceipt()
	doc, _ := os.ReadFile(filepath.Join(f.root, f.doc))
	kept, _ := os.ReadFile(filepath.Join(f.root, "internal", "kept.go"))
	if rc.Version != 1 || rc.Doc != f.doc || rc.DocSHA256 != sha256Hex(doc) || len(rc.Claims) != 0 {
		t.Fatalf("receipt %+v", rc)
	}
	if rc.Sources["internal/kept.go"] != sha256Hex(kept) {
		t.Fatalf("sources %v", rc.Sources)
	}
	if !bytes.HasSuffix(b, []byte("}\n")) || !bytes.Contains(b, []byte("\n  \"claims\": []")) {
		t.Fatalf("encoding:\n%s", b)
	}
	fi, _ := os.Stat(filepath.Join(f.root, ReceiptPath(f.doc)))
	if fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode %v", fi.Mode())
	}
	f.must(WriteReceipt(ctx, f.optsP()))
	if _, b2 := f.readReceipt(); !bytes.Equal(b, b2) {
		t.Fatal("receipt bytes are not deterministic")
	}
}

func TestReceiptDeclinedAndForcedClaims(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		resolve    ResolveOptions
	}{
		{"declined", ClaimDeclined, ResolveOptions{Decline: []string{"F-1"}}},
		{"forced", ClaimForced, ResolveOptions{ForceSettle: "F-1", Reason: "accepted risk"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFlow(t, "")
			ctx := context.Background()
			f.must(Review(ctx, f.optsP()))
			r := tc.resolve
			r.Options = f.opts()
			f.must(Resolve(ctx, &r))
			if v := f.converge(ctx).Verdict; v != VerdictPassPartial {
				t.Fatalf("verdict %s", v)
			}
			f.must(WriteReceipt(ctx, f.optsP()))
			rc, _ := f.readReceipt()
			if len(rc.Claims) != 1 {
				t.Fatalf("claims %+v", rc.Claims)
			}
			c := rc.Claims[0]
			doc := ParseDoc(flowSpec, nil)
			if c.ID != "F-1" || c.Kind != tc.kind || c.Severity != SevMajor || len(c.Lines) != 2 ||
				c.Lines[0] < 1 || c.Lines[1] > len(doc.Lines) || c.Lines[0] > c.Lines[1] || c.Message == "" {
				t.Fatalf("claim %+v", c)
			}
		})
	}
}

func TestClaimKind(t *testing.T) {
	st := &State{Transitions: []Transition{
		{Lineage: "F-1", To: StFixed, By: "arbiter", Reason: "withdrawn: not supported"},
		{Lineage: "F-2", To: StFixed, By: "arbiter", Reason: "withdrawn: x"},
		{Lineage: "F-1", From: StFixed, To: StFixed, By: "reviewer"},
		{Lineage: "F-2", To: StReopened, By: "reviewer"},
	}}
	cases := []struct {
		l    *Lineage
		want string
	}{
		{&Lineage{ID: "F-1", Severity: SevMajor, Status: StFixed}, ClaimArbiterWithdrawn},
		{&Lineage{ID: "F-1", Severity: SevNit, Status: StFixed}, ""},
		{&Lineage{ID: "F-2", Severity: SevBlocker, Status: StReopened}, ""},
		{&Lineage{ID: "F-3", Severity: SevMajor, Status: StDeclined}, ClaimDeclined},
		{&Lineage{ID: "F-4", Severity: SevBlocker, Status: StSettled, Downgrade: "forced_blocker"}, ClaimForced},
		{&Lineage{ID: "F-5", Severity: SevMajor, Status: StSettled}, ""},
		{&Lineage{ID: "F-6", Severity: SevMajor, Status: StFixed}, ""},
	}
	for _, c := range cases {
		if got := claimKind(st, c.l); got != c.want {
			t.Errorf("%s %s %s: %q, want %q", c.l.ID, c.l.Severity, c.l.Status, got, c.want)
		}
	}
}

func TestSectionLines(t *testing.T) {
	text := "# T\n\n## A\n\na\n\n### A1\n\nx\n\n## B\n\nb\n"
	s := &snapshot{doc: ParseDoc(text, nil)}
	for h, want := range map[string][2]int{"## A": {3, 10}, "### A1": {7, 10}, "## B": {11, 13}, "## Missing": {1, 13}} {
		got := s.sectionLines(h)
		if got[0] != want[0] || got[1] != want[1] {
			t.Errorf("%s: %v, want %v", h, got, want)
		}
	}
	if !lineageLess("F-2", "F-10") || lineageLess("F-10", "F-9") {
		t.Fatal("lineageLess")
	}
}

func TestReceiptSymlinkRefused(t *testing.T) {
	f := newFlow(t, "")
	ctx := context.Background()
	f.must(Review(ctx, f.optsP()))
	f.must(Resolve(ctx, &ResolveOptions{Options: f.opts(), Decline: []string{"F-1"}}))
	f.converge(ctx)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(f.root, ".toolbelt")); err != nil {
		t.Fatal(err)
	}
	_, err := WriteReceipt(ctx, f.optsP())
	wantReason(t, err, "receipt_path_untrusted")
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("wrote outside the repository: %v", entries)
	}
}
