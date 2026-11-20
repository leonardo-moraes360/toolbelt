package secplay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustLoad(t *testing.T) *Corpus {
	t.Helper()
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return c
}

func TestCorpusLoadsAndMirrorsAreReciprocal(t *testing.T) {
	c := mustLoad(t)
	if len(c.Red) == 0 || len(c.Blue) == 0 {
		t.Fatalf("empty corpus: red=%d blue=%d", len(c.Red), len(c.Blue))
	}
	// validateMirrors already ran in Load; assert every playbook has a mirror
	// and it resolves both ways.
	for name, pb := range c.Red {
		if pb.Mirror == "" {
			t.Errorf("red %s has no mirror", name)
			continue
		}
		m, ok := c.Pair(name)
		if !ok || m.Color != Blue {
			t.Errorf("red %s does not pair to a blue playbook", name)
			continue
		}
		back, ok := c.Pair(m.Name)
		if !ok || back.Name != name {
			t.Errorf("mirror of red %s (%s) does not pair back", name, m.Name)
		}
	}
	for name, pb := range c.Blue {
		if pb.Mirror == "" {
			t.Errorf("blue %s has no mirror", name)
		}
	}
}

func TestSearchCompactAndRelevant(t *testing.T) {
	c := mustLoad(t)
	hits := c.Search(Red, "deserialization", SearchOptions{Limit: 5})
	if len(hits) == 0 {
		t.Fatal("no hits for deserialization")
	}
	if hits[0].Name != "red-insecure-deserialization" {
		t.Errorf("top hit = %s, want red-insecure-deserialization", hits[0].Name)
	}
	// Empty query returns the list (name order), bounded by limit.
	all := c.Search(Blue, "", SearchOptions{Limit: 3})
	if len(all) != 3 {
		t.Errorf("empty query with limit 3 returned %d", len(all))
	}
}

func TestRenderBudgetAndSection(t *testing.T) {
	c := mustLoad(t)
	pb, ok := c.Get(Red, "red-sqli")
	if !ok {
		t.Fatal("red-sqli missing")
	}

	// Section slice returns just that block.
	sec, err := pb.Render(RenderOptions{Section: "Detection"})
	if err != nil {
		t.Fatalf("section render: %v", err)
	}
	if sec.Section == "" || strings.Contains(sec.Body, "## Where It Hides") {
		t.Errorf("section render leaked other sections: %q", sec.Body)
	}

	// Tiny budget forces truncation with a section index.
	tr, err := pb.Render(RenderOptions{Budget: 20})
	if err != nil {
		t.Fatalf("budget render: %v", err)
	}
	if !tr.Truncated {
		t.Error("expected truncated=true under a tiny budget")
	}
	if tr.Tokens > 20 {
		t.Errorf("budget exceeded: %d > 20", tr.Tokens)
	}
	if len(tr.Sections) == 0 {
		t.Error("truncated render should carry the section index")
	}

	// Full ignores the budget.
	full, err := pb.Render(RenderOptions{Budget: 20, Full: true})
	if err != nil {
		t.Fatalf("full render: %v", err)
	}
	if full.Truncated {
		t.Error("full render must not be truncated")
	}
	if full.Body != pb.Body() {
		t.Error("full render should equal the whole body")
	}
}

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Errorf("empty = %d, want 0", got)
	}
	if got := EstimateTokens("abcd"); got != 1 {
		t.Errorf("4 chars = %d, want 1", got)
	}
	if got := EstimateTokens("abcde"); got != 2 {
		t.Errorf("5 chars = %d, want 2 (round up)", got)
	}
}

func fileHit(vs []Violation, name string) bool {
	for _, v := range vs {
		if v.File == name {
			return true
		}
	}
	return false
}

