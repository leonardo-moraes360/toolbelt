package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo makes root/.toolbelt/vault with the given notes (path → text).
func newRepo(t *testing.T, notes map[string]string) (root string, v *Vault) {
	t.Helper()
	root = t.TempDir()
	for p, text := range notes {
		write(t, filepath.Join(root, ".toolbelt", "vault", filepath.FromSlash(p)), text)
	}
	if err := os.MkdirAll(filepath.Join(root, ".toolbelt", "vault"), 0o755); err != nil {
		t.Fatal(err)
	}
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, v
}

func write(t *testing.T, p, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func wantReason(t *testing.T, err error, reason string, code int) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Reason != reason || e.Code != code {
		t.Fatalf("want %s (exit %d), got %v", reason, code, err)
	}
}

func check(t *testing.T, v *Vault) *CheckResult {
	t.Helper()
	r, err := v.Check()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func codes(r *CheckResult) map[string]int {
	out := map[string]int{}
	for _, is := range r.Issues {
		out[is.Code]++
	}
	return out
}

func TestOpen(t *testing.T) {
	_, err := Open(t.TempDir())
	wantReason(t, err, "no_toolbelt_dir", ExitError)
	_, err = Open("relative")
	wantReason(t, err, "usage", ExitError)

	root := t.TempDir()
	write(t, filepath.Join(root, ".toolbelt", "config.yaml"), "vault:\n  path: notes\n")
	sub := filepath.Join(root, "deep", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = Open(sub)
	wantReason(t, err, "vault_not_found", ExitFail)
	write(t, filepath.Join(root, "notes", "a.md"), "# A\n")
	v, err := Open(sub) // config read from the root, not the cwd
	if err != nil || v.Rel != "notes" {
		t.Fatalf("custom path: %+v %v", v, err)
	}

	write(t, filepath.Join(root, ".toolbelt", "config.yaml"), "vault:\n  path: ../x\n")
	_, err = Open(root)
	wantReason(t, err, "config", ExitError)

	outside := t.TempDir()
	write(t, filepath.Join(root, ".toolbelt", "config.yaml"), "vault:\n  path: link\n")
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	_, err = Open(root)
	wantReason(t, err, "vault_path_outside_repo", ExitError)
}

func TestResolution(t *testing.T) {
	_, v := newRepo(t, map[string]string{
		"a.md":        "# A\n\n## Intro\n",
		"x/b/c.md":    "# C\n",
		"one/dup.md":  "",
		"two/dup.md":  "",
		"aliased.md":  "---\naliases: [nick]\n---\n",
		"v1.2.md":     "",
		"img/pic.png": "",
		"source.md": "[[a]] [[A#intro]] [[a#missing]] [[x/b/c]] [[b/c]] [[dup]] [[one/dup]] [[nick]] " +
			"![[pic.png]] [[gone.pdf]] [[v1.2]] [[#local]] [[nope]] [[a#^block]]\n## Local\n",
	})
	r, err := v.Links("source")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range r.Links {
		got = append(got, l.Data.Target+"="+l.State+":"+l.To)
	}
	want := []string{
		"a=ok:a", "A=ok:a", "a=broken_fragment:a", "x/b/c=ok:x/b/c", "b/c=ok:x/b/c", "dup=ambiguous:",
		"one/dup=ok:one/dup", "nick=broken:", "pic.png=attachment:img/pic.png", "gone.pdf=missing_attachment:",
		"v1.2=ok:v1.2", "=ok:source", "nope=broken:", "a=ok:a",
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
	c := codes(check(t, v))
	if c["broken_link"] != 2 || c["ambiguous_link"] != 1 || c["broken_fragment"] != 1 || c["missing_attachment"] != 1 {
		t.Fatalf("check codes %v", c)
	}
}

func TestFind(t *testing.T) {
	_, v := newRepo(t, map[string]string{"one/dup.md": "", "two/dup.md": "", "a.md": "", "b.md": "[[a]]\n[[a]]\n", "c.md": "x [[A]]\n"})
	_, err := v.Show("dup")
	wantReason(t, err, "ambiguous_link", ExitFail)
	_, err = v.Show("zzz")
	wantReason(t, err, "note_not_found", ExitFail)
	if r, err := v.Show("one/dup.md"); err != nil || r.Note.Path != "one/dup" {
		t.Fatalf("path with .md: %+v %v", r, err)
	}
	r, err := v.Backlinks("a")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Backlinks) != 3 || r.Backlinks[0].Path != "b" || r.Backlinks[1].Line != 2 || r.Backlinks[2].Path != "c" || r.Backlinks[2].Col != 3 {
		t.Fatalf("backlinks %+v", r.Backlinks)
	}
}

func TestFrontmatterClassification(t *testing.T) {
	cases := map[string]string{
		"---\ntitle: [x\n---\n":           "bad_frontmatter",
		"---\ntitle: x\n":                 "bad_frontmatter",
		"---\ntitle: [a, b]\n---\n":       "bad_frontmatter",
		"---\nsource: robot\n---\n":       "bad_frontmatter",
		"---\ntype: essay\n---\n":         "bad_frontmatter",
		"---\nconfidence: 2\n---\n":       "bad_frontmatter",
		"---\ncreated: yesterday\n---\n":  "bad_frontmatter",
		"---\nsource: inferred\n---\n":    "missing_confidence",
		"---\nconfidence: 0.5\n---\n":     "unused_confidence",
		"---\ntags: [ok, Bad Tag]\n---\n": "bad_tag",
	}
	for src, code := range cases {
		n := parseNote("v/n.md", "n", src, []string{"fact", "decision"})
		if len(n.Issues) != 1 || n.Issues[0].Code != code {
			t.Errorf("%q: issues %+v, want %s", src, n.Issues, code)
		}
	}
}

func TestFrontmatterFullAndDefaults(t *testing.T) {
	n := parseNote("v/n.md", "n", "---\ntitle: T\ntype: decision\ntags: one\naliases: [a1]\nsource: inferred\nconfidence: 0.7\ncreated: 2026-11-20\nupdated: 2026-11-20T10:00:00Z\nextra: kept\n---\n# Heading\nbody\n", []string{"fact", "decision"})
	got := fmt.Sprintf("%d|%s|%s|%v|%v|%s|%v|%s|%s|%q", len(n.Issues), n.Title, n.Type, n.Tags, n.Aliases, n.Source, *n.Confidence, n.Created, n.Updated, n.Body)
	if want := `0|T|decision|[one]|[a1]|inferred|0.7|2026-11-20|2026-11-20T10:00:00Z|"# Heading\nbody"`; got != want {
		t.Fatalf("full:\n got %s\nwant %s", got, want)
	}
	if n := parseNote("v/n.md", "n", "no frontmatter\n# First H1\n", nil); n.Title != "First H1" || n.Type != "fact" || n.Source != SourceManual {
		t.Fatalf("defaults: %+v", n)
	}
	if n := parseNote("v/sub/n.md", "sub/n", "text\n", nil); n.Title != "n" || n.Name != "n" {
		t.Fatalf("name title: %+v", n)
	}
	if n := parseNote("v/n.md", "n", "---\ntype: essay\n---\n", []string{"fact"}); n.Issues[0].Line != 2 || n.Type != "fact" {
		t.Fatalf("line and default: %+v", n)
	}
}

func TestSecurity(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret.md")
	write(t, outside, "TOP SECRET [[x]]\n")
	root, v := newRepo(t, map[string]string{
		"big.md":                 strings.Repeat("x", 300<<10),
		"sec.md":                 "# Title ghp_" + strings.Repeat("a", 30) + "\npassword: hunter22\n[[sk-" + strings.Repeat("b", 24) + "]]\n",
		".obsidian/workspace.md": "[[hidden]]",
	})
	vdir := filepath.Join(root, ".toolbelt", "vault")
	if err := os.Symlink(outside, filepath.Join(vdir, "link.md")); err != nil {
		t.Fatal(err)
	}
	mkfifo(t, filepath.Join(vdir, "fifo.md"))
	r := check(t, v) // terminates: the FIFO is never opened
	c := codes(r)
	if c["note_too_large"] != 1 || c["not_regular"] < 1 || r.Notes != 1 || r.ExitCode != ExitFail {
		t.Fatalf("check %+v", r)
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "TOP SECRET") || strings.Contains(string(b), "hidden") || strings.Contains(string(b), "sk-bbbb") {
		t.Fatalf("leak: %s", b)
	}
	s, err := v.Show("sec")
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(s)
	if strings.Contains(string(out), "ghp_aaaa") || strings.Contains(string(out), "hunter22") || strings.Contains(string(out), "sk-bbbb") {
		t.Fatalf("not redacted: %s", out)
	}
	l, _ := v.List(Filter{})
	if len(l.Notes) != 1 || strings.Contains(l.Notes[0].Data.Title, "ghp_") {
		t.Fatalf("list: %+v", l.Notes)
	}
}

func TestCheckEmptyAndDeterministic(t *testing.T) {
	_, v := newRepo(t, nil)
	if r := check(t, v); !r.OK || r.Notes != 0 || len(r.Issues) != 0 {
		t.Fatalf("empty: %+v", r)
	}
	_, v = newRepo(t, map[string]string{"b.md": "[[x]] [[y]]\n", "a.md": "---\ntype: zz\n---\n[[q]]\n"})
	a, _ := json.Marshal(check(t, v))
	b, _ := json.Marshal(check(t, v))
	if string(a) != string(b) {
		t.Fatal("check output is not deterministic")
	}
	r := check(t, v)
	if r.Issues[0].File != ".toolbelt/vault/a.md" || r.Issues[len(r.Issues)-1].Data.Target != "y" {
		t.Fatalf("order %+v", r.Issues)
	}
}

func TestListFilter(t *testing.T) {
	_, v := newRepo(t, map[string]string{
		"a.md": "---\ntype: decision\ntags: [db]\n---\n",
		"b.md": "---\nsource: inferred\nconfidence: 0.4\n---\n",
		"c.md": "",
	})
	for f, want := range map[Filter]string{{Type: "decision"}: "a", {Tag: "db"}: "a", {Source: "inferred"}: "b", {}: "a,b,c"} {
		r, err := v.List(f)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, n := range r.Notes {
			got = append(got, n.Path)
		}
		if strings.Join(got, ",") != want {
			t.Errorf("%+v: %v", f, got)
		}
	}
}

func TestReviewRegressions(t *testing.T) {
	root, v := newRepo(t, map[string]string{
		"big.md":  strings.Repeat("y", 300<<10),
		"two.png": "", "x/two.png": "",
		"src.md": "[[Dr. Smith]] [[v2.0 release]] [[gone.pdf]] [[big]] [[big#any]] [[two.png]] " +
			"[[a|ghp_" + strings.Repeat("c", 30) + "]] [[a#sk-" + strings.Repeat("d", 24) + "]]\n",
		"a.md":      "",
		"bad.md":    "---\ntitle: x\nsource: manual\n\tb: 2\n---\n",
		"unused.md": "---\ntitle: x\nconfidence: 0.4\n---\n",
	})
	_ = root
	r, err := v.Links("src")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range r.Links {
		got = append(got, l.State)
	}
	want := "broken broken missing_attachment ok ok ambiguous_attachment ok broken_fragment"
	if strings.Join(got, " ") != want {
		t.Fatalf("states %v", got)
	}
	out, _ := json.Marshal(r)
	if strings.Contains(string(out), "ghp_cccc") || strings.Contains(string(out), "sk-dddd") {
		t.Fatalf("label/fragment not redacted: %s", out)
	}
	c := check(t, v)
	if cs := codes(c); cs["broken_link"] != 2 || cs["ambiguous_attachment"] != 1 || cs["note_too_large"] != 1 {
		t.Fatalf("codes %v", cs)
	}
	checkIssueLines(t, c)
}

func TestFindForms(t *testing.T) {
	_, v := newRepo(t, map[string]string{"dir/a.md": "# H\n", "b.md": ""})
	for _, target := range []string{"a", "[[a]]", "a#H", "[[a|label]]", "dir/a", "dir/a.md", "DIR/A"} {
		r, err := v.Show(target)
		if err != nil || r.Note.Path != "dir/a" {
			t.Errorf("%q: %+v %v", target, r, err)
		}
	}
}

func TestDeterministicCommands(t *testing.T) {
	_, v := newRepo(t, map[string]string{"a.md": "# A\n[[b]] [[c]]\n", "b.md": "[[a]]\n", "c.md": "[[a#a]]\n"})
	run := func() string {
		s, _ := v.Show("a")
		l, _ := v.Links("a")
		b, _ := v.Backlinks("a")
		out, _ := json.Marshal([]any{s, l, b})
		return string(out)
	}
	if run() != run() {
		t.Fatal("show/links/backlinks are not deterministic")
	}
}

func TestSymlinkedVaultInsideRepo(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "real", "a.md"), "# A\n")
	write(t, filepath.Join(root, ".toolbelt", "config.yaml"), "vault:\n  path: notes\n")
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "notes")); err != nil {
		t.Fatal(err)
	}
	v, err := Open(root)
	if err != nil || v.Rel != "real" {
		t.Fatalf("%+v %v", v, err)
	}
}

func checkIssueLines(t *testing.T, c *CheckResult) {
	t.Helper()
	for _, is := range c.Issues {
		switch {
		case is.File == ".toolbelt/vault/bad.md" && is.Code == "bad_frontmatter" && (is.Line < 2 || is.Line > 4):
			// yaml.v3 reports the line where it noticed the problem, which can be
			// the one before; it must at least be inside the frontmatter.
			t.Errorf("yaml error line %d, want 2..4", is.Line)
		case is.File == ".toolbelt/vault/unused.md" && is.Code == "unused_confidence" && is.Line != 3:
			t.Errorf("unused_confidence line %d, want 3", is.Line)
		}
	}
}
