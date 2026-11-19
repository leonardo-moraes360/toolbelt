package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/graph"
	"github.com/leonardo-moraes360/toolbelt/internal/mem"
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Grafo de código sob demanda (ADR-011)",
	Long: `Consulta e indexa o grafo de código sob demanda.

Desabilitado por padrão. Ative com TB_GRAPH=on (ou --enable) para
medir o ROI via A/B (bench/ab/graph-on.yaml vs graph-off.yaml).`,
}

var graphEnable bool

func init() {
	graphCmd.PersistentFlags().BoolVar(&graphEnable, "enable", false, "habilita o grafo nesta invocação")
	graphCmd.AddCommand(graphIndexCmd, graphRebuildCmd, graphSearchCmd, graphCallersCmd, graphDepsCmd, graphImpactCmd, graphExplainCmd, graphPathCmd, graphStatsCmd)
}

func graphStore() (*graph.Store, error) {
	if graphEnable {
		_ = os.Setenv(graph.EnvEnable, "on")
	}
	s := graph.NewStore("")
	if err := s.RequireEnabled(); err != nil {
		return nil, err
	}
	cwd, _ := os.Getwd()
	dir := mem.LocateRepoToolbeltDir(cwd)
	s = graph.NewStore(graph.DefaultPath(dir))
	_ = s.Load()
	return s, nil
}

func graphStoreFresh() (*graph.Store, error) {
	s, err := graphStore()
	if err != nil {
		return nil, err
	}
	opts := graphBuildOptions()
	indexOnChange := true
	if cfg, err := config.Load(); err == nil {
		indexOnChange = cfg.Graph.IndexOnChange
	}
	refreshed, err := s.EnsureFresh(".", &opts, indexOnChange)
	if err != nil {
		return nil, err
	}
	if !jsonOutput {
		switch {
		case refreshed:
			fmt.Fprintln(os.Stderr, "grafo reindexado (estava desatualizado)")
		case s.Stale():
			fmt.Fprintln(os.Stderr, "aviso: grafo desatualizado; rode tb graph rebuild")
		}
	}
	return s, nil
}

var graphIndexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Indexa código sob demanda",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "."
		if len(args) > 0 {
			root = args[0]
		}
		s, err := graphStore()
		if err != nil {
			return err
		}
		opts := graphBuildOptions()
		idx, err := s.RebuildOpts(root, &opts)
		if err != nil {
			return err
		}
		s.SetStale(false)
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(graphStatsPayload(s))
		}
		fmt.Printf("grafo indexado: %d nós, %d arestas → %s\n", len(idx.Nodes), len(idx.Edges), s.Path())
		return nil
	},
}

var graphRebuildCmd = &cobra.Command{
	Use:   "rebuild [path]",
	Short: "Reconstrói o grafo completo",
	RunE:  graphIndexCmd.RunE,
}

var graphSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Busca no grafo",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		s, err := graphStoreFresh()
		if err != nil {
			return err
		}
		hits := s.Search(args[0], limit)
		return printGraphNodes(s, hits)
	},
}

var graphCallersCmd = &cobra.Command{
	Use:   "callers",
	Short: "Encontra chamadores de uma função",
	RunE: func(cmd *cobra.Command, args []string) error {
		fn, _ := cmd.Flags().GetString("function")
		depth, _ := cmd.Flags().GetInt("depth")
		if fn == "" {
			return fmt.Errorf("--function é obrigatório")
		}
		s, err := graphStoreFresh()
		if err != nil {
			return err
		}
		nodes := s.Callers(fn, depth)
		if len(nodes) == 0 {
			if reason := s.EmptyCallersReason(fn); reason != nil {
				return reason
			}
		}
		return printGraphNodes(s, nodes)
	},
}

var graphDepsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Mostra dependências",
	RunE: func(cmd *cobra.Command, args []string) error {
		mod, _ := cmd.Flags().GetString("module")
		file, _ := cmd.Flags().GetString("file")
		transitive, _ := cmd.Flags().GetBool("transitive")
		target := mod
		if target == "" {
			target = file
		}
		if target == "" {
			return fmt.Errorf("--module ou --file é obrigatório")
		}
		s, err := graphStoreFresh()
		if err != nil {
			return err
		}
		return printGraphNodes(s, s.Deps(target, transitive))
	},
}

