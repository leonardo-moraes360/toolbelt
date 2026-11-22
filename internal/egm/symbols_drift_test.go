package egm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// symFlow is a converged document citing only internal/sym.go#Foo, with
// write_v3 on and a committed v3 receipt.
func symFlow(t *testing.T, cite string) (*flow, context.Context) {
	t.Helper()
	f := newFlow(t, "  clean_rounds: 1\n  receipt: {write_v3: true}\n")
	writeFile(t, filepath.Join(f.root, "internal", "sym.go"), symGo)
	f.edit("`internal/missing.go`", cite)
	gitT(t, f.root, "add", ".")
	gitT(t, f.root, "commit", "-qm", "cite symbol")
	ctx := context.Background()
	if r := f.must(Review(ctx, f.optsP())).Report; r.Verdict != VerdictPassPartial {
		t.Fatalf("review: %s %+v", r.Verdict, r.Findings)
	}
	f.commitReceipt(ctx)
	return f, ctx
}

func (f *flow) commitSym(msg, from, to string) {
	f.t.Helper()
	p := filepath.Join(f.root, "internal", "sym.go")
	b, err := os.ReadFile(p)
	if err != nil {
		f.t.Fatal(err)
	}
	s := strings.Replace(string(b), from, to, 1)
	if s == string(b) {
		f.t.Fatalf("%q not in sym.go", from)
	}
	writeFile(f.t, p, s)
	gitT(f.t, f.root, "commit", "-qam", msg)
}

func TestReceiptV3(t *testing.T) {
	f, ctx := symFlow(t, "`internal/sym.go#Foo`")
	rc, raw := f.readReceipt()
	if rc.Version != receiptVersionSymbols || len(rc.Symbols) != 1 || rc.Symbols["internal/sym.go#Foo"] == "" {
		t.Fatalf("receipt %+v\n%s", rc, raw)
	}
	if !strings.Contains(string(raw), "\"symbols\": {\n    \"internal/sym.go#Foo\"") {
		t.Fatalf("encoding:\n%s", raw)
	}
	d := driftOfDoc(f.drift(ctx, ""), f.doc)
	if d.Status != DriftFresh || d.Baseline != BaselineReceipt || len(d.OutsideSymbols) != 0 {
		t.Fatalf("drift %+v", d)
	}
	// A v3 receipt with no symbol citation keeps the key, empty.
	empty := Receipt{Version: receiptVersionSymbols, Commit: rc.Commit, Doc: rc.Doc, DocSHA256: rc.DocSHA256, Sources: rc.Sources, Claims: []Claim{}}
	if !strings.Contains(string(empty.Encode()), "\"symbols\": {}") {
		t.Fatalf("empty v3:\n%s", empty.Encode())
	}
	// Readers: wrong key set, bad hash and v2 with symbols are invalid.
	for name, mut := range map[string]func(*Receipt){
		"forged key": func(r *Receipt) { r.Symbols = map[string]string{"internal/sym.go#Bar": sha256Hex(nil)} },
		"bad hash":   func(r *Receipt) { r.Symbols = map[string]string{"internal/sym.go#Foo": "abc"} },
		"v2 symbols": func(r *Receipt) { r.Version = receiptVersionCommit },
	} {
		bad := *rc
		mut(&bad)
		writeFile(t, filepath.Join(f.root, filepath.FromSlash(ReceiptPath(f.doc))), string(bad.Encode()))
		gitT(t, f.root, "add", "-f", ".toolbelt")
		gitT(t, f.root, "commit", "-qm", name)
		if d := driftOfDoc(f.drift(ctx, ""), f.doc); d.Status != DriftUnreadable && d.Status != DriftNoBaseline {
			t.Errorf("%s: %+v", name, d)
		}
	}
}

