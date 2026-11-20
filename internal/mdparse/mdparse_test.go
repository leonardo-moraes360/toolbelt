package mdparse

import (
	"reflect"
	"testing"
)

func TestWikilinkForms(t *testing.T) {
	cases := []struct {
		in   string
		want []Wikilink
	}{
		{"see [[a]] here", []Wikilink{{Line: 1, Col: 5, Target: "a"}}},
		{"[[a|b]]", []Wikilink{{Line: 1, Col: 1, Target: "a", Label: "b"}}},
		{`| [[a\|b]] |`, []Wikilink{{Line: 1, Col: 3, Target: "a", Label: "b"}}},
		{"[[a#h]]", []Wikilink{{Line: 1, Col: 1, Target: "a", Fragment: "h"}}},
		{"[[a#^x]]", []Wikilink{{Line: 1, Col: 1, Target: "a", Fragment: "^x"}}},
		{"[[#h]]", []Wikilink{{Line: 1, Col: 1, Fragment: "h"}}},
		{"![[a]]", []Wikilink{{Line: 1, Col: 2, Target: "a", Embed: true}}},
		{"[[folder/a]]", []Wikilink{{Line: 1, Col: 1, Target: "folder/a"}}},
		{"[[a.md]] [[b.MD]]", []Wikilink{{Line: 1, Col: 1, Target: "a"}, {Line: 1, Col: 10, Target: "b"}}},
		{"[[ a ]]", []Wikilink{{Line: 1, Col: 1, Target: "a"}}},
		{"[[A]]", []Wikilink{{Line: 1, Col: 1, Target: "A"}}},
		{"ação [[nota]]", []Wikilink{{Line: 1, Col: 6, Target: "nota"}}},
		{"[[img.png]]", []Wikilink{{Line: 1, Col: 1, Target: "img.png"}}},
		{"[[a|b|c]]", []Wikilink{{Line: 1, Col: 1, Target: "a", Label: "b|c"}}},
	}
	for _, c := range cases {
		if got := Wikilinks(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: got %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestWikilinksIgnoredAndDropped(t *testing.T) {
	src := "---\nrel: \"[[in-frontmatter]]\"\n---\n" +
		"```\n[[in-backticks]]\n```\n" +
		"~~~~\n[[in-tildes]]\n~~~\n[[still-in-tildes]]\n~~~~\n" +
		"`[[one]]` and ``a [[two]] ` b`` and ` `` ` [[real]]\n" +
		"[[]] [[a] \n[[a\n]]\n[[x[y]]\n" +
		"[[after]]\n"
	got := Wikilinks(src)
	var targets []string
	for _, w := range got {
		targets = append(targets, w.Target)
	}
	if !reflect.DeepEqual(targets, []string{"real", "after"}) {
		t.Fatalf("targets %v (%+v)", targets, got)
	}
	if got[0].Line != 12 || got[1].Line != 17 {
		t.Fatalf("lines %d %d", got[0].Line, got[1].Line)
	}
}

func TestWikilinksCRLFAndUnterminatedCode(t *testing.T) {
	src := "---\r\ntitle: x\r\n---\r\nline [[a]]\r\n` open [[b]]\r\n"
	got := Wikilinks(src)
	if len(got) != 2 || got[0].Line != 4 || got[0].Target != "a" || got[1].Target != "b" {
		t.Fatalf("%+v", got)
	}
}

func TestHeadings(t *testing.T) {
	src := "---\n# not a heading (yaml comment)\ntitle: x\n---\n" +
		"# Title\n\n```go\n# inside fence\n```\n" +
		"## Sub section ##\n   ### Indented\n    #### code block indent\n" +
		"#no-space\n####### seven\n#\n## Ação & Coisas\n"
	got := Headings(src)
	want := []Heading{
		{Line: 5, Level: 1, Text: "Title", Slug: "title"},
		{Line: 10, Level: 2, Text: "Sub section", Slug: "sub-section"},
		{Line: 11, Level: 3, Text: "Indented", Slug: "indented"},
		{Line: 16, Level: 2, Text: "Ação & Coisas", Slug: "ação-coisas"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestHeadingKeepsHashWithoutSpace(t *testing.T) {
	got := Headings("# C# notes\n# Issue#42\n")
	if len(got) != 2 || got[0].Text != "C# notes" || got[1].Text != "Issue#42" {
		t.Fatalf("%+v", got)
	}
}

func TestSplitFrontmatter(t *testing.T) {
	fm, ok, err := SplitFrontmatter("---\ntitle: x\ntags: [a]\n...\nbody\n")
	if err != nil || !ok || fm.YAML != "title: x\ntags: [a]" || fm.EndLine != 4 {
		t.Fatalf("%+v %v %v", fm, ok, err)
	}
	fm, ok, err = SplitFrontmatter("---\r\ntitle: x\r\n---\r\n")
	if err != nil || !ok || fm.YAML != "title: x" || fm.EndLine != 3 {
		t.Fatalf("crlf: %+v %v %v", fm, ok, err)
	}
	if _, ok, err := SplitFrontmatter("# no frontmatter\n---\n"); ok || err != nil {
		t.Fatalf("none: %v %v", ok, err)
	}
	if _, ok, err := SplitFrontmatter("---\ntitle: x\n"); !ok || err != ErrUnterminatedFrontmatter {
		t.Fatalf("unterminated: %v %v", ok, err)
	}
	// An unterminated frontmatter masks nothing: its lines are body.
	if got := Wikilinks("---\n[[a]]\n"); len(got) != 1 {
		t.Fatalf("unterminated mask: %+v", got)
	}
}

func TestSlugifyAndNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"SQLite adiado": "sqlite-adiado", "  Ação!  ": "ação", "!!!": "", "a--b__c": "a-b-c", "Go 1.27": "go-1-27",
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
	if got := NormalizeForSearch("Ação Útil"); got != "acao util" {
		t.Errorf("NormalizeForSearch = %q", got)
	}
}

func TestLines(t *testing.T) {
	if got := Lines("a\r\nb\nc"); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("%q", got)
	}
	if Lines("") != nil {
		t.Fatal("empty")
	}
}
