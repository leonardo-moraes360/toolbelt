package graph

import (
	"reflect"
	"testing"
)

func searchStore(nodes ...Node) *Store {
	s := NewStore("")
	s.index = &Index{Nodes: nodes}
	return s
}

func hitIDs(ns []Node) []string {
	out := make([]string, len(ns))
	for i := range ns {
		out[i] = ns[i].ID
	}
	return out
}

func TestTokenize(t *testing.T) {
	cases := map[string][]string{
		"numeroBr":            {"numerobr", "numero", "br"},
		"parseHTTPRequest":    {"parsehttprequest", "parse", "http", "request"},
		"internal/graph/x.go": {"internal", "graph", "x", "go"},
		"snake_case_name":     {"snake", "case", "name"},
		"Problemas novos":     {"problemas", "novos"},
		"v2api":               {"v2api", "v", "2", "api"},
		"listAPIs":            {"listapis", "list", "apis"},
		"IDs":                 {"ids"},
		"ação":              {"ação"},
		"":                    nil,
	}
	for in, want := range cases {
		if got := tokenize(in); !reflect.DeepEqual(got, want) {
			t.Errorf("tokenize(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSearchExactTitleRanksFirst(t *testing.T) {
	s := searchStore(
		Node{ID: "func:a.ScaleAll", Title: "ScaleAll", Path: "a/scale.go"},
		Node{ID: "file:a/scale.go", Title: "scale.go", Path: "a/scale.go"},
		Node{ID: "func:a.Scale", Title: "Scale", Path: "a/x.go"},
	)
	got := hitIDs(s.Search("Scale", 10))
	if len(got) != 3 || got[0] != "func:a.Scale" {
		t.Fatalf("exact title should rank first, got %v", got)
	}
}

func TestSearchMultiTermAnyOrder(t *testing.T) {
	s := searchStore(
		Node{ID: "func:graph.appendRenderEdges", Title: "appendRenderEdges", Path: "internal/graph/r.go", Package: "graph"},
		Node{ID: "func:graph.appendEdge", Title: "appendEdge", Path: "internal/graph/e.go", Package: "graph"},
		Node{ID: "func:x.render", Title: "render", Path: "x/r.go", Package: "x"},
	)
	// The old substring search returned nothing for this query.
	got := hitIDs(s.Search("render edges", 10))
	if len(got) == 0 || got[0] != "func:graph.appendRenderEdges" {
		t.Fatalf("node matching both terms should rank first, got %v", got)
	}
}

func TestSearchKeepsSubstringMatches(t *testing.T) {
	s := searchStore(
		Node{ID: "func:svc.Scale", Title: "Scale", Path: "service/scale.go"},
		Node{ID: "func:other.Run", Title: "Run", Path: "other/run.go"},
	)
	for _, q := range []string{"Sca", "service/", "func:svc"} {
		if got := hitIDs(s.Search(q, 10)); !reflect.DeepEqual(got, []string{"func:svc.Scale"}) {
			t.Errorf("Search(%q) = %v, want substring match kept", q, got)
		}
	}
}

func TestSearchTitleOutweighsPath(t *testing.T) {
	s := searchStore(
		Node{ID: "func:p.helper", Title: "helper", Path: "drift/helper.go"},
		Node{ID: "func:p.runEngine", Title: "DriftEngine", Path: "p/main.go"},
		Node{ID: "func:q.other", Title: "other", Path: "q/other.go"},
	)
	// "drift" is not a substring of either ID ("func:p.helper", "func:p.runEngine")
	// but is of both haystacks via path/title, so both sit in the substring
	// tier and BM25 field weight decides the order.
	got := hitIDs(s.Search("drift", 10))
	if len(got) != 2 || got[0] != "func:p.runEngine" {
		t.Fatalf("title match should outrank path match, got %v", got)
	}
}

func TestSearchExactTitleBeatsHigherScore(t *testing.T) {
	s := searchStore(
		Node{ID: "func:a.parse", Title: "parse", Path: "very/long/nested/path/for/this/node/file.go"},
		Node{ID: "func:a.parseParseParse", Title: "parse_parse_parse", Path: "parse/parse.go", Package: "parse"},
	)
	if got := hitIDs(s.Search("parse", 10)); got[0] != "func:a.parse" {
		t.Fatalf("exact title must outrank any score, got %v", got)
	}
}

func TestSearchSubstringTierSurvivesLimit(t *testing.T) {
	// One node matches the old substring search; many match only the terms.
	nodes := []Node{{ID: "func:z.renderEdges", Title: "zrenderEdges"}}
	for i := 0; i < 30; i++ {
		nodes = append(nodes, Node{ID: "func:a.render" + string(rune('a'+i%26)), Title: "render edges"})
	}
	s := searchStore(nodes...)
	got := hitIDs(s.Search("renderedges", 5))
	if len(got) == 0 || got[0] != "func:z.renderEdges" {
		t.Fatalf("substring match must stay within limit, got %v", got)
	}
}

func TestSearchStatsCacheFollowsIndex(t *testing.T) {
	s := searchStore(Node{ID: "a", Title: "alpha"})
	if len(s.Search("alpha", 10)) != 1 {
		t.Fatal("expected alpha")
	}
	s.index = &Index{Nodes: []Node{{ID: "b", Title: "beta"}}}
	if got := hitIDs(s.Search("beta", 10)); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("stats should be rebuilt for new index, got %v", got)
	}
}

func TestSearchDeterministicAndLimited(t *testing.T) {
	s := searchStore(
		Node{ID: "c", Title: "foo"},
		Node{ID: "a", Title: "foo"},
		Node{ID: "b", Title: "foo"},
	)
	if got := hitIDs(s.Search("foo", 2)); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("ties should break by ID and respect limit, got %v", got)
	}
	if s.Search("", 10) != nil || s.Search("foo", 0) != nil || s.Search("nomatch", 10) != nil {
		t.Fatal("empty query, zero limit and no match should return nil")
	}
	if NewStore("").Search("foo", 10) != nil {
		t.Fatal("store without index should return nil")
	}
}

func BenchmarkSearch5k(b *testing.B) {
	nodes := make([]Node, 5000)
	for i := range nodes {
		nodes[i] = Node{
			ID:      "func:pkg.handleRequestNumber" + string(rune('a'+i%26)),
			Title:   "handleRequestNumber" + string(rune('a'+i%26)),
			Path:    "internal/pkg/sub/file_name.go",
			Package: "pkg",
		}
	}
	s := searchStore(nodes...)
	b.ResetTimer()
	for b.Loop() {
		s.Search("request handler", 20)
	}
}
