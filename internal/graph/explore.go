package graph

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	DefaultExplainLimit = 5
	// MaxExplainNeighbors caps neighbors per direction per explained node.
	MaxExplainNeighbors = 50
	DefaultPathMaxDepth = 6
)

// Direction tells whether an edge leaves (out) or enters (in) the node it is
// listed under, or — in a path step — whether the hop follows the stored edge
// direction (forward) or walks it backwards (backward).
type Direction string

const (
	DirOut      Direction = "out"
	DirIn       Direction = "in"
	DirForward  Direction = "forward"
	DirBackward Direction = "backward"
)

// Neighbor is one edge incident to an explained node plus the node on the
// other end, keeping the edge provenance (source/confidence).
type Neighbor struct {
	Node       Node      `json:"node"`
	Edge       EdgeType  `json:"edge"`
	Direction  Direction `json:"direction"`
	Source     string    `json:"source"`
	Confidence float64   `json:"confidence"`
}

type Explanation struct {
	Node         Node       `json:"node"`
	Outgoing     []Neighbor `json:"outgoing"`
	Incoming     []Neighbor `json:"incoming"`
	OutDegree    int        `json:"out_degree"`
	InDegree     int        `json:"in_degree"`
	TruncatedOut int        `json:"truncated_out,omitempty"`
	TruncatedIn  int        `json:"truncated_in,omitempty"`
	// UnresolvedOut counts outgoing calls whose target is not a node in the
	// index (e.g. stdlib or unresolved method receivers). Other edge types
	// with a missing endpoint are skipped silently, on both sides.
	UnresolvedOut int `json:"unresolved_out,omitempty"`
}

// Every step but the first carries the edge used to reach it from the
// previous step.
type PathStep struct {
	Node       Node      `json:"node"`
	Edge       EdgeType  `json:"edge,omitempty"`
	Direction  Direction `json:"direction,omitempty"`
	Source     string    `json:"source,omitempty"`
	Confidence float64   `json:"confidence,omitempty"`
}

var (
	ErrNodeNotFound = errors.New("node not found in graph")
	ErrNoPath       = errors.New("no path between nodes")
)

// edgeUsable drops low-confidence call edges, matching Callers/Impact.
func edgeUsable(e *Edge) bool {
	return e.Type != EdgeCalls || e.Confidence >= MinCallsConfidence
}

// Low-confidence call edges are omitted, as in callers/impact.
func (s *Store) Explain(query string, limit int) ([]Explanation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = DefaultExplainLimit
	}
	ids := s.resolveNodeIDs(query)
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrNodeNotFound, strings.TrimSpace(query))
	}
	if len(ids) > limit {
		ids = ids[:limit]
	}
	byID := s.nodesByID()
	want := make(map[string]*Explanation, len(ids))
	out := make([]Explanation, len(ids))
	for i, id := range ids {
		out[i].Node = byID[id]
		want[id] = &out[i]
	}
	for i := range s.index.Edges {
		e := &s.index.Edges[i]
		if !edgeUsable(e) {
			continue
		}
		if ex, ok := want[e.SourceID]; ok {
			if n, ok := byID[e.TargetID]; ok {
				ex.Outgoing = append(ex.Outgoing, neighborOf(&n, e, DirOut))
			} else if e.Type == EdgeCalls {
				ex.UnresolvedOut++
			}
		}
		if ex, ok := want[e.TargetID]; ok {
			if n, ok := byID[e.SourceID]; ok {
				ex.Incoming = append(ex.Incoming, neighborOf(&n, e, DirIn))
			}
		}
	}
	for i := range out {
		ex := &out[i]
		ex.Outgoing = uniqueNeighbors(ex.Outgoing)
		ex.Incoming = uniqueNeighbors(ex.Incoming)
		ex.OutDegree, ex.InDegree = len(ex.Outgoing), len(ex.Incoming)
		ex.Outgoing, ex.TruncatedOut = capNeighbors(ex.Outgoing)
		ex.Incoming, ex.TruncatedIn = capNeighbors(ex.Incoming)
		if ex.Outgoing == nil {
			ex.Outgoing = []Neighbor{}
		}
		if ex.Incoming == nil {
			ex.Incoming = []Neighbor{}
		}
	}
	return out, nil
}

func neighborOf(n *Node, e *Edge, d Direction) Neighbor {
	return Neighbor{Node: *n, Edge: e.Type, Direction: d, Source: e.Source, Confidence: e.Confidence}
}

func sortNeighbors(ns []Neighbor) {
	sort.SliceStable(ns, func(i, j int) bool {
		if ns[i].Edge != ns[j].Edge {
			return ns[i].Edge < ns[j].Edge
		}
		if ns[i].Node.ID != ns[j].Node.ID {
			return ns[i].Node.ID < ns[j].Node.ID
		}
		if ns[i].Confidence != ns[j].Confidence {
			return ns[i].Confidence > ns[j].Confidence
		}
		return ns[i].Source < ns[j].Source
	})
}

// uniqueNeighbors sorts and collapses repeated (edge type, node) pairs — e.g.
// a function called twice — keeping the highest confidence (ties: lowest
// source name), so the kept provenance is deterministic.
func uniqueNeighbors(ns []Neighbor) []Neighbor {
	sortNeighbors(ns)
	out := ns[:0]
	for i := range ns {
		n := &ns[i]
		if last := len(out) - 1; last >= 0 && out[last].Edge == n.Edge && out[last].Node.ID == n.Node.ID {
			continue // sorted: first of the run has the best confidence
		}
		out = append(out, *n)
	}
	return out
}

