package secplay

import (
	"github.com/leonardo-moraes360/toolbelt/internal/graph"
)

func init() {
	graph.RegisterNodeSource(GraphNodes)
}

// GraphNodes contributes the AppSec playbook corpus to the code graph: one
// `playbook:<name>` node per playbook and a `mirror` edge from each red
// playbook to its blue counterpart, so `tb graph path` / `tb graph explain`
// link attack and defense. root is unused — the corpus is embedded.
func GraphNodes(string) ([]graph.Node, []graph.Edge, error) {
	c, err := Load()
	if err != nil {
		return nil, nil, err
	}
	var nodes []graph.Node
	var edges []graph.Edge

	addNodes := func(m map[string]*Playbook) {
		for _, name := range sortedNames(m) {
			pb := m[name]
			nodes = append(nodes, graph.Node{
				ID:         playbookNodeID(pb.Name),
				Type:       graph.NodePlaybook,
				Path:       pb.Class,
				Title:      pb.Name,
				Package:    pb.Area,
				Source:     "manual",
				Confidence: 1.0,
			})
		}
	}
	addNodes(c.Red)
	addNodes(c.Blue)

	// One mirror edge per pair (red -> blue); traversal is bidirectional, so a
	// single edge links both directions for path/explain.
	for _, name := range sortedNames(c.Red) {
		pb := c.Red[name]
		if pb.Mirror == "" {
			continue
		}
		edges = append(edges, graph.Edge{
			SourceID:   playbookNodeID(pb.Name),
			TargetID:   playbookNodeID(pb.Mirror),
			Type:       graph.EdgeMirror,
			Source:     "manual",
			Confidence: 1.0,
		})
	}
	return nodes, edges, nil
}

func playbookNodeID(name string) string { return "playbook:" + name }
