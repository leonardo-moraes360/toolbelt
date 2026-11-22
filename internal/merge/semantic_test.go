package merge

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

// addTicketFn is addTicket with arbitrary worktree changes (deletes,
// symlinks) instead of file writes.
func addTicketFn(t *testing.T, dir string, mutate func()) ticket.Ticket {
	t.Helper()
	s := ticket.Open(dir)
	tk, err := s.Create(&ticket.Ticket{Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	base := git(t, dir, "rev-parse", "main")
	git(t, dir, "checkout", "-q", "-b", "work/"+strings.ToLower(tk.ID), base)
	mutate()
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", tk.ID)
	head := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "checkout", "-q", "main")
	if _, err := s.Transition(tk.ID, ticket.StatusRunning); err != nil {
		t.Fatal(err)
	}
	out, err := s.Update(tk.ID, func(x *ticket.Ticket) error {
		x.Status = ticket.StatusDone
		x.Result = &ticket.Result{BaseCommit: base, HeadCommit: head}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func commitFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for f, c := range files {
		write(t, dir, f, c)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "more base")
}

func semanticRun(t *testing.T, dir string, strategy Strategy, dry bool, ids ...string) (*Result, error) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: ids, Strategy: strategy, DryRun: dry})
}

// AC2: adjacent edits conflict under fail, semantic applies both.
func TestSemanticDisjointAdjacentEdits(t *testing.T) {
	dir := newRepo(t)
	addTicket(t, dir, nil, map[string]string{"shared.txt": "l1\nl2\nl3\nA4\nl5\nl6\nl7\nl8\nl9\n"})
	addTicket(t, dir, nil, map[string]string{"shared.txt": "l1\nl2\nl3\nl4\nB5\nl6\nl7\nl8\nl9\n"})
	start := git(t, dir, "rev-parse", "main")
	if _, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001", "TKT-002"}, DryRun: true}); !errors.Is(err, ErrConflict) {
		t.Fatalf("fail must conflict: %v", err)
	}
	res, err := semanticRun(t, dir, StrategySemantic, false, "TKT-001", "TKT-002")
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, dir, "shared.txt"); got != "l1\nl2\nl3\nA4\nB5\nl6\nl7\nl8\nl9\n" {
		t.Fatalf("content %q", got)
	}
	checkAdjacentReport(t, dir, res, start)
	assertNoTempWorktree(t, dir)
}

func checkAdjacentReport(t *testing.T, dir string, res *Result, start string) {
	t.Helper()
	if res.Files[0].Status != "semantic" || len(res.Resolutions) != 1 || res.Resolutions[0].Rule != "disjoint" {
		t.Fatalf("report: %+v", res)
	}
	if rs := res.Resolutions[0]; rs.BaseSHA256 == "" || rs.ResultSHA256 == "" || rs.Ticket != "TKT-002" {
		t.Fatalf("resolution: %+v", rs)
	}
	if !res.Applied || res.ResultTree == "" || git(t, dir, "rev-parse", "main") == start {
		t.Fatalf("not applied: %+v", res)
	}
	if res.ResultTree != git(t, dir, "rev-parse", "main^{tree}") {
		t.Fatal("result_tree differs from the branch tree")
	}
	if res.Conflicts[0].Resolution != "semantic" || res.Conflicts[0].Hunks[0].Rule != "disjoint" {
		t.Fatalf("conflict: %+v", res.Conflicts[0])
	}
}

const goBase = "package p\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc f() { fmt.Println(os.Args) }\n"

// AC3: different imports added to one block resolve with go_imports.
func TestSemanticGoImports(t *testing.T) {
	dir := newRepo(t)
	commitFiles(t, dir, map[string]string{"p.go": goBase})
	addTicket(t, dir, nil, map[string]string{"p.go": strings.Replace(goBase, "\t\"os\"\n", "\t\"os\"\n\t\"strings\"\n", 1)})
	addTicket(t, dir, nil, map[string]string{"p.go": strings.Replace(goBase, "\t\"os\"\n", "\t\"os\"\n\t\"bytes\"\n", 1)})
	res, err := semanticRun(t, dir, StrategySemantic, false, "TKT-001", "TKT-002")
	if err != nil {
		t.Fatal(err)
	}
	got := readFile(t, dir, "p.go")
	if !strings.Contains(got, "\t\"os\"\n\t\"bytes\"\n\t\"strings\"\n") {
		t.Fatalf("imports not merged (sorted within the conflict region):\n%s", got)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "", got, 0); err != nil {
		t.Fatal(err)
	}
	if res.Resolutions[0].Rule != "go_imports" {
		t.Fatalf("rule %+v", res.Resolutions)
	}
}

