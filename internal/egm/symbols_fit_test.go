package egm

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func (f *flow) commitReceiptBytes(name string, b []byte) {
	f.t.Helper()
	writeFile(f.t, filepath.Join(f.root, filepath.FromSlash(ReceiptPath(f.doc))), string(b))
	gitT(f.t, f.root, "add", "-f", ".toolbelt")
	gitT(f.t, f.root, "commit", "-qm", name)
}

// A v3 receipt must hold every citation of its document: a stripped key, or
// `symbols: null`, is receipt_invalid in CI and unreadable in drift-report.
func TestReceiptV3KeySetMustEqualCitations(t *testing.T) {
	f, ctx := symFlow(t, "`internal/sym.go#Foo` `internal/sym.go#Bar`")
	rc, raw := f.readReceipt()
	if len(rc.Symbols) != 2 {
		t.Fatalf("symbols %v", rc.Symbols)
	}
	if _, _, code, err := receiptCheck(ctx, f.root, f.doc, "HEAD", f.cfg); err != nil || code != "" {
		t.Fatalf("intact receipt: %q %v", code, err)
	}
	stripped := *rc
	stripped.Symbols = map[string]string{"internal/sym.go#Foo": rc.Symbols["internal/sym.go#Foo"]}
	nulled := regexp.MustCompile(`(?s)"symbols": \{.*?\}`).ReplaceAll(raw, []byte(`"symbols": null`))
	for name, b := range map[string][]byte{"missing key": stripped.Encode(), "symbols null": nulled} {
		f.commitReceiptBytes(name, b)
		if _, _, code, err := receiptCheck(ctx, f.root, f.doc, "HEAD", f.cfg); err != nil || code != "receipt_invalid" {
			t.Errorf("%s: ci code %q %v", name, code, err)
		}
		if d := driftOfDoc(f.drift(ctx, ""), f.doc); d.Status != DriftUnreadable {
			t.Errorf("%s: drift %+v", name, d)
		}
	}
}

// A path is symbol-scoped only when every symbol citation of it is in the
// baseline; otherwise any edit of the file is drift.
func TestSymbolScopeNeedsEveryKey(t *testing.T) {
	text := "`internal/sym.go#Foo` `internal/sym.go#Bar` `internal/other.go#Baz`\n"
	h := sha256Hex(nil)
	sc := symbolScopes("docs/specs/x.md", text, nil, map[string]string{"internal/sym.go#Foo": h, "internal/other.go#Baz": h})
	if _, ok := sc["internal/sym.go"]; ok {
		t.Fatalf("a path with a missing key is symbol-scoped: %v", sc)
	}
	if len(sc["internal/other.go"]) != 1 {
		t.Fatalf("complete path lost its scope: %v", sc)
	}
}

// Missing and ambiguous symbols block a receipt at write time.
func TestReceiptRefusedForUnresolvedSymbol(t *testing.T) {
	for _, cite := range []string{"`internal/sym.go#Nope`", "`internal/sym.go#init`"} {
		f := newFlow(t, "  clean_rounds: 1\n  receipt: {write_v3: true}\n")
		writeFile(t, filepath.Join(f.root, "internal", "sym.go"), symGo+"\nfunc init() {}\n")
		f.edit("`internal/missing.go`", cite)
		gitT(t, f.root, "add", ".")
		gitT(t, f.root, "commit", "-qm", "cite")
		ctx := context.Background()
		o := f.opts()
		o.Engine = EngineDeterministic
		rep := f.must(Review(ctx, &o)).Report
		if rep.Verdict == VerdictPass || rep.Verdict == VerdictPassPartial {
			t.Errorf("%s: verdict %s", cite, rep.Verdict)
		}
		if out := f.must(WriteReceipt(ctx, f.optsP())); out.ExitCode == ExitOK || out.Receipt != "" {
			t.Errorf("%s: receipt written: %+v", cite, out)
		}
	}
}

func TestSymbolAmbiguousIsMajor(t *testing.T) {
	f := newFlow(t, "")
	writeFile(t, filepath.Join(f.root, "internal", "sym.go"), symGo+"\nfunc init() {}\n")
	f.edit("`internal/missing.go`", "`internal/sym.go#init`")
	gitT(t, f.root, "add", ".")
	gitT(t, f.root, "commit", "-qm", "cite")
	o := f.opts()
	o.Engine = EngineDeterministic
	var hit bool
	for _, fd := range f.must(Review(context.Background(), &o)).Report.Findings {
		if strings.Contains(fd.Message, "symbol_ambiguous") {
			hit = fd.Severity == SevMajor
		}
	}
	if !hit {
		t.Fatal("no major symbol_ambiguous finding for #init")
	}
}

