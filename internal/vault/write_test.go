package vault

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fixedNow = func() time.Time { return time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC) }

func TestNew(t *testing.T) {
	root, v := newRepo(t, nil)
	r, err := v.New(&NewOptions{Title: "SQLite adiado", Type: "decision", Tags: []string{"storage"}, Body: "Why.\n", Now: fixedNow})
	if err != nil || r.Path != "sqlite-adiado" || r.File != ".toolbelt/vault/sqlite-adiado.md" || r.Redacted {
		t.Fatalf("%+v %v", r, err)
	}
	b, _ := os.ReadFile(filepath.Join(root, ".toolbelt", "vault", "sqlite-adiado.md"))
	want := "---\ntitle: SQLite adiado\ntype: decision\ntags:\n    - storage\nsource: manual\ncreated: \"2026-11-20T12:00:00Z\"\n---\n# SQLite adiado\n\nWhy.\n"
	if string(b) != want {
		t.Fatalf("content:\n%s", b)
	}
	if c := check(t, v); !c.OK || c.Notes != 1 || len(c.Issues) != 0 {
		t.Fatalf("check after new: %+v", c)
	}
	_, err = v.New(&NewOptions{Title: "SQLite adiado"})
	wantReason(t, err, "note_exists", ExitFail)
}

func TestNewRefusals(t *testing.T) {
	root, v := newRepo(t, nil)
	half := 0.5
	cases := []struct {
		o      NewOptions
		reason string
	}{
		{NewOptions{Title: "x\nsource: manual"}, "bad_title"},
		{NewOptions{Title: "!!!"}, "bad_title"},
		{NewOptions{Title: " "}, "bad_title"},
		{NewOptions{Title: "a", Type: "essay"}, "usage"},
		{NewOptions{Title: "a", Source: SourceInferred}, "usage"},
		{NewOptions{Title: "a", Confidence: &half}, "usage"},
		{NewOptions{Title: "a", Tags: []string{"Bad Tag"}}, "usage"},
		{NewOptions{Title: "a", Dir: "../out"}, "dir_outside_vault"},
		{NewOptions{Title: "a", Dir: "/abs"}, "dir_outside_vault"},
		{NewOptions{Title: "a", Dir: ".obsidian"}, "dir_outside_vault"},
		{NewOptions{Title: "a", Body: strings.Repeat("x", 300<<10)}, "note_too_large"},
	}
	for _, c := range cases {
		_, err := v.New(&c.o)
		wantReason(t, err, c.reason, ExitError)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".toolbelt", "vault", "link")); err != nil {
		t.Fatal(err)
	}
	_, err := v.New(&NewOptions{Title: "a", Dir: "link/sub"})
	wantReason(t, err, "dir_outside_vault", ExitError)
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("wrote outside: %v", entries)
	}
}

func TestNewRedactsAndQuotes(t *testing.T) {
	root, v := newRepo(t, nil)
	secret := "ghp_" + strings.Repeat("z", 30)
	half := 0.5
	r, err := v.New(&NewOptions{Title: "Key " + secret + ": done #1", Body: "token " + secret, Source: SourceInferred, Confidence: &half, Dir: "a/b", Now: fixedNow})
	if err != nil || !r.Redacted || strings.Contains(r.Path, "ghp_") || r.Path != "a/b/key-redacted-done-1" {
		t.Fatalf("%+v %v", r, err)
	}
	b, _ := os.ReadFile(filepath.Join(root, ".toolbelt", "vault", "a", "b", "key-redacted-done-1.md"))
	if strings.Contains(string(b), "ghp_") {
		t.Fatalf("secret written:\n%s", b)
	}
	s, err := v.Show("a/b/key-redacted-done-1")
	if err != nil || s.Note.Data.Title != "Key [REDACTED]: done #1" || s.Note.Source != SourceInferred || *s.Note.Confidence != 0.5 {
		t.Fatalf("round trip %+v %v", s, err)
	}
	long := strings.Repeat("palavra ", 30)
	r, err = v.New(&NewOptions{Title: long})
	if err != nil || len(r.Path) > maxSlugBytes || strings.HasSuffix(r.Path, "-") {
		t.Fatalf("long slug %+v %v", r, err)
	}
}