// go_imports drops a spec that one side removed from base.
func TestSemanticGoImportsRemoval(t *testing.T) {
	dir := newRepo(t)
	commitFiles(t, dir, map[string]string{"p.go": goBase})
	addTicket(t, dir, nil, map[string]string{"p.go": strings.Replace(goBase, "\t\"os\"\n", "", 1)})
	addTicket(t, dir, nil, map[string]string{"p.go": strings.Replace(goBase, "\t\"os\"\n", "\t\"os\"\n\t\"io\"\n", 1)})
	if _, err := semanticRun(t, dir, StrategySemantic, false, "TKT-001", "TKT-002"); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, dir, "p.go")
	if strings.Contains(got, "\"os\"") || !strings.Contains(got, "\"io\"") {
		t.Fatalf("removal not honored:\n%s", got)
	}
}

// AC4: the same line changed differently stays in conflict with ranges, no ref moves.
func TestSemanticOverlapRefuses(t *testing.T) {
	dir := conflictRepo(t)
	start := git(t, dir, "rev-parse", "main")
	res, err := semanticRun(t, dir, StrategySemantic, false, "TKT-001", "TKT-002")
	if !errors.Is(err, ErrConflict) || !strings.Contains(err.Error(), "--strategy prompt") {
		t.Fatalf("err %v", err)
	}
	c := res.Conflicts[0]
	if c.Resolution != "" || c.Reason != "overlap" || len(c.Hunks) != 1 || c.Hunks[0].Reason != "overlap" || c.Hunks[0].OursLines[0] == 0 {
		t.Fatalf("conflict: %+v", c)
	}
	if res.Files[0].Status != "conflict" || git(t, dir, "rev-parse", "main") != start {
		t.Fatalf("state: %+v", res.Files)
	}
	assertNoTempWorktree(t, dir)
}

// AC5: file-level refusals carry their reason.
func TestSemanticFileLevelRefusals(t *testing.T) {
	dir := newRepo(t)
	commitFiles(t, dir, map[string]string{"bin.dat": "a\x00b\n", "gone.txt": "x\n"})
	if err := os.Symlink("t0", filepath.Join(dir, "ln")); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "link")
	addTicketFn(t, dir, func() {
		write(t, dir, "bin.dat", "a\x00c\n")
		_ = os.Remove(filepath.Join(dir, "gone.txt"))
		_ = os.Remove(filepath.Join(dir, "ln"))
		_ = os.Symlink("t1", filepath.Join(dir, "ln"))
		write(t, dir, "new.txt", "one\n")
	})
	addTicketFn(t, dir, func() {
		write(t, dir, "bin.dat", "a\x00d\n")
		write(t, dir, "gone.txt", "y\n")
		_ = os.Remove(filepath.Join(dir, "ln"))
		_ = os.Symlink("t2", filepath.Join(dir, "ln"))
		write(t, dir, "new.txt", "two\n")
	})
	res, err := semanticRun(t, dir, StrategySemantic, true, "TKT-001", "TKT-002")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err %v", err)
	}
	got := map[string]string{}
	for _, c := range res.Conflicts {
		got[c.File] = c.Reason
	}
	want := map[string]string{"bin.dat": "binary", "gone.txt": "delete_modify", "ln": "symlink", "new.txt": "add_add"}
	for f, r := range want {
		if got[f] != r {
			t.Errorf("%s: reason %q want %q (all %v)", f, got[f], r, got)
		}
	}
}

func TestClassifyStages(t *testing.T) {
	st := func(m1, m2, m3 string) *[3]stage {
		return &[3]stage{{mode: m1, blob: "a"}, {mode: m2, blob: "b"}, {mode: m3, blob: "c"}}
	}
	for _, tc := range []struct {
		s    *[3]stage
		want string
	}{
		{st("100644", "100755", "100644"), "mode"},
		{st("120000", "120000", "120000"), "symlink"},
		{st("160000", "160000", "160000"), "gitlink"},
		{st("100644", "100644", "100644"), ""},
		{&[3]stage{{}, {mode: "100644", blob: "b"}, {mode: "100644", blob: "c"}}, "add_add"},
		{&[3]stage{{mode: "100644", blob: "a"}, {}, {mode: "100644", blob: "c"}}, "delete_modify"},
	} {
		if got := classifyStages(tc.s); got != tc.want {
			t.Errorf("%+v: %q want %q", tc.s, got, tc.want)
		}
	}
}

