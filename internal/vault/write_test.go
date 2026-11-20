package vault

import (
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
