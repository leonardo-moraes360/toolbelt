package graph

import (
	"errors"
	"fmt"
	"testing"
)

func exploreStore(nodes []Node, edges []Edge) *Store {
	s := NewStore("")
	s.index = &Index{Nodes: nodes, Edges: edges}
	return s
}

func fn(id, title string) Node {
	return Node{ID: id, Type: NodeFunction, Title: title, Source: "extracted", Confidence: 1}
}

func file(id, path string) Node {
	return Node{ID: id, Type: NodeFile, Title: path, Path: path, Source: "extracted", Confidence: 1}
}

func edge(src, dst string, t EdgeType, conf float64) Edge {
	return Edge{SourceID: src, TargetID: dst, Type: t, Source: "extracted", Confidence: conf}
}

func sampleExploreStore() *Store {
	return exploreStore(
		[]Node{
			file("file:a.go", "a.go"),
			file("file:b.go", "b.go"),
			fn("func:a.Handler", "Handler"),
			fn("func:a.Validate", "Validate"),
			fn("func:b.Store", "Store"),
			fn("func:b.Guess", "Guess"),
			fn("func:c.Isolated", "Isolated"),
		},
		[]Edge{
			edge("file:a.go", "func:a.Handler", EdgeDefines, 1),
			edge("file:a.go", "func:a.Validate", EdgeDefines, 1),
			edge("file:b.go", "func:b.Store", EdgeDefines, 1),
			edge("file:b.go", "func:b.Guess", EdgeDefines, 1),
			edge("func:a.Handler", "func:a.Validate", EdgeCalls, 1),
			edge("func:a.Validate", "func:b.Store", EdgeCalls, 0.75),
			edge("func:a.Handler", "func:b.Guess", EdgeCalls, 0.4), // below MinCallsConfidence
			edge("file:a.go", "file:b.go", EdgeImports, 1),
		},
	)
}

