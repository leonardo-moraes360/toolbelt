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
		return nil, fmt.Errorf("module ou file é obrigatório")
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