// AC6: a sensitive path is never resolved, even with --allow-sensitive.
func TestSemanticSensitiveNeverResolved(t *testing.T) {
	dir := newRepo(t)
	ci := ".github/workflows/ci.yml"
	commitFiles(t, dir, map[string]string{ci: "a\nb\nc\nd\ne\n"})
	addTicket(t, dir, nil, map[string]string{ci: "a\nB1\nc\nd\ne\n"})
	addTicket(t, dir, nil, map[string]string{ci: "a\nb\nC2\nd\ne\n"})
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	res, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001", "TKT-002"}, Strategy: StrategySemantic, AllowSensitive: true})
	if !errors.Is(err, ErrConflict) || res.Conflicts[0].Reason != "sensitive" {
		t.Fatalf("err %v res %+v", err, res)
	}
}

// AC8: file content that looks like markers is parsed through --marker-size.
func TestSemanticMarkerLookalikeContent(t *testing.T) {
	dir := newRepo(t)
	base := "# T\n=======\n<<<<<<< x\nl3\nl4\n>>>>>>> y\n||||||| z\nl8\n"
	commitFiles(t, dir, map[string]string{"doc.md": base})
	addTicket(t, dir, nil, map[string]string{"doc.md": strings.Replace(base, "l3\n", "A3\n", 1)})
	addTicket(t, dir, nil, map[string]string{"doc.md": strings.Replace(base, "l4\n", "B4\n", 1)})
	if _, err := semanticRun(t, dir, StrategySemantic, false, "TKT-001", "TKT-002"); err != nil {
		t.Fatal(err)
	}
	want := "# T\n=======\n<<<<<<< x\nA3\nB4\n>>>>>>> y\n||||||| z\nl8\n"
	if got := readFile(t, dir, "doc.md"); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestMarkerSizeFor(t *testing.T) {
	s := &[3]stage{{data: []byte("x\n")}, {data: []byte("<<<<<<<<<\n=\n")}, {data: []byte(" =====\n")}}
	if n := markerSizeFor(s); n != 10 {
		t.Fatalf("n=%d", n)
	}
	if n := markerSizeFor(&[3]stage{}); n != 7 {
		t.Fatalf("default n=%d", n)
	}
}

// AC7: a resolution that leaves a foreign marker line, or breaks Go, stays unresolved.
func TestFinishChecks(t *testing.T) {
	mk := func(path, ours, theirs string, text []string) *fileState {
		rg := &region{ID: 1, Base: []string{"b\n"}, Ours: []string{"o\n"}, Theirs: []string{"t\n"}}
		st := &fileState{path: path, markerSize: 7, regions: []*region{rg}, segs: []segment{{reg: rg}},
			texts: map[int][]string{1: text}, rules: map[int]string{1: "agent"}, reasons: map[int]string{}}
		st.stages[1].data, st.stages[2].data = []byte(ours), []byte(theirs)
		return st
	}
	if _, why := mk("a.txt", "", "", []string{"ok\n", "=======\n"}).finish(); why != "agent_markers" {
		t.Fatalf("markers: %q", why)
	}
	if _, why := mk("a.txt", "", "", []string{"<<<<<<< ours\n"}).finish(); why != "agent_markers" {
		t.Fatalf("markers: %q", why)
	}
	// A marker line that the inputs already hold is allowed.
	st := mk("a.txt", "", "", []string{"=======\n"})
	st.regions[0].Ours = []string{"=======\n"}
	if _, why := st.finish(); why != "" {
		t.Fatalf("allowed marker: %q", why)
	}
	good := "package p\n\nfunc f() {}\n"
	if _, why := mk("a.go", good, good, []string{"func (\n"}).finish(); why != "parse_failed" {
		t.Fatalf("parse: %q", why)
	}
	if _, why := mk("a.go", "garbage(", good, []string{"func (\n"}).finish(); why != "" {
		t.Fatalf("go check skipped when a stage does not parse: %q", why)
	}
}

func TestDiffEditsAndDisjoint(t *testing.T) {
	base := []string{"a\n", "b\n", "c\n", "d\n"}
	ours := []string{"a\n", "B\n", "c\n", "d\n"}
	theirs := []string{"a\n", "b\n", "c\n", "D\n", "e\n"}
	st := &fileState{maxRegionLines: 100}
	text, ok, _ := st.disjoint(&region{Base: base, Ours: ours, Theirs: theirs})
	if !ok || joinLines(text) != "a\nB\nc\nD\ne\n" {
		t.Fatalf("%q %v", joinLines(text), ok)
	}
	// Two insertions at one base position overlap.
	if _, ok, why := st.disjoint(&region{Base: []string{"a\n"}, Ours: []string{"a\n", "x\n"}, Theirs: []string{"a\n", "y\n"}}); ok || why != "overlap" {
		t.Fatalf("same-position insertions: %v %q", ok, why)
	}
	if _, ok, why := (&fileState{maxRegionLines: 2}).disjoint(&region{Base: base, Ours: ours, Theirs: theirs}); ok || why != "region_too_large" {
		t.Fatalf("size: %v %q", ok, why)
	}
}

// AC9: repository merge.* keys are ignored with config_untrusted.
func TestSemanticRepoConfigUntrusted(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, ".toolbelt/config.yaml", "merge:\n  semantic:\n    max_file_bytes: 99999999\n")
	addTicket(t, dir, nil, map[string]string{"shared.txt": "l1\nl2\nl3\nA4\nl5\nl6\nl7\nl8\nl9\n"})
	addTicket(t, dir, nil, map[string]string{"shared.txt": "l1\nl2\nl3\nl4\nB5\nl6\nl7\nl8\nl9\n"})
	res, err := semanticRun(t, dir, StrategySemantic, true, "TKT-001", "TKT-002")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 || res.Warnings[0].Code != "config_untrusted" || !strings.Contains(res.Warnings[0].Message, "merge.semantic.max_file_bytes") {
		t.Fatalf("warnings %+v", res.Warnings)
	}
}