func TestExplainListsEdgesWithProvenance(t *testing.T) {
	s := sampleExploreStore()
	got, err := s.Explain("Validate", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Node.ID != "func:a.Validate" {
		t.Fatalf("explain matched %+v", got)
	}
	ex := got[0]
	if ex.OutDegree != 1 || ex.Outgoing[0].Node.ID != "func:b.Store" {
		t.Fatalf("outgoing = %+v", ex.Outgoing)
	}
	if o := ex.Outgoing[0]; o.Edge != EdgeCalls || o.Direction != DirOut || o.Source != "extracted" || o.Confidence != 0.75 {
		t.Fatalf("outgoing provenance = %+v", o)
	}
	in := map[string]EdgeType{}
	for _, n := range ex.Incoming {
		if n.Direction != DirIn {
			t.Fatalf("incoming direction = %s", n.Direction)
		}
		in[n.Node.ID] = n.Edge
	}
	if in["func:a.Handler"] != EdgeCalls || in["file:a.go"] != EdgeDefines || len(in) != 2 {
		t.Fatalf("incoming = %+v", ex.Incoming)
	}
}

func TestExplainDropsLowConfidenceCalls(t *testing.T) {
	s := sampleExploreStore()
	got, err := s.Explain("Handler", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range got[0].Outgoing {
		if n.Node.ID == "func:b.Guess" {
			t.Fatalf("low-confidence call leaked: %+v", n)
		}
	}
	if got[0].OutDegree != 1 {
		t.Fatalf("out degree = %d, want 1", got[0].OutDegree)
	}
}

func TestExplainExactIDWinsAndNotFound(t *testing.T) {
	s := sampleExploreStore()
	got, err := s.Explain("file:a.go", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Node.ID != "file:a.go" {
		t.Fatalf("exact id = %+v", got)
	}
	if _, err := s.Explain("Nope", 0); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("err = %v, want ErrNodeNotFound", err)
	}
	if _, err := s.Explain("   ", 0); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("blank err = %v", err)
	}
}

func TestExplainTruncatesNeighbors(t *testing.T) {
	nodes := []Node{fn("func:hub", "Hub")}
	var edges []Edge
	for i := 0; i < MaxExplainNeighbors+7; i++ {
		id := fmt.Sprintf("func:caller%03d", i)
		nodes = append(nodes, fn(id, fmt.Sprintf("Caller%03d", i)))
		edges = append(edges, edge(id, "func:hub", EdgeCalls, 1))
	}
	got, err := exploreStore(nodes, edges).Explain("Hub", 1)
	if err != nil {
		t.Fatal(err)
	}
	ex := got[0]
	if len(ex.Incoming) != MaxExplainNeighbors || ex.TruncatedIn != 7 || ex.InDegree != MaxExplainNeighbors+7 {
		t.Fatalf("incoming=%d truncated=%d degree=%d", len(ex.Incoming), ex.TruncatedIn, ex.InDegree)
	}
	if ex.Outgoing == nil {
		t.Fatal("outgoing must be empty slice, not nil (stable JSON)")
	}
}

func TestPathFindsShortestWithDirections(t *testing.T) {
	s := sampleExploreStore()
	steps, err := s.ShortestPath("Handler", "Store", 0)
	if err != nil {
		t.Fatal(err)
	}
	ids := pathIDs(steps)
	want := []string{"func:a.Handler", "func:a.Validate", "func:b.Store"}
	if fmt.Sprint(ids) != fmt.Sprint(want) {
		t.Fatalf("path = %v, want %v", ids, want)
	}
	if steps[0].Edge != "" || steps[1].Direction != DirForward || steps[2].Confidence != 0.75 {
		t.Fatalf("steps = %+v", steps)
	}

	back, err := s.ShortestPath("Store", "Handler", 0)
	if err != nil {
		t.Fatal(err)
	}
	if back[1].Direction != DirBackward {
		t.Fatalf("reverse walk direction = %s", back[1].Direction)
	}
}

func TestPathIgnoresLowConfidenceCalls(t *testing.T) {
	s := sampleExploreStore()
	steps, err := s.ShortestPath("Handler", "Guess", 0)
	if err != nil {
		t.Fatal(err)
	}
	// The direct 0.4 call is skipped; route goes through files instead.
	for i := 1; i < len(steps); i++ {
		if steps[i].Edge == EdgeCalls && steps[i].Confidence < MinCallsConfidence {
			t.Fatalf("used low-confidence edge: %+v", steps)
		}
	}
	if len(steps) < 3 {
		t.Fatalf("path too short, low-confidence edge used? %v", pathIDs(steps))
	}
}

func TestPathErrors(t *testing.T) {
	s := sampleExploreStore()
	if _, err := s.ShortestPath("Handler", "Isolated", 0); !errors.Is(err, ErrNoPath) {
		t.Fatalf("err = %v, want ErrNoPath", err)
	}
	if _, err := s.ShortestPath("Missing", "Handler", 0); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("err = %v, want ErrNodeNotFound", err)
	}
	if _, err := s.ShortestPath("Handler", "Store", 1); !errors.Is(err, ErrNoPath) {
		t.Fatalf("max depth 1: err = %v, want ErrNoPath", err)
	}
	same, err := s.ShortestPath("Handler", "func:a.Handler", 0)
	if err != nil || len(same) != 1 {
		t.Fatalf("same node path = %v, %v", same, err)
	}
}

func TestPathDeterministic(t *testing.T) {
	s := sampleExploreStore()
	first, err := s.ShortestPath("Handler", "Store", 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		again, _ := s.ShortestPath("Handler", "Store", 0)
		if fmt.Sprint(pathIDs(again)) != fmt.Sprint(pathIDs(first)) {
			t.Fatalf("non-deterministic: %v vs %v", pathIDs(again), pathIDs(first))
		}
	}
}

