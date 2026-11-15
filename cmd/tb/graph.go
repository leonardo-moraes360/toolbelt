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
	graphCmd.AddCommand(graphIndexCmd, graphRebuildCmd, graphSearchCmd, graphCallersCmd, graphDepsCmd, graphImpactCmd, graphStatsCmd)
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
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(s.Stats())
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
		s, err := graphStore()
		if err != nil {
			return err
		}
		hits := s.Search(args[0], limit)
		return printNodes(hits)
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
		s, err := graphStore()
		if err != nil {
			return err
		}
		return printNodes(s.Callers(fn, depth))
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
		s, err := graphStore()
		if err != nil {
			return err
		}
		return printNodes(s.Deps(target, transitive))
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
		s, err := graphStore()
		if err != nil {
			return err
		}
		return printNodes(s.Impact(file, depth))
	},
}

var graphStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Estatísticas do grafo",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := graphStore()
		if err != nil {
			return err
		}
		stats := s.Stats()
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(stats)
		}
		for k, v := range stats {
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
	if cfg.Graph.MaxFileBytes > 0 {
		opts.MaxFileBytes = cfg.Graph.MaxFileBytes
	}
	return opts
}

func printNodes(nodes []graph.Node) error {
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(nodes)
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