func TestApplyWritesSecChecks(t *testing.T) {
	c := mustLoad(t)
	tb := filepath.Join(t.TempDir(), ".toolbelt")

	// Dry run writes nothing.
	if _, err := c.Apply(tb, "sql-injection", true); err != nil {
		t.Fatalf("dry-run apply: %v", err)
	}
	if _, err := os.Stat(SecChecksPath(tb)); !os.IsNotExist(err) {
		t.Fatal("dry-run must not write secchecks.yaml")
	}

	// Real apply writes the check.
	res, err := c.Apply(tb, "sql-injection", false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Added) == 0 {
		t.Fatal("apply added no checks")
	}
	if _, err := os.Stat(SecChecksPath(tb)); err != nil {
		t.Fatalf("secchecks.yaml not written: %v", err)
	}

	// Idempotent: second apply adds nothing new.
	res2, err := c.Apply(tb, "sql-injection", false)
	if err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if len(res2.Added) != 0 {
		t.Errorf("re-apply added %d (want 0, idempotent)", len(res2.Added))
	}
}

func TestRunFlagsPlantedPattern(t *testing.T) {
	c := mustLoad(t)
	dir := t.TempDir()
	tb := filepath.Join(dir, ".toolbelt")
	if _, err := c.Apply(tb, "sql-injection", false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	vuln := "db.query(\"SELECT * FROM users WHERE id=\" + id)\n"
	if err := os.WriteFile(filepath.Join(dir, "handler.go"), []byte(vuln), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "clean.go"), []byte("package x\nfunc f() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	vs, err := Run(dir, tb)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !fileHit(vs, "handler.go") {
		t.Error("expected a violation in handler.go")
	}
	if fileHit(vs, "clean.go") {
		t.Error("clean.go should not match")
	}
}

func TestRunSkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	tb := filepath.Join(dir, ".toolbelt")
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("PRIVATE KEY MATERIAL\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "leak.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	// A catch-all check over *.txt: a followed symlink would surface the secret.
	sc := &SecChecks{Version: 1, Checks: []Check{{
		ID: "catch-all", Description: "x", Pattern: "PRIVATE", IncludeGlobs: []string{"*.txt"}, Severity: SeverityWarning,
	}}}
	if err := saveSecChecks(SecChecksPath(tb), sc); err != nil {
		t.Fatal(err)
	}
	vs, err := Run(dir, tb)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, v := range vs {
		if v.File == "leak.txt" {
			t.Fatal("Run followed a symlink and read a file outside the tree")
		}
	}
}

func TestRenderNoSectionsStillTruncates(t *testing.T) {
	pb := &Playbook{Name: "x", Color: Red, body: "# Title\n\n" + strings.Repeat("word ", 4000)}
	pb.Sections = splitSections(pb.body) // nil: no level-2 heading
	r, err := pb.Render(RenderOptions{Budget: 30})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !r.Truncated {
		t.Error("want truncated for a long section-less body")
	}
	if r.Body == "" {
		t.Error("content lost silently: body is empty")
	}
	if r.Tokens > 30 {
		t.Errorf("budget exceeded: %d > 30", r.Tokens)
	}
}

func TestGlobWithSlashRejected(t *testing.T) {
	ch := Check{ID: "x", Pattern: "y", IncludeGlobs: []string{"internal/**/*.go"}}
	if _, err := ch.compile(); err == nil {
		t.Error("expected error for a glob with a path separator")
	}
}

// TestRunNeverExecutesBody guards the no-exec promise: a playbook body or a
// planted file containing shell-looking text must never be executed by load or
// run. We assert Run only matches text (no side effects) by planting a sentinel
// command whose execution would create a file, then checking it never appears.
func TestRunDoesNotExecute(t *testing.T) {
	c := mustLoad(t)
	dir := t.TempDir()
	tb := filepath.Join(dir, ".toolbelt")
	if _, err := c.Apply(tb, "rce", false); err != nil {
		t.Fatalf("apply rce: %v", err)
	}
	sentinel := filepath.Join(dir, "PWNED")
	payload := "os.system(\"touch " + sentinel + "\")\n"
	if err := os.WriteFile(filepath.Join(dir, "evil.py"), []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(dir, tb); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("Run executed the payload — no-exec guarantee violated")
	}
}