func TestPathOnRealIndex(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root+"/mathutil/math.go", `package mathutil
func Scale(v, factor int) int { return v * factor }
`)
	mustWrite(t, root+"/service/billing.go", `package service
func Total(price, qty int) int { return Scale(price, qty) }
func Discounted(price, qty, discount int) int { return Total(price, qty) - discount }
func Scale(v, f int) int { return v * f }
`)
	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	s := exploreStore(idx.Nodes, idx.Edges)
	steps, err := s.ShortestPath("Discounted", "Total", 0)
	if err != nil {
		t.Fatalf("path: %v (edges=%d)", err, len(idx.Edges))
	}
	if len(steps) != 2 {
		t.Fatalf("expected direct call Discounted→Total, got %v", pathIDs(steps))
	}
	ex, err := s.Explain("Total", 0)
	if err != nil || len(ex) == 0 {
		t.Fatalf("explain Total: %v %v", ex, err)
	}
}

func pathIDs(steps []PathStep) []string {
	out := make([]string, len(steps))
	for i, st := range steps {
		out[i] = st.Node.ID
	}
	return out
}

func TestDanglingEdgesSkipped(t *testing.T) {
	s := exploreStore(
		[]Node{fn("func:a.A", "A"), fn("func:b.B", "B")},
		[]Edge{
			edge("func:a.A", "func:s.Ghost", EdgeCalls, 0.8),
			edge("func:b.B", "func:s.Ghost", EdgeCalls, 0.8),
			edge("func:a.A", "func:len", EdgeCalls, 0.8),
		},
	)
	if steps, err := s.ShortestPath("A", "B", 0); !errors.Is(err, ErrNoPath) {
		t.Fatalf("path through phantom node: %v %v", pathIDs(steps), err)
	}
	ex, err := s.Explain("A", 0)
	if err != nil {
		t.Fatal(err)
	}
	if ex[0].UnresolvedOut != 2 || len(ex[0].Outgoing) != 0 {
		t.Fatalf("unresolved=%d outgoing=%+v", ex[0].UnresolvedOut, ex[0].Outgoing)
	}
}

func TestExplainCollapsesRepeatedCalls(t *testing.T) {
	s := exploreStore(
		[]Node{fn("func:a.A", "A"), fn("func:a.B", "B")},
		[]Edge{edge("func:a.A", "func:a.B", EdgeCalls, 0.7), edge("func:a.A", "func:a.B", EdgeCalls, 0.9)},
	)
	ex, err := s.Explain("A", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(ex[0].Outgoing) != 1 || ex[0].Outgoing[0].Confidence != 0.9 || ex[0].OutDegree != 1 {
		t.Fatalf("outgoing = %+v", ex[0].Outgoing)
	}
}

func TestResolvePrecedenceExactIDOverTitle(t *testing.T) {
	// A node whose title equals another node's ID: exact ID must win.
	s := exploreStore(
		[]Node{fn("Run", "Other"), fn("func:x.Run", "Run"), fn("func:y.Run", "Run")},
		nil,
	)
	ex, err := s.Explain("Run", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(ex) != 1 || ex[0].Node.ID != "Run" {
		t.Fatalf("exact ID should win over title, got %+v", ex)
	}
	// Without an ID match, every exact-title match is returned, sorted.
	ex, err = s.Explain("func:z", 0)
	if !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("err = %v", err)
	}
	s2 := exploreStore([]Node{fn("func:y.Run", "Run"), fn("func:x.Run", "Run")}, nil)
	ex, err = s2.Explain("Run", 0)
	if err != nil || len(ex) != 2 || ex[0].Node.ID != "func:x.Run" {
		t.Fatalf("title tier = %+v, %v", ex, err)
	}
}

func TestUnresolvedCountsOnlyCalls(t *testing.T) {
	s := exploreStore(
		[]Node{file("file:a.go", "a.go")},
		[]Edge{edge("file:a.go", "pkg:fmt", EdgeImports, 1), edge("file:a.go", "file:missing.md", EdgeLinks, 1)},
	)
	ex, err := s.Explain("file:a.go", 0)
	if err != nil {
		t.Fatal(err)
	}
	if ex[0].UnresolvedOut != 0 {
		t.Fatalf("non-call edges counted as unresolved: %d", ex[0].UnresolvedOut)
	}
}
