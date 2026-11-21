package egm

import (
	"context"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
)

// AC18: readers accept v1 and v2 (2 needs a 40- or 64-hex commit).
func TestReceiptVersions(t *testing.T) {
	hex40, hex64 := strings.Repeat("a", 40), strings.Repeat("b", 64)
	for _, tc := range []struct {
		version int
		commit  string
		ok      bool
	}{
		{1, "", true}, {1, hex40, false}, {2, hex40, true}, {2, hex64, true},
		{2, "", false}, {2, strings.Repeat("A", 40), false}, {2, hex40[:39], false}, {3, hex40, false},
	} {
		rc := &Receipt{Version: tc.version, Commit: tc.commit, Doc: "d.md", DocSHA256: "x", Sources: map[string]string{}}
		if got := rc.valid("d.md", 10); got != tc.ok {
			t.Errorf("version %d commit %q: valid=%v", tc.version, tc.commit, got)
		}
	}
}

// AC18: tb writes v2 (with HEAD) only with egm.receipt.write_v2, and drift
// reads it as a receipt baseline.
func TestReceiptWriteV2(t *testing.T) {
	f, ctx := convergedAtHead(t)
	writeFile(t, f.cfg, "egm:\n  clean_rounds: 1\n  receipt: {write_v2: true}\n")
	head := strings.TrimSpace(gitT(t, f.root, "rev-parse", "HEAD"))
	f.commitReceipt(ctx)
	rc, raw := f.readReceipt()
	if rc.Version != receiptVersionCommit || rc.Commit != head || !strings.Contains(string(raw), "\n  \"commit\": \""+head+"\",\n") {
		t.Fatalf("receipt %+v\n%s", rc, raw)
	}
	if d := driftOfDoc(f.drift(ctx, ""), f.doc); d.Status != DriftFresh || d.Baseline != BaselineReceipt {
		t.Fatalf("v2 baseline: %+v", d)
	}
}

// The new keys: receipt is user-only; drift.max_docs and bootstrap.max_bytes
// only go down.
func TestB3RepoConfig(t *testing.T) {
	user := config.DefaultEgmConfig()
	user.Receipt.WriteV2 = true
	set := ApplyRepoConfig(&user, []byte("egm:\n  receipt: {write_v2: false}\n  drift: {max_docs: 9}\n  bootstrap: {max_bytes: 100}\n"))
	if !set.Receipt.WriteV2 || set.Drift.MaxDocs != 5 || set.Bootstrap.MaxBytes != 100 || len(set.Warnings) != 2 {
		t.Fatalf("%+v %+v", set.EgmConfig, set.Warnings)
	}
}

// AC22: the Phase B personas load but are no reviewers.
func TestUtilityPersonas(t *testing.T) {
	for _, n := range UtilityPersonaNames {
		if _, err := LoadPersona(n); err != nil {
			t.Fatal(err)
		}
		wantReason(t, checkReviewers([]string{n}), "usage")
	}
	if p, _ := LoadPersona(driftPersona); p.MaxSeverity != SevMajor {
		t.Fatalf("drift cap %s", p.MaxSeverity)
	}
}

// AC21 (cleaning): comments, level-1 headings, an early Status line and the
// Sources section go; fenced code and later Status lines stay.
func TestCleanBootstrapBody(t *testing.T) {
	in := "Title\n=====\n\nStatus: done\n<!-- tb:egm-example -->\n# H1\n\n## Overview\n\nStatus: kept\n\n" +
		"```sh\n# not a heading\n## Sources\n```\n\n## Sources\n\n- [x](x.go)\n\n### Sub\n\n## Problem\n\nP.\n<!-- open"
	want := "## Overview\n\nStatus: kept\n\n```sh\n# not a heading\n## Sources\n```\n\n## Problem\n\nP."
	if got := cleanBootstrapBody(in); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if got := cleanBootstrapBody("## A\n<!<!-- x -->-- tb:egm-example -->\n"); got != "## A" {
		t.Fatalf("nested comment: %q", got)
	}
	for _, tc := range []struct {
		in, reason string
	}{
		{"", reasonBootstrapInvalid}, {"<!-- x -->", reasonBootstrapInvalid}, {"\xff\xfe", reasonBootstrapInvalid},
		{strings.Repeat("a", 11), reasonBootstrapInvalid}, {"```markdown\n## A\n```", ""},
	} {
		if _, why := bootstrapBody([]byte(tc.in), 10); why != tc.reason {
			t.Errorf("%q: %q", tc.in, why)
		}
	}
}

// The context diff drops the header that names the temp files.
func TestUnifiedDiff(t *testing.T) {
	requireUnixGit(t)
	d, err := unifiedDiff(context.Background(), "a\nb\nc\n", "a\nB\nc\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(d, "@@ ") || strings.Contains(d, "diff --git") || !strings.Contains(d, "-b\n+B\n") || !strings.Contains(d, " a\n") {
		t.Fatalf("diff:\n%s", d)
	}
}