func capNeighbors(ns []Neighbor) (kept []Neighbor, truncated int) {
	if len(ns) <= MaxExplainNeighbors {
		return ns, 0
	}
	return ns[:MaxExplainNeighbors], len(ns) - MaxExplainNeighbors
}

type adjacency struct {
	to   string
	edge *Edge
	dir  Direction
}

// ShortestPath returns the shortest path (fewest hops) between any node matching from
// and any node matching to, walking edges in both directions. Each step says
// whether it follows the stored edge (forward) or walks it backwards. Ties are
// broken by node ID, so the result is deterministic.
func (s *Store) ShortestPath(from, to string, maxDepth int) ([]PathStep, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if maxDepth <= 0 {
		maxDepth = DefaultPathMaxDepth
	}
	srcIDs := s.resolveNodeIDs(from)
	if len(srcIDs) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrNodeNotFound, strings.TrimSpace(from))
	}
	dstIDs := s.resolveNodeIDs(to)
	if len(dstIDs) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrNodeNotFound, strings.TrimSpace(to))
	}
	byID := s.nodesByID()
	dst := make(map[string]bool, len(dstIDs))
	for _, id := range dstIDs {
		dst[id] = true
	}
	sort.Strings(srcIDs)
	for _, id := range srcIDs {
		if dst[id] {
			return []PathStep{{Node: byID[id]}}, nil
		}
	}

	adj := s.adjacencyList(byID)
	prev := map[string]adjacency{}
	visited := map[string]bool{}
	for _, id := range srcIDs {
		visited[id] = true
	}
	frontier := srcIDs
	for depth := 0; depth < maxDepth && len(frontier) > 0; depth++ {
		var next []string
		for _, cur := range frontier {
			for _, a := range adj[cur] {
				if visited[a.to] {
					continue
				}
				visited[a.to] = true
				prev[a.to] = adjacency{to: cur, edge: a.edge, dir: a.dir}
				if dst[a.to] {
					return buildPath(byID, prev, a.to), nil
				}
				next = append(next, a.to)
			}
		}
		frontier = next
	}
	return nil, fmt.Errorf("%w: %q → %q (up to %d hops)", ErrNoPath, strings.TrimSpace(from), strings.TrimSpace(to), maxDepth)
}

// adjacencyList builds an undirected view of usable edges. Edges with an
// endpoint missing from the index (unresolved call targets) are skipped so a
// path never walks through a phantom node.
func (s *Store) adjacencyList(byID map[string]Node) map[string][]adjacency {
	adj := map[string][]adjacency{}
	for i := range s.index.Edges {
		e := &s.index.Edges[i]
		if !edgeUsable(e) || e.SourceID == e.TargetID {
			continue
		}
		if _, ok := byID[e.SourceID]; !ok {
			continue
		}
		if _, ok := byID[e.TargetID]; !ok {
			continue
		}
		adj[e.SourceID] = append(adj[e.SourceID], adjacency{to: e.TargetID, edge: e, dir: DirForward})
		adj[e.TargetID] = append(adj[e.TargetID], adjacency{to: e.SourceID, edge: e, dir: DirBackward})
	}
	for id := range adj {
		list := adj[id]
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].to != list[j].to {
				return list[i].to < list[j].to
			}
			if list[i].dir != list[j].dir {
				return list[i].dir == DirForward
			}
			return list[i].edge.Type < list[j].edge.Type
		})
	}
	return adj
}

func buildPath(byID map[string]Node, prev map[string]adjacency, end string) []PathStep {
	var rev []PathStep
	cur := end
	for {
		p, ok := prev[cur]
		if !ok {
			rev = append(rev, PathStep{Node: byID[cur]})
			break
		}
		rev = append(rev, PathStep{
			Node: byID[cur], Edge: p.edge.Type, Direction: p.dir,
			Source: p.edge.Source, Confidence: p.edge.Confidence,
		})
		cur = p.to
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

func (s *Store) nodesByID() map[string]Node {
	m := make(map[string]Node, len(s.index.Nodes))
	for _, n := range s.index.Nodes {
		m[n.ID] = n
	}
	return m
}

// resolveNodeIDs maps a free-form query to node IDs, most specific tier first:
// exact ID, then exact title, then function suffix match, then file/package.
// Results are sorted for determinism.
func (s *Store) resolveNodeIDs(query string) []string {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	for _, n := range s.index.Nodes {
		if n.ID == q {
			return []string{n.ID}
		}
	}
	var ids []string
	seen := map[string]bool{}
	for _, n := range s.index.Nodes {
		if n.Title == q && !seen[n.ID] {
			seen[n.ID] = true
			ids = append(ids, n.ID)
		}
	}
	if len(ids) == 0 {
		ids = s.findFunctionIDs(q)
	}
	if len(ids) == 0 {
		ids = dedupe(s.findPackageOrFileIDs(q))
	}
	sort.Strings(ids)
	return ids
}

func dedupe(ids []string) []string {
	seen := map[string]bool{}
	out := ids[:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
