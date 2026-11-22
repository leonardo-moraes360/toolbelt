package registry

import (
	"context"
	"fmt"
	"os"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/graph"
	"github.com/leonardo-moraes360/toolbelt/internal/mem"
	"github.com/leonardo-moraes360/toolbelt/internal/vecsearch"
)

func init() {
	RegisterGraphTools()
}

func RegisterGraphTools() {
	Register(&Tool{
		Name:        "tb_graph_search",
		Description: "Searches the code graph (on demand; requires TB_GRAPH=on). mode semantic or hybrid needs graph.vector.enabled in the user config and tb_graph_index with vectors: true; it starts or reuses a warm embedding child (sandboxed, no network) but writes nothing and never reindexes",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {Type: "string", Description: "Query"},
				"limit": {Type: "integer", Description: "Maximum results", Default: 20},
				"mode": {Type: "string", Description: "lexical (BM25), semantic (vectors) or hybrid (RRF of both)", Default: vecsearch.ModeLexical,
					Enum: []string{vecsearch.ModeLexical, vecsearch.ModeSemantic, vecsearch.ModeHybrid}},
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
				"node":  {Type: "string", Description: "ID (file:, section:, note:, tag:...), title, function, or file"},
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
		Description: "Indexes the code graph on demand; vectors: true also builds or updates .toolbelt/graph.vec (needs graph.vector.enabled in the user config)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"path":    {Type: "string", Description: "Root to index", Default: "."},
				"vectors": {Type: "boolean", Description: "Also build or update the vector index", Default: false},
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

// graphStaleHint tells the agent how to refresh a stale index.
const graphStaleHint = "index is stale (files changed since the last index): call tb_graph_index, then repeat the query"

// graphFormatHint is the stale hint for an index written by an older node ID scheme.
const graphFormatHint = "index format changed (written by an older tb): call tb_graph_index, then repeat the query"

// openGraphStoreReadOnly opens the index for an MCP query without ever
// writing it. Unlike the CLI, which reindexes when graph.index_on_change is
// on, MCP queries only report staleness: they are annotated readOnlyHint, so
// a client may auto-approve or parallelize them, and only tb_graph_index
// writes. (M8ven review, finding 3.)
func openGraphStoreReadOnly() (*graph.Store, error) {
	s, err := openGraphStore()
	if err != nil {
		return nil, err
	}
	opts := graphOptionsFromConfig()
	stale, err := s.CheckStale(".", &opts)
	if err != nil {
		return nil, err
	}
	s.SetStale(stale)
	return s, nil
}

// graphPayload adds the staleness fields shared by every query result.
func graphPayload(s *graph.Store, fields map[string]any) map[string]any {
	fields["stale"] = s.Stale()
	if s.Stale() {
		fields["hint"] = graphStaleHint
		if s.FormatOutdated() {
			fields["hint"] = graphFormatHint
		}
	}
	return fields
}

func graphNodesPayload(s *graph.Store, nodes []graph.Node) map[string]any {
	return graphPayload(s, map[string]any{"nodes": nodes})
}

func handleGraphSearch(ctx context.Context, args map[string]any) (any, error) {
	mode := StringArg(args, "mode")
	if mode == "" {
		mode = vecsearch.ModeLexical
	}
	if err := vecsearch.CheckMode(mode, "mode"); err != nil {
		return nil, err
	}
	s, err := openGraphStoreReadOnly()
	if err != nil {
		return nil, err
	}
	q := StringArg(args, "query")
	limit := IntArg(args, "limit", 20)
	if mode != vecsearch.ModeLexical {
		return graphVectorSearch(ctx, s, mode, q, limit)
	}
	return graphNodesPayload(s, s.Search(q, limit)), nil
}

func handleGraphCallers(_ context.Context, args map[string]any) (any, error) {
	s, err := openGraphStoreReadOnly()
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
	s, err := openGraphStoreReadOnly()
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
	s, err := openGraphStoreReadOnly()
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
	s, err := openGraphStoreReadOnly()
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
	return graphPayload(s, map[string]any{"nodes": exps}), nil
}

func handleGraphPath(_ context.Context, args map[string]any) (any, error) {
	s, err := openGraphStoreReadOnly()
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
	return graphPayload(s, map[string]any{"hops": len(steps) - 1, "steps": steps}), nil
}

func handleGraphIndex(ctx context.Context, args map[string]any) (any, error) {
	s, err := openGraphStore()
	if err != nil {
		return nil, err
	}
	path := StringArg(args, "path")
	if path == "" {
		path = "."
	}
	if BoolArg(args, "vectors", false) {
		if err := vecsearch.CheckVectorRoot(s, path); err != nil {
			return nil, err
		}
	}
	opts := graphOptionsFromConfig()
	idx, err := s.RebuildOpts(path, &opts)
	if err != nil {
		return nil, err
	}
	s.SetStale(false)
	payload := map[string]any{
		"nodes": len(idx.Nodes),
		"edges": len(idx.Edges),
		"path":  s.Path(),
		"stale": false,
	}
	if BoolArg(args, "vectors", false) {
		if err := graphVectorIndex(ctx, s, idx, payload); err != nil {
			return nil, err
		}
	}
	return payload, nil
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
