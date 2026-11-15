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
				"limit": {Type: "string", Description: "Máximo de resultados", Default: "20"},
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
				"depth":    {Type: "string", Description: "Profundidade", Default: "1"},
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
				"transitive": {Type: "string", Description: "true/false", Default: "false"},
			},
		},
		Handler: handleGraphDeps,
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

func handleGraphSearch(_ context.Context, args map[string]any) (any, error) {
	s, err := openGraphStore()
	if err != nil {
		return nil, err
	}
	q := StringArg(args, "query")
	limit := 20
	if v := StringArg(args, "limit"); v != "" {
		_, _ = fmt.Sscanf(v, "%d", &limit)
	}
	return s.Search(q, limit), nil
}

func handleGraphCallers(_ context.Context, args map[string]any) (any, error) {
	s, err := openGraphStore()
	if err != nil {
		return nil, err
	}
	fn := StringArg(args, "function")
	depth := 1
	if v := StringArg(args, "depth"); v != "" {
		_, _ = fmt.Sscanf(v, "%d", &depth)
	}
	return s.Callers(fn, depth), nil
}

func handleGraphDeps(_ context.Context, args map[string]any) (any, error) {
	s, err := openGraphStore()
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
	transitive := StringArg(args, "transitive") == "true"
	return s.Deps(target, transitive), nil
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
	return map[string]any{
		"nodes": len(idx.Nodes),
		"edges": len(idx.Edges),
		"path":  s.Path(),
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