func TestSymbolDrift(t *testing.T) {
	f, ctx := symFlow(t, "`internal/sym.go#Foo`")
	status := func() *DriftDoc { return driftOfDoc(f.drift(ctx, ""), f.doc) }

	f.commitSym("other func", "return 2", "return 3")
	if d := status(); d.Status != DriftFresh || len(d.OutsideSymbols) != 1 || d.OutsideSymbols[0] != "internal/sym.go" || len(d.Changed) != 0 {
		t.Fatalf("outside: %+v", d)
	}
	f.commitSym("doc comment", "// Foo does one thing.", "// Foo does one thing, reworded.")
	if d := status(); d.Status != DriftFresh {
		t.Fatalf("comment: %+v", d)
	}
	f.commitSym("helper", "return 1", "return 10")
	if d := status(); d.Status != DriftFresh {
		t.Fatalf("callee change is outside the symbol: %+v", d)
	}
	driftChangedAndMissing(ctx, t, f)
}

func driftChangedAndMissing(ctx context.Context, t *testing.T, f *flow) {
	t.Helper()
	status := func() *DriftDoc { return driftOfDoc(f.drift(ctx, ""), f.doc) }
	f.commitSym("foo body", "return helper()", "return helper() + 1")
	r := f.drift(ctx, "")
	d := driftOfDoc(r, f.doc)
	if d.Status != DriftSymbolChanged || len(d.SymbolsChanged) != 1 || d.SymbolsChanged[0].Symbol != "Foo" || r.ExitCode() != ExitFail || r.V != "egm-source-drift/0.2" {
		t.Fatalf("changed: %+v", d)
	}
	f.commitSym("rename foo", "func Foo()", "func Foo2()")
	if d := status(); d.Status != DriftSymbolMissing || d.SymbolsMissing[0].Reason != "deleted" {
		t.Fatalf("missing: %+v", d)
	}
	f.commitSym("break file", "package internal", "package internal\nfunc (")
	if d := status(); d.Status != DriftSymbolMissing || d.SymbolsMissing[0].Reason != "unparsed" {
		t.Fatalf("unparsed: %+v", d)
	}
	gitT(t, f.root, "rm", "-q", "internal/sym.go")
	gitT(t, f.root, "commit", "-qm", "rm")
	if d := status(); d.Status != DriftSourceMissing {
		t.Fatalf("file gone: %+v", d)
	}
}

func TestSymbolDriftPlainCitationIsFileScoped(t *testing.T) {
	f, ctx := symFlow(t, "`internal/sym.go#Foo` and `internal/sym.go`")
	if rc, _ := f.readReceipt(); len(rc.Symbols) != 1 {
		t.Fatalf("symbols %v", rc.Symbols)
	}
	f.commitSym("other func", "return 2", "return 3")
	if d := driftOfDoc(f.drift(ctx, ""), f.doc); d.Status != DriftDrifted {
		t.Fatalf("plain citation must keep file-level drift: %+v", d)
	}
}

func TestV2ReceiptStaysFileScoped(t *testing.T) {
	f := newFlow(t, "  clean_rounds: 1\n  receipt: {write_v2: true}\n")
	writeFile(t, filepath.Join(f.root, "internal", "sym.go"), symGo)
	f.edit("`internal/missing.go`", "`internal/sym.go#Foo`")
	gitT(t, f.root, "add", ".")
	gitT(t, f.root, "commit", "-qm", "cite")
	ctx := context.Background()
	f.must(Review(ctx, f.optsP()))
	f.commitReceipt(ctx)
	f.commitSym("other func", "return 2", "return 3")
	if d := driftOfDoc(f.drift(ctx, ""), f.doc); d.Status != DriftDrifted {
		t.Fatalf("v2 is file-scoped: %+v", d)
	}
}

func TestCISymbolDrift(t *testing.T) {
	f, ctx := symFlow(t, "`internal/sym.go#Foo`")
	ciOffline(t, f, "  receipt: {write_v3: true}\n")
	f.commitSym("foo body", "return helper()", "return helper() + 1")
	if r := f.ciGate(ctx, "HEAD~1", false); !hasGateWarning(r, "ci_source_drift") {
		t.Fatalf("symbol change: %+v", r)
	}
	f.commitSym("other func", "return 2", "return 3")
	if r := f.ciGate(ctx, "HEAD~1", false); hasGateWarning(r, "ci_source_drift") {
		t.Fatalf("outside change raised: %+v", r)
	}
}