var graphImpactCmd = &cobra.Command{
	Use:   "impact",
	Short: "Analisa impacto de mudanças",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		depth, _ := cmd.Flags().GetInt("depth")
		if file == "" {
			return fmt.Errorf("--file é obrigatório")
		}
		s, err := graphStoreFresh()
		if err != nil {
			return err
		}
		nodes := s.Impact(file, depth)
		if len(nodes) == 0 {
			if reason := s.EmptyImpactReason(file); reason != nil {
				return reason
			}
		}
		return printGraphNodes(s, nodes)
	},
}

var graphExplainCmd = &cobra.Command{
	Use:   "explain <node>",
	Short: "Explica um nó: arestas de entrada/saída com proveniência",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		s, err := graphStoreFresh()
		if err != nil {
			return err
		}
		exps, err := s.Explain(args[0], limit)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{
				"stale": s.Stale(),
				"nodes": exps,
			})
		}
		for i, ex := range exps {
			if i > 0 {
				fmt.Println()
			}
			fmt.Printf("%s  %s  %s\n", ex.Node.ID, ex.Node.Type, ex.Node.Path)
			fmt.Printf("  saída (%d):\n", ex.OutDegree)
			printNeighbors(ex.Outgoing, ex.TruncatedOut, "→")
			if ex.UnresolvedOut > 0 {
				fmt.Printf("    (+%d chamadas sem nó no índice: stdlib/receptor não resolvido)\n", ex.UnresolvedOut)
			}
			fmt.Printf("  entrada (%d):\n", ex.InDegree)
			printNeighbors(ex.Incoming, ex.TruncatedIn, "←")
		}
		return nil
	},
}

func printNeighbors(ns []graph.Neighbor, truncated int, arrow string) {
	for i := range ns {
		n := &ns[i]
		fmt.Printf("    %s %-9s %-40s %s %.2f\n", arrow, n.Edge, n.Node.ID, n.Source, n.Confidence)
	}
	if truncated > 0 {
		fmt.Printf("    … +%d omitidas\n", truncated)
	}
}

var graphPathCmd = &cobra.Command{
	Use:   "path <from> <to>",
	Short: "Menor caminho entre dois nós (arestas em ambos os sentidos)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		maxDepth, _ := cmd.Flags().GetInt("max-depth")
		s, err := graphStoreFresh()
		if err != nil {
			return err
		}
		steps, err := s.ShortestPath(args[0], args[1], maxDepth)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(map[string]any{
				"stale": s.Stale(),
				"hops":  len(steps) - 1,
				"steps": steps,
			})
		}
		for i, st := range steps {
			if i == 0 {
				fmt.Printf("%s\n", st.Node.ID)
				continue
			}
			arrow := "→"
			if st.Direction == graph.DirBackward {
				arrow = "←"
			}
			fmt.Printf("  %s %-9s %s  (%s %.2f)\n", arrow, st.Edge, st.Node.ID, st.Source, st.Confidence)
		}
		return nil
	},
}

var graphStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Estatísticas do grafo",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := graphStoreFresh()
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(graphStatsPayload(s))
		}
		if s.Stale() {
			fmt.Println("stale: true")
		}
		for k, v := range s.Stats() {
			fmt.Printf("%s: %d\n", k, v)
		}
		return nil
	},
}

func init() {
	graphSearchCmd.Flags().Int("limit", 20, "máximo de resultados")
	graphCallersCmd.Flags().String("function", "", "nome da função")
	graphCallersCmd.Flags().Int("depth", 1, "profundidade")
	graphDepsCmd.Flags().String("module", "", "módulo/pacote")
	graphDepsCmd.Flags().String("file", "", "arquivo")
	graphDepsCmd.Flags().Bool("transitive", false, "inclui transitivas")
	graphImpactCmd.Flags().String("file", "", "arquivo modificado")
	graphImpactCmd.Flags().Int("depth", 2, "profundidade")
	graphExplainCmd.Flags().Int("limit", graph.DefaultExplainLimit, "máximo de nós explicados")
	graphPathCmd.Flags().Int("max-depth", graph.DefaultPathMaxDepth, "máximo de saltos")
}

func graphBuildOptions() graph.BuildOptions {
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

func graphStatsPayload(s *graph.Store) map[string]any {
	out := map[string]any{"stale": s.Stale()}
	for k, v := range s.Stats() {
		out[k] = v
	}
	return out
}

func printGraphNodes(s *graph.Store, nodes []graph.Node) error {
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"stale": s.Stale(),
			"nodes": nodes,
		})
	}
	if len(nodes) == 0 {
		fmt.Println("(vazio)")
		return nil
	}
	for _, n := range nodes {
		fmt.Printf("%-28s %-10s %s\n", n.ID, n.Type, n.Path)
	}
	return nil
}
