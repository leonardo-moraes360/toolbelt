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
		Description: "Busca no grafo de código (sob demanda; requer TB_GRAPH=on)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {Type: "string", Description: "Consulta"},
				"limit": {Type: "integer", Description: "Máximo de resultados", Default: 20},
			},
			Required: []string{"query"},
		},
		Handler: handleGraphSearch,
	})
	Register(&Tool{
		Name:        "tb_graph_callers",
		Description: "Encontra chamadores de uma função no grafo",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"function": {Type: "string", Description: "Nome da função"},
				"depth":    {Type: "integer", Description: "Profundidade", Default: 1},
			},
			Required: []string{"function"},
		},
		Handler: handleGraphCallers,
	})
	Register(&Tool{
		Name:        "tb_graph_deps",
		Description: "Lista dependências de módulo/arquivo no grafo",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"module":     {Type: "string", Description: "Módulo ou pacote"},
				"file":       {Type: "string", Description: "Arquivo"},
				"transitive": {Type: "boolean", Description: "Inclui dependências transitivas", Default: false},
			},
		},
		Handler: handleGraphDeps,
	})
	Register(&Tool{
		Name:        "tb_graph_impact",
		Description: "Analisa impacto de mudanças em um arquivo (paridade com tb graph impact)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"file":  {Type: "string", Description: "Arquivo modificado"},
				"depth": {Type: "integer", Description: "Profundidade", Default: 2},
			},
			Required: []string{"file"},
		},
		Handler: handleGraphImpact,
	})
	Register(&Tool{
		Name:        "tb_graph_explain",
		Description: "Explica um nó do grafo: arestas de entrada/saída com proveniência (paridade com tb graph explain)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"node":  {Type: "string", Description: "ID, título, função ou arquivo"},
				"limit": {Type: "integer", Description: "Máximo de nós explicados", Default: graph.DefaultExplainLimit},
			},
			Required: []string{"node"},
		},
		Handler: handleGraphExplain,
	})
	Register(&Tool{
		Name:        "tb_graph_path",
		Description: "Menor caminho entre dois nós do grafo (paridade com tb graph path)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"from":      {Type: "string", Description: "Nó de origem"},
				"to":        {Type: "string", Description: "Nó de destino"},
				"max_depth": {Type: "integer", Description: "Máximo de saltos", Default: graph.DefaultPathMaxDepth},
			},
			Required: []string{"from", "to"},
		},
		Handler: handleGraphPath,
	})
	Register(&Tool{
		Name:        "tb_graph_index",
		Description: "Indexa o grafo de código sob demanda",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"path": {Type: "string", Description: "Raiz a indexar", Default: "."},
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
	}
	return fields
}

func graphNodesPayload(s *graph.Store, nodes []graph.Node) map[string]any {
	return graphPayload(s, map[string]any{"nodes": nodes})
}

func handleGraphSearch(_ context.Context, args map[string]any) (any, error) {
	s, err := openGraphStoreReadOnly()
	if err != nil {
		return nil, err
	}
	q := StringArg(args, "query")
	limit := IntArg(args, "limit", 20)
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
		return nil, fmt.Errorf("module ou file é obrigatório")
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
		return nil, fmt.Errorf("file é obrigatório")
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
		return nil, fmt.Errorf("node é obrigatório")
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
		return nil, fmt.Errorf("from e to são obrigatórios")
	}
	steps, err := s.ShortestPath(from, to, IntArg(args, "max_depth", graph.DefaultPathMaxDepth))
	if err != nil {
		return nil, err
	}
	return graphPayload(s, map[string]any{"hops": len(steps) - 1, "steps": steps}), nil
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