func TestSemanticTooLargeFile(t *testing.T) {
	dir := newRepo(t)
	addTicket(t, dir, nil, map[string]string{"shared.txt": "l1\nl2\nl3\nA4\nl5\nl6\nl7\nl8\nl9\n"})
	addTicket(t, dir, nil, map[string]string{"shared.txt": "l1\nl2\nl3\nl4\nB5\nl6\nl7\nl8\nl9\n"})
	cfg := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(cfg, []byte("merge:\n  semantic:\n    max_file_bytes: 10\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001", "TKT-002"}, Strategy: StrategySemantic, DryRun: true, Config: cfg})
	if !errors.Is(err, ErrConflict) || res.Conflicts[0].Reason != "too_large" {
		t.Fatalf("err %v res %+v", err, res)
	}
}

// fail and last_wins never read the user config: an invalid merge section must not matter.
func TestFailAndLastWinsIgnoreMergeConfig(t *testing.T) {
	dir := conflictRepo(t)
	cfg := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(cfg, []byte("merge:\n  prompt:\n    attempts: 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001", "TKT-002"}, Strategy: StrategyLastWins, DryRun: true, Config: cfg}); err != nil {
		t.Fatal(err)
	}
}

// .gitattributes at the picked commit: -merge, merge=binary and binary all
// refuse the file with reason binary, even when the text would merge.
func TestSemanticGitattributesRefuse(t *testing.T) {
	dir := newRepo(t)
	base := "l1\nl2\nl3\nl4\nl5\n"
	files := map[string]string{".gitattributes": "a.txt -merge\nb.txt merge=binary\nc.txt binary\n"}
	for _, f := range []string{"a.txt", "b.txt", "c.txt"} {
		files[f] = base
	}
	commitFiles(t, dir, files)
	one, two := map[string]string{}, map[string]string{}
	for _, f := range []string{"a.txt", "b.txt", "c.txt"} {
		one[f] = "l1\nA2\nl3\nl4\nl5\n"
		two[f] = "l1\nl2\nB3\nl4\nl5\n"
	}
	addTicket(t, dir, nil, one)
	addTicket(t, dir, nil, two)
	res, err := semanticRun(t, dir, StrategySemantic, true, "TKT-001", "TKT-002")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err %v", err)
	}
	if len(res.Conflicts) != 3 {
		t.Fatalf("conflicts %+v", res.Conflicts)
	}
	for _, c := range res.Conflicts {
		if c.Reason != "binary" {
			t.Errorf("%s: reason %q", c.File, c.Reason)
		}
	}
}

