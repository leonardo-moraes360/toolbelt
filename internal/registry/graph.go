package registry

import (
	"context"
	"fmt"
	"os"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/graph"
	"github.com/leonardo-moraes360/toolbelt/internal/mem"
)

func init() {
	RegisterGraphTools()
}

func RegisterGraphTools() {
	Register(&Tool{
		Name:        "tb_graph_search",
		Description: "Searches the code graph (on demand; requires TB_GRAPH=on)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {Type: "string", Description: "Query"},
				"limit": {Type: "integer", Description: "Maximum results", Default: 20},
			},
			Required: []string{"query"},
		},
		Handler: handleGraphSearch,
	})
	Register(&Tool{
		Name:        "tb_graph_callers",
		Description: "Finds callers of a function in the graph",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"function": {Type: "string", Description: "Function name"},
				"depth":    {Type: "integer", Description: "Depth", Default: 1},
			},
			Required: []string{"function"},
		},
		Handler: handleGraphCallers,
	})
	Register(&Tool{
		Name:        "tb_graph_deps",
		Description: "Lists module/file dependencies in the graph",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"module":     {Type: "string", Description: "Module or package"},
				"file":       {Type: "string", Description: "File"},
				"transitive": {Type: "boolean", Description: "Includes transitive dependencies", Default: false},
			},
		},
		Handler: handleGraphDeps,
	})
	Register(&Tool{
		Name:        "tb_graph_impact",
		Description: "Analyzes the impact of changes to a file (parity with tb graph impact)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"file":  {Type: "string", Description: "Modified file"},
				"depth": {Type: "integer", Description: "Depth", Default: 2},
			},
			Required: []string{"file"},
		},
		Handler: handleGraphImpact,
	})
	Register(&Tool{
		Name:        "tb_graph_explain",
		Description: "Explains a graph node: incoming/outgoing edges with provenance (parity with tb graph explain)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"node":  {Type: "string", Description: "ID, title, function, or file"},
				"limit": {Type: "integer", Description: "Maximum nodes explained", Default: graph.DefaultExplainLimit},
			},
			Required: []string{"node"},
		},
		Handler: handleGraphExplain,
	})
	Register(&Tool{
		Name:        "tb_graph_path",
		Description: "Shortest path between two graph nodes (parity with tb graph path)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"from":      {Type: "string", Description: "Source node"},
				"to":        {Type: "string", Description: "Target node"},
				"max_depth": {Type: "integer", Description: "Maximum hops", Default: graph.DefaultPathMaxDepth},
			},
			Required: []string{"from", "to"},
		},
		Handler: handleGraphPath,
	})
	Register(&Tool{
		Name:        "tb_graph_index",
		Description: "Indexes the code graph on demand",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"path": {Type: "string", Description: "Root to index", Default: "."},
			},
		},
		Handler: handleGraphIndex,
	})
}

func openGraphStore() (*graph.Store, error) {
	if err := graph.NewStore("").RequireEnabled(); err != nil {
		return nil, err
	}
	cwd, _ := os.Getwd()
	dir := mem.LocateRepoToolbeltDir(cwd)
	s := graph.NewStore(graph.DefaultPath(dir))
	_ = s.Load()
	return s, nil
}

func openGraphStoreFresh() (*graph.Store, error) {
	s, err := openGraphStore()
	if err != nil {
		return nil, err
	}
	opts := graphOptionsFromConfig()
	indexOnChange := true
	if cfg, err := config.Load(); err == nil {
		indexOnChange = cfg.Graph.IndexOnChange
	}
	if _, err := s.EnsureFresh(".", &opts, indexOnChange); err != nil {
		return nil, err
	}
	return s, nil
}

func graphNodesPayload(s *graph.Store, nodes []graph.Node) map[string]any {
	return map[string]any{
		"stale": s.Stale(),
		"nodes": nodes,
	}
}

func handleGraphSearch(_ context.Context, args map[string]any) (any, error) {
	s, err := openGraphStoreFresh()
	if err != nil {
		return nil, err
	}
	q := StringArg(args, "query")
	limit := IntArg(args, "limit", 20)
	return graphNodesPayload(s, s.Search(q, limit)), nil
}