// R16 and refs exemptions: the example marker and refs_ignore exempt a
// symbol citation; a cited .go dropped for the packet budget or a secret is
// reported as not verified.
func TestSymbolCitationExemptionsAndDrops(t *testing.T) {
	run := func(extra, cite string, files map[string]string) *Report {
		f := newFlow(t, extra)
		for p, c := range files {
			writeFile(t, filepath.Join(f.root, filepath.FromSlash(p)), c)
		}
		f.edit("`internal/missing.go`", cite)
		gitT(t, f.root, "add", ".")
		gitT(t, f.root, "commit", "-qm", "cite")
		o := f.opts()
		o.Engine = EngineDeterministic
		return f.must(Review(context.Background(), &o)).Report
	}
	has := func(r *Report, sub string) bool {
		for _, fd := range r.Findings {
			if strings.Contains(fd.Message, sub) {
				return true
			}
		}
		return false
	}
	files := map[string]string{"internal/sym.go": symGo}
	if r := run("", "`internal/sym.go#Nope` "+exampleMarker, files); has(r, "`Nope`") {
		t.Errorf("example marker ignored: %+v", r.Findings)
	}
	if r := run("  refs_ignore: [internal/sym.go]\n", "`internal/sym.go#Nope`", files); has(r, "`Nope`") {
		t.Errorf("refs_ignore ignored: %+v", r.Findings)
	}
	secret := map[string]string{"internal/sym.go": symGo + "\n" + secretLine}
	r := run("", "`internal/sym.go#Foo`", secret)
	var nit bool
	for _, fd := range r.Findings {
		nit = nit || (fd.Severity == SevNit && strings.Contains(fd.Message, "not verified (secret_like)"))
	}
	if !nit {
		t.Errorf("no not-verified nit: %+v", r.Findings)
	}
}

// Round state records the resolved symbols (16).
func TestRoundRecordsSymbols(t *testing.T) {
	f, _ := symFlow(t, "`internal/sym.go#Foo`")
	repo, err := DiscoverRepo(filepath.Join(f.root, ".x"))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := StateDir(repo.ID, Slug(f.doc))
	if err != nil {
		t.Fatal(err)
	}
	st, err := loadState(dir)
	if err != nil || st == nil || len(st.Rounds) == 0 {
		t.Fatalf("state %v %v", st, err)
	}
	if r := st.Rounds[len(st.Rounds)-1]; len(r.Symbols) != 1 || r.Symbols["internal/sym.go#Foo"] == "" {
		t.Fatalf("round symbols %v", r.Symbols)
	}
}

// Acceptance 21 through CI: a v2 receipt gives the Phase B result (file
// hashes), whatever the symbols; 23: with no symbols the JSON is stable and
// carries the new arrays empty.
func TestCIReceiptV2PhaseBResult(t *testing.T) {
	f := newFlow(t, "  clean_rounds: 1\n  receipt: {write_v2: true}\n")
	writeFile(t, filepath.Join(f.root, "internal", "sym.go"), symGo)
	f.edit("`internal/missing.go`", "`internal/sym.go#Foo`")
	gitT(t, f.root, "add", ".")
	gitT(t, f.root, "commit", "-qm", "cite")
	ctx := context.Background()
	f.must(Review(ctx, f.optsP()))
	f.commitReceipt(ctx)
	if _, _, code, err := receiptCheck(ctx, f.root, f.doc, "HEAD", f.cfg); err != nil || code != "" {
		t.Fatalf("fresh v2: %q %v", code, err)
	}
	f.commitSym("other func", "return 2", "return 3")
	if _, _, code, err := receiptCheck(ctx, f.root, f.doc, "HEAD", f.cfg); err != nil || code != "receipt_stale" {
		t.Fatalf("v2 after an edit of the cited file: %q %v", code, err)
	}
}

func TestDriftJSONStableWithoutSymbols(t *testing.T) {
	f, ctx := convergedAtHead(t)
	f.commitReceipt(ctx)
	a, _ := json.Marshal(f.drift(ctx, ""))
	b, _ := json.Marshal(f.drift(ctx, ""))
	if !bytes.Equal(a, b) {
		t.Fatalf("not byte-identical:\n%s\n%s", a, b)
	}
	for _, k := range []string{`"symbols_changed":[]`, `"symbols_missing":[]`, `"outside_symbols":[]`} {
		if !strings.Contains(string(a), k) {
			t.Errorf("%s missing in %s", k, a)
		}
	}
}