// A region whose text is not unique in its stage file is not merged by go_imports.
func TestGoImportsNeedsUniqueRegions(t *testing.T) {
	const ours = `package p

import (
	"a"
	"b"
)

import (
	"a"
	"b"
)
`
	const theirs = `package p

import (
	"a"
	"b"
)

import (
	"a"
	"b"
)
`
	merged := "package p\n\nimport (\n<<<<<<< ours\n\t\"a\"\n\t\"b\"\n||||||| base\n\t\"a\"\n=======\n\t\"a\"\n\t\"b\"\n>>>>>>> theirs\n)\n\nimport (\n\t\"a\"\n\t\"b\"\n)\n"
	st := &fileState{path: "p.go", markerSize: 7, texts: map[int][]string{}, rules: map[int]string{}, reasons: map[int]string{}, novel: map[int]int{}, maxRegionLines: 100}
	st.stages[1].data, st.stages[2].data = []byte(ours), []byte(theirs)
	if err := st.parse(merged); err != nil {
		t.Fatal(err)
	}
	rg := st.regions[0]
	if rg.OursExact || rg.TheirsExact {
		t.Fatalf("duplicated text must not be exact: %+v", rg)
	}
	if _, ok := st.goImports(rg); ok {
		t.Fatal("go_imports applied to an ambiguous region")
	}
	// The same file with a unique block is accepted.
	u := strings.Replace(ours, "\t\"a\"\n\t\"b\"\n)\n\nimport (\n\t\"a\"\n\t\"b\"\n", "\t\"a\"\n\t\"b\"\n)\n\nimport (\n\t\"c\"\n", 1)
	st2 := &fileState{path: "p.go", markerSize: 7, texts: map[int][]string{}, rules: map[int]string{}, reasons: map[int]string{}, novel: map[int]int{}, maxRegionLines: 100}
	st2.stages[1].data, st2.stages[2].data = []byte(u), []byte(u)
	m2 := strings.Replace(merged, "import (\n\t\"a\"\n\t\"b\"\n)\n", "import (\n\t\"c\"\n)\n", 1)
	if err := st2.parse(m2); err != nil {
		t.Fatal(err)
	}
	if !st2.regions[0].OursExact || !st2.regions[0].TheirsExact {
		t.Fatalf("unique region must be exact: %+v", st2.regions[0])
	}
	if _, ok := st2.goImports(st2.regions[0]); !ok {
		t.Fatal("go_imports must apply to a unique region")
	}
}

// CRLF files: marker lines end in \r\n and must still parse.
func TestSemanticCRLFDisjoint(t *testing.T) {
	dir := newRepo(t)
	crlf := func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }
	commitFiles(t, dir, map[string]string{"w.txt": crlf("l1\nl2\nl3\nl4\nl5\nl6\n")})
	addTicket(t, dir, nil, map[string]string{"w.txt": crlf("l1\nl2\nl3\nA4\nl5\nl6\n")})
	addTicket(t, dir, nil, map[string]string{"w.txt": crlf("l1\nl2\nl3\nl4\nB5\nl6\n")})
	if _, err := semanticRun(t, dir, StrategySemantic, false, "TKT-001", "TKT-002"); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, dir, "w.txt"), crlf("l1\nl2\nl3\nA4\nB5\nl6\n"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	for _, l := range []string{"<<<<<<< ours\r\n", "=======\r", "||||||| base\r\n", ">>>>>>> theirs\r\n"} {
		c := l[0]
		if !isMarker(l, c, 7) {
			t.Errorf("%q not a marker", l)
		}
	}
}

// A conflict at the end of a file without a final newline stays without one.
func TestSemanticNoAddedFinalNewline(t *testing.T) {
	dir := newRepo(t)
	commitFiles(t, dir, map[string]string{"n.txt": "l1\nl2\nl3\nl4"})
	addTicket(t, dir, nil, map[string]string{"n.txt": "l1\nl2\nA3\nl4"})
	addTicket(t, dir, nil, map[string]string{"n.txt": "l1\nl2\nl3\nB4"})
	if _, err := semanticRun(t, dir, StrategySemantic, false, "TKT-001", "TKT-002"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, dir, "n.txt"); got != "l1\nl2\nA3\nB4" {
		t.Fatalf("got %q", got)
	}
}

// A pick that does not resolve whole contributes no resolutions.
func TestSemanticUnappliedPickListsNoResolutions(t *testing.T) {
	dir := newRepo(t)
	commitFiles(t, dir, map[string]string{"ok.txt": "l1\nl2\nl3\nl4\nl5\n", "bad.txt": "x\n"})
	addTicket(t, dir, nil, map[string]string{"ok.txt": "l1\nl2\nA3\nl4\nl5\n", "bad.txt": "one\n"})
	addTicket(t, dir, nil, map[string]string{"ok.txt": "l1\nl2\nl3\nB4\nl5\n", "bad.txt": "two\n"})
	res, err := semanticRun(t, dir, StrategySemantic, true, "TKT-001", "TKT-002")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err %v", err)
	}
	if len(res.Resolutions) != 0 {
		t.Fatalf("resolutions of an unapplied pick: %+v", res.Resolutions)
	}
}