func handleGraphCallers(_ context.Context, args map[string]any) (any, error) {
	s, err := openGraphStoreFresh()
	if err != nil {
		return nil, err
	}
	fn := StringArg(args, "function")
	depth := IntArg(args, "depth", 1)
	nodes := s.Callers(fn, depth)
	if len(nodes) == 0 {
		if reason := s.EmptyCallersReason(fn); reason != nil {
			return nil, reason
		}
	}
	return graphNodesPayload(s, nodes), nil
}

func handleGraphDeps(_ context.Context, args map[string]any) (any, error) {
	s, err := openGraphStoreFresh()
	if err != nil {
		return nil, err
	}
	target := StringArg(args, "module")
	if target == "" {
		target = StringArg(args, "file")
	}
	if target == "" {
		return nil, fmt.Errorf("module or file is required")
	}
	transitive := BoolArg(args, "transitive", false)
	return graphNodesPayload(s, s.Deps(target, transitive)), nil
}

func handleGraphImpact(_ context.Context, args map[string]any) (any, error) {
	s, err := openGraphStoreFresh()
	if err != nil {
		return nil, err
	}
	file := StringArg(args, "file")
	if file == "" {
		return nil, fmt.Errorf("file is required")
	}
	depth := IntArg(args, "depth", 2)
	nodes := s.Impact(file, depth)
	if len(nodes) == 0 {
		if reason := s.EmptyImpactReason(file); reason != nil {
			return nil, reason
		}
	}
	return graphNodesPayload(s, nodes), nil
}

func handleGraphExplain(_ context.Context, args map[string]any) (any, error) {
	s, err := openGraphStoreFresh()
	if err != nil {
		return nil, err
	}
	node := StringArg(args, "node")
	if node == "" {
		return nil, fmt.Errorf("node is required")
	}
	exps, err := s.Explain(node, IntArg(args, "limit", graph.DefaultExplainLimit))
	if err != nil {
		return nil, err
	}
	return map[string]any{"stale": s.Stale(), "nodes": exps}, nil
}

func handleGraphPath(_ context.Context, args map[string]any) (any, error) {
	s, err := openGraphStoreFresh()
	if err != nil {
		return nil, err
	}
	from, to := StringArg(args, "from"), StringArg(args, "to")
	if from == "" || to == "" {
		return nil, fmt.Errorf("from and to are required")
	}
	steps, err := s.ShortestPath(from, to, IntArg(args, "max_depth", graph.DefaultPathMaxDepth))
	if err != nil {
		return nil, err
	}
	return map[string]any{"stale": s.Stale(), "hops": len(steps) - 1, "steps": steps}, nil
}

func handleGraphIndex(_ context.Context, args map[string]any) (any, error) {
	s, err := openGraphStore()
	if err != nil {
		return nil, err
	}
	path := StringArg(args, "path")
	if path == "" {
		path = "."
	}
	opts := graphOptionsFromConfig()
	idx, err := s.RebuildOpts(path, &opts)
	if err != nil {
		return nil, err
	}
	s.SetStale(false)
	return map[string]any{
		"nodes": len(idx.Nodes),
		"edges": len(idx.Edges),
		"path":  s.Path(),
		"stale": false,
	}, nil
}

func graphOptionsFromConfig() graph.BuildOptions {
	opts := graph.DefaultBuildOptions()
	cfg, err := config.Load()
	if err != nil {
		return opts
	}
	if len(cfg.Graph.Languages) > 0 {
		opts.Languages = cfg.Graph.Languages
	}
	if len(cfg.Graph.Docs) > 0 {
		opts.Docs = cfg.Graph.Docs
	}
	if len(cfg.Graph.Frameworks) > 0 {
		opts.Frameworks = cfg.Graph.Frameworks
	}
	if cfg.Graph.MaxFileBytes > 0 {
		opts.MaxFileBytes = cfg.Graph.MaxFileBytes
	}
	return opts
}