func TestSearch(t *testing.T) {
	secret := "ghp_" + strings.Repeat("k", 30)
	_, v := newRepo(t, map[string]string{
		"a.md": "---\ntitle: Decisão sobre SQLite\n---\nbody\n",
		"b.md": "# B\n\nline two\nMencionamos a migração aqui.\n",
		"c.md": "token " + secret + "\n",
		"d.md": "---\ntags: [storage]\n---\n",
	})
	hit := func(q string) []string {
		t.Helper()
		r, err := v.Search(q, 0)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, h := range r.Hits {
			out = append(out, h.Path+":"+h.Field)
		}
		return out
	}
	if got := hit("decisao"); strings.Join(got, ",") != "a:title" {
		t.Fatalf("accent: %v", got)
	}
	if got := hit("MIGRACAO"); strings.Join(got, ",") != "b:body" {
		t.Fatalf("case/accent body: %v", got)
	}
	if got := hit("storage"); strings.Join(got, ",") != "d:tags" {
		t.Fatalf("tags: %v", got)
	}
	if got := hit("ghp_k"); len(got) != 0 {
		t.Fatalf("secret probe found: %v", got)
	}
	r, _ := v.Search("migração", 0)
	if r.Hits[0].Line != 4 || !strings.Contains(r.Hits[0].Data.Snippet, "migração") {
		t.Fatalf("line/snippet %+v", r.Hits[0])
	}
	if r, _ := v.Search("e", 1); len(r.Hits) != 1 {
		t.Fatalf("limit: %+v", r.Hits)
	}
	_, err := v.Search("  ", 0)
	wantReason(t, err, "usage", ExitError)
}

func TestNewReviewRegressions(t *testing.T) {
	root, v := newRepo(t, nil)
	nan, inf := math.NaN(), math.Inf(1)
	for _, c := range []*float64{&nan, &inf} {
		_, err := v.New(&NewOptions{Title: "a", Source: SourceInferred, Confidence: c})
		wantReason(t, err, "usage", ExitError)
	}
	for _, title := range []string{"evil‮TXT", "zero​width", "line sep"} {
		_, err := v.New(&NewOptions{Title: title})
		wantReason(t, err, "bad_title", ExitError)
	}
	// Raw size is checked before redaction can shrink it.
	pem := "-----BEGIN RSA PRIVATE KEY-----\n" + strings.Repeat("A", 300<<10) + "\n-----END RSA PRIVATE KEY-----"
	_, err := v.New(&NewOptions{Title: "big", Body: pem})
	wantReason(t, err, "note_too_large", ExitError)
	if r, err := v.New(&NewOptions{Title: "CON"}); err != nil || r.Path != "con-note" {
		t.Fatalf("reserved: %+v %v", r, err)
	}
	for _, dir := range []string{"C:foo", "a:b", "nul", "x/aux", "end.", "end "} {
		_, err := v.New(&NewOptions{Title: "d", Dir: dir})
		wantReason(t, err, "dir_outside_vault", ExitError)
	}
	// A refused note leaves no directory behind.
	_, _ = v.New(&NewOptions{Title: "big", Dir: "made", Body: strings.Repeat("x", 300<<10)})
	if _, err := os.Stat(filepath.Join(root, ".toolbelt", "vault", "made")); !os.IsNotExist(err) {
		t.Fatalf("debris dir: %v", err)
	}
}

func TestEnsureDirRefusesSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	err := EnsureDir(root, "docs/notes")
	wantReason(t, err, "vault_path_outside_repo", ExitError)
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("created outside: %v", entries)
	}
	if err := EnsureDir(root, "real/notes"); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(root, "real", "notes")); err != nil || !fi.IsDir() {
		t.Fatalf("not created: %v", err)
	}
}

func TestSearchReviewRegressions(t *testing.T) {
	_, v := newRepo(t, map[string]string{
		"a.md": "---\nx: 1\ntitle: Alpha\n---\n",
		"b.md": "\r\n\r\n# Beta heading\r\nbody\r\n",
		"c.md": "token ghp_" + strings.Repeat("k", 30) + "\n",
		"d.md": "word\n", "e.md": "word\n", "f.md": "word\n",
	})
	if r, _ := v.Search("redacted", 0); len(r.Hits) != 0 {
		t.Fatalf("placeholder probe: %+v", r.Hits)
	}
	if r, _ := v.Search("alpha", 0); r.Hits[0].Line != 3 {
		t.Fatalf("frontmatter title line %d", r.Hits[0].Line)
	}
	if r, _ := v.Search("beta heading", 0); r.Hits[0].Line != 3 || r.Hits[0].Field != "title" {
		t.Fatalf("H1 title line %+v", r.Hits[0])
	}
	r, _ := v.Search("word", 2)
	if len(r.Hits) != 2 || !r.Truncated || r.Hits[0].Path != "d" || r.Hits[1].Path != "e" {
		t.Fatalf("limit: %+v", r)
	}
	if r, _ := v.Search("word", 3); r.Truncated {
		t.Fatal("exact limit reported as truncated")
	}
}
