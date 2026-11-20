package secplay

import (
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/graph"
)

func TestGraphNodesEmitsPlaybooksAndMirrors(t *testing.T) {
	nodes, edges, err := GraphNodes("")
	if err != nil {
		t.Fatalf("GraphNodes: %v", err)
	}
	c := mustLoad(t)
	if len(nodes) != len(c.Red)+len(c.Blue) {
		t.Errorf("nodes=%d, want %d", len(nodes), len(c.Red)+len(c.Blue))
	}
	for _, n := range nodes {
		if n.Type != graph.NodePlaybook {
			t.Errorf("node %s has type %s, want playbook", n.ID, n.Type)
		}
	}
	// One mirror edge per red playbook with a mirror; each resolves to a node.
	ids := map[string]bool{}
	for _, n := range nodes {
		ids[n.ID] = true
	}
	for _, e := range edges {
		if e.Type != graph.EdgeMirror {
			t.Errorf("edge %s->%s type %s, want mirror", e.SourceID, e.TargetID, e.Type)
		}
		if !ids[e.SourceID] || !ids[e.TargetID] {
			t.Errorf("mirror edge %s->%s references a missing node", e.SourceID, e.TargetID)
		}
	}
	if len(edges) != len(c.Red) {
		t.Errorf("mirror edges=%d, want %d (one per red playbook)", len(edges), len(c.Red))
	}
}

// TestGraphBuildIncludesPlaybooks proves the node source is wired into the graph
// build: an index of an empty tree still carries the playbook nodes and a
// resolvable red->blue mirror edge (so tb graph path links them).
func TestGraphBuildIncludesPlaybooks(t *testing.T) {
	idx, err := graph.BuildFromRootOpts(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var haveRed, haveBlue, haveMirror bool
	for _, n := range idx.Nodes {
		switch n.ID {
		case "playbook:red-sqli":
			haveRed = true
		case "playbook:blue-sqli":
			haveBlue = true
		}
	}
	for _, e := range idx.Edges {
		if e.Type == graph.EdgeMirror && e.SourceID == "playbook:red-sqli" && e.TargetID == "playbook:blue-sqli" {
			haveMirror = true
		}
	}
	if !haveRed || !haveBlue {
		t.Errorf("playbook nodes missing from build: red=%v blue=%v", haveRed, haveBlue)
	}
	if !haveMirror {
		t.Error("red-sqli -> blue-sqli mirror edge missing from build")
	}
}
