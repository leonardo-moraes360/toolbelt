package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/bench"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
)

var benchCmd = &cobra.Command{
	Use:   "bench",
	Short: "Harness de benchmark para agentes de código",
	Long: `Sistema de benchmark para medir performance de agentes de código.

Permite executar um conjunto de tarefas contra um agente externo
configurável e coletar métricas como pass@1, tokens, tempo e custo.

Suporta comparação A/B entre configs/feature flags no mesmo conjunto
de tarefas (sucesso, tempo e tokens no JSONL e no relatório Markdown).

Subcommands:
  run      Executa benchmark completo
  list     Lista tarefas disponíveis
  report   Gera relatório de resultados existentes
  compare  Compara dois benchmarks ou configs A/B`,
}

func init() {
	benchCmd.AddCommand(benchRunCmd)
	benchCmd.AddCommand(benchListCmd)
	benchCmd.AddCommand(benchReportCmd)
	benchCmd.AddCommand(benchCompareCmd)

	benchRunCmd.Flags().StringP("agent", "a", "", "command do agente (obrigatório ou use --fake)")
	benchRunCmd.Flags().Bool("fake", false, "usa agente fake para testes")
	benchRunCmd.Flags().StringP("tasks-dir", "t", "bench/tasks", "diretório das tarefas")
	benchRunCmd.Flags().StringP("fixtures-dir", "f", "bench/fixtures", "diretório das fixtures")
	benchRunCmd.Flags().StringP("output", "o", "", "arquivo de saída JSONL")
	benchRunCmd.Flags().IntP("repetitions", "r", 1, "repetições por tarefa")
	benchRunCmd.Flags().StringSlice("task", nil, "IDs específicos de tarefas (opcional)")
	benchRunCmd.Flags().String("fake-config", "", "arquivo de configuração do agente fake")
	benchRunCmd.Flags().String("label", "", "rótulo da variante A/B (ex: graph-on)")
	benchRunCmd.Flags().StringToString("flag", nil, "feature flags key=value (ex: --flag graph=on)")
	benchRunCmd.Flags().String("ab-config", "", "arquivo YAML de variante A/B")

	benchListCmd.Flags().StringP("tasks-dir", "t", "bench/tasks", "diretório das tarefas")

	benchReportCmd.Flags().StringP("input", "i", "", "arquivo JSONL de entrada")
	benchReportCmd.Flags().StringP("output", "o", "", "arquivo Markdown de saída")

	benchCompareCmd.Flags().String("before", "", "arquivo JSONL da variante A")
	benchCompareCmd.Flags().String("after", "", "arquivo JSONL da variante B")
	benchCompareCmd.Flags().String("config-a", "", "YAML da variante A (roda + compara)")
	benchCompareCmd.Flags().String("config-b", "", "YAML da variante B (roda + compara)")
	benchCompareCmd.Flags().Bool("fake", false, "usa agente fake ao rodar configs A/B")
	benchCompareCmd.Flags().StringP("tasks-dir", "t", "bench/tasks", "diretório das tarefas")
	benchCompareCmd.Flags().StringP("fixtures-dir", "f", "bench/fixtures", "diretório das fixtures")
	benchCompareCmd.Flags().IntP("repetitions", "r", 1, "repetições por tarefa")
	benchCompareCmd.Flags().StringSlice("task", nil, "IDs específicos de tarefas")
	benchCompareCmd.Flags().StringP("output", "o", "", "arquivo Markdown de saída")
	benchCompareCmd.Flags().String("output-dir", "bench/results", "diretório para JSONL das configs A/B")
}

var benchRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Executa benchmark",
	Long: `Executa benchmark contra um agente de código.

O agente é especificado como um command shell que recebe o prompt
e diretório de trabalho via placeholders:
  {prompt}  - prompt da tarefa (interpolado com quoting seguro)
  {workdir} - diretório de trabalho temporário (quoting seguro)
  {timeout} - timeout configurado (quoting seguro)

Também exporta TB_BENCH_PROMPT, TB_BENCH_WORKDIR e TB_BENCH_TIMEOUT.

Exemplo:
  tb bench run --agent "cursor-agent {prompt} --workdir {workdir}"
  tb bench run --fake --label graph-off --flag graph=off
  tb bench run --fake --ab-config bench/ab/graph-off.yaml`,
	RunE: runBenchmark,
}

var benchListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista tarefas disponíveis",
	RunE: func(cmd *cobra.Command, args []string) error {
		tasksDir, _ := cmd.Flags().GetString("tasks-dir")
		if tasksDir == "" {
			tasksDir = "bench/tasks"
		}

		tasks, err := bench.LoadTasksFromDir(tasksDir)
		if err != nil {
			return fmt.Errorf("loading tasks: %w", err)
		}

		if len(tasks) == 0 {
			fmt.Println("Nenhuma tarefa encontrada em", tasksDir)
			return nil
		}

		fmt.Printf("Tarefas em %s:\n\n", tasksDir)
		for _, t := range tasks {
			cat := t.Category
			if cat == "" {
				cat = "geral"
			}
			diff := t.Difficulty
			if diff == "" {
				diff = "?"
			}
			fmt.Printf("  %-24s [%s] %s - %s\n", t.ID, diff, cat, t.Name)
		}
		fmt.Printf("\nTotal: %d tarefas\n", len(tasks))

		return nil
	},
}

var benchReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Gera relatório Markdown de resultados",
	RunE: func(cmd *cobra.Command, args []string) error {
		input, _ := cmd.Flags().GetString("input")
		output, _ := cmd.Flags().GetString("output")

		if input == "" {
			return fmt.Errorf("--input é obrigatório")
		}

		results, err := bench.ReadJSONL(input)
		if err != nil {
			return fmt.Errorf("reading results: %w", err)
		}

		if len(results) == 0 {
			return fmt.Errorf("no results found in %s", input)
		}

		agentID := results[0].AgentID
		benchID := results[0].ConfigLabel
		if benchID == "" {
			benchID = "report"
		}
		summary := bench.CalculateSummary(benchID, agentID, results)

		var w *os.File
		if output == "" {
			w = os.Stdout
		} else {
			var err error
			w, err = os.Create(output)
			if err != nil {
				return fmt.Errorf("creating output file: %w", err)
			}
			defer w.Close()
		}

		return bench.WriteMarkdownReport(w, summary, results)
	},
}

var benchCompareCmd = &cobra.Command{
	Use:   "compare",
	Short: "Compara dois benchmarks ou configs A/B",
	Long: `Compara métricas de sucesso, tempo e tokens entre dois lados A/B.

Modos:
  1. Arquivos JSONL existentes:
       tb bench compare --before a.jsonl --after b.jsonl
  2. Configs A/B (roda as duas no mesmo conjunto de tarefas):
       tb bench compare --config-a bench/ab/graph-off.yaml \
                        --config-b bench/ab/graph-on.yaml --fake`,
	RunE: runCompare,
}

func runCompare(cmd *cobra.Command, args []string) error {
	beforePath, _ := cmd.Flags().GetString("before")
	afterPath, _ := cmd.Flags().GetString("after")
	configA, _ := cmd.Flags().GetString("config-a")
	configB, _ := cmd.Flags().GetString("config-b")
	output, _ := cmd.Flags().GetString("output")

	var beforeResults, afterResults []bench.Result
	var err error

	switch {
	case configA != "" && configB != "":
		beforeResults, afterResults, err = runABConfigs(cmd, configA, configB)
		if err != nil {
			return err
		}
	case beforePath != "" && afterPath != "":
		beforeResults, err = bench.ReadJSONL(beforePath)
		if err != nil {
			return fmt.Errorf("reading before results: %w", err)
		}
		afterResults, err = bench.ReadJSONL(afterPath)
		if err != nil {
			return fmt.Errorf("reading after results: %w", err)
		}
	default:
		return fmt.Errorf("informe --before/--after ou --config-a/--config-b")
	}

	if len(beforeResults) == 0 || len(afterResults) == 0 {
		return fmt.Errorf("resultados vazios para comparação A/B")
	}

	labelA := beforeResults[0].ConfigLabel
	if labelA == "" {
		labelA = "A"
	}
	labelB := afterResults[0].ConfigLabel
	if labelB == "" {
		labelB = "B"
	}

	beforeSummary := bench.CalculateSummary(labelA, beforeResults[0].AgentID, beforeResults)
	afterSummary := bench.CalculateSummary(labelB, afterResults[0].AgentID, afterResults)

	var w *os.File
	if output == "" {
		w = os.Stdout
	} else {
		w, err = os.Create(output)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer w.Close()
	}

	return bench.WriteComparisonReport(w, beforeSummary, afterSummary)
}

func runABConfigs(cmd *cobra.Command, configA, configB string) (sideA, sideB []bench.Result, err error) {
	useFake, _ := cmd.Flags().GetBool("fake")
	tasksDir, _ := cmd.Flags().GetString("tasks-dir")
	fixturesDir, _ := cmd.Flags().GetString("fixtures-dir")
	repetitions, _ := cmd.Flags().GetInt("repetitions")
	taskIDs, _ := cmd.Flags().GetStringSlice("task")
	outputDir, _ := cmd.Flags().GetString("output-dir")

	variantA, err := bench.LoadABVariant(configA)
	if err != nil {
		return nil, nil, err
	}
	variantB, err := bench.LoadABVariant(configB)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, nil, err
	}

	resultsA, err := runVariant(ctx, useFake, tasksDir, fixturesDir, outputDir, repetitions, taskIDs, variantA)
	if err != nil {
		return nil, nil, fmt.Errorf("variante A: %w", err)
	}
	resultsB, err := runVariant(ctx, useFake, tasksDir, fixturesDir, outputDir, repetitions, taskIDs, variantB)
	if err != nil {
		return nil, nil, fmt.Errorf("variante B: %w", err)
	}
	return resultsA, resultsB, nil
}

func runVariant(
	ctx context.Context,
	useFake bool,
	tasksDir, fixturesDir, outputDir string,
	repetitions int,
	taskIDs []string,
	variant *bench.ABVariant,
) ([]bench.Result, error) {
	agent := defaultAgent(useFake, "")
	bench.ApplyVariant(&agent, variant)
	outPath := filepath.Join(outputDir, sanitizeLabel(variant.Label)+".jsonl")
	runner := bench.NewRunner(agent, tasksDir, fixturesDir, outputDir, repetitions)
	results, err := runner.Run(ctx, taskIDs)
	if err != nil && len(results) == 0 {
		return nil, err
	}
	bench.TagResults(results, variant)
	if err := bench.WriteJSONL(results, outPath); err != nil {
		return nil, err
	}
	log.Info("variante A/B salva", "label", variant.Label, "path", outPath)
	return results, nil
}

func sanitizeLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "variant"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, label)
}

func defaultAgent(useFake bool, agentCmd string) bench.AgentConfig {
	if useFake {
		return bench.AgentConfig{
			ID:           "fake",
			Command:      "go run ./cmd/fake-agent --workdir {workdir} --prompt {prompt}",
			OutputParser: "json",
		}
	}
	return bench.AgentConfig{
		ID:           "external",
		Command:      agentCmd,
		OutputParser: "json",
	}
}

func runBenchmark(cmd *cobra.Command, args []string) error {
	opts, err := parseBenchRunFlags(cmd)
	if err != nil {
		return err
	}

	agent := defaultAgent(opts.useFake, opts.agentCmd)
	if opts.useFake && opts.fakeConfig != "" {
		agent.Command = "go run ./cmd/fake-agent --config " + opts.fakeConfig + " --workdir {workdir} --prompt {prompt}"
	}

	variant, err := resolveVariant(opts.abConfig, opts.label, opts.flags)
	if err != nil {
		return err
	}
	bench.ApplyVariant(&agent, variant)
	outputPath := opts.outputPath
	if outputPath == "" {
		outputPath = defaultOutputPath(variant)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	ctx, cancel := withSignalCancel()
	defer cancel()

	runner := bench.NewRunner(agent, opts.tasksDir, opts.fixturesDir, filepath.Dir(outputPath), opts.repetitions)
	log.Info("iniciando benchmark",
		"agent", agent.ID,
		"label", labelOr(variant),
		"tasks_dir", opts.tasksDir,
		"repetitions", opts.repetitions,
		"output", outputPath)

	startTime := time.Now()
	results, err := runner.Run(ctx, opts.taskIDs)
	endTime := time.Now()
	if err != nil && len(results) == 0 {
		return fmt.Errorf("benchmark failed: %w", err)
	}

	bench.TagResults(results, variant)
	if err := bench.WriteJSONL(results, outputPath); err != nil {
		return fmt.Errorf("writing results: %w", err)
	}

	summary := bench.CalculateSummary(benchIDFor(variant), agent.ID, results)
	summary.StartTime = startTime
	summary.EndTime = endTime
	printBenchSummary(summary, variant, outputPath)
	writeBenchMarkdown(outputPath, summary, results)
	return nil
}

type benchRunOpts struct {
	agentCmd    string
	useFake     bool
	tasksDir    string
	fixturesDir string
	outputPath  string
	repetitions int
	taskIDs     []string
	fakeConfig  string
	label       string
	flags       map[string]string
	abConfig    string
}

func parseBenchRunFlags(cmd *cobra.Command) (*benchRunOpts, error) {
	opts := &benchRunOpts{}
	opts.agentCmd, _ = cmd.Flags().GetString("agent")
	opts.useFake, _ = cmd.Flags().GetBool("fake")
	opts.tasksDir, _ = cmd.Flags().GetString("tasks-dir")
	opts.fixturesDir, _ = cmd.Flags().GetString("fixtures-dir")
	opts.outputPath, _ = cmd.Flags().GetString("output")
	opts.repetitions, _ = cmd.Flags().GetInt("repetitions")
	opts.taskIDs, _ = cmd.Flags().GetStringSlice("task")
	opts.fakeConfig, _ = cmd.Flags().GetString("fake-config")
	opts.label, _ = cmd.Flags().GetString("label")
	opts.flags, _ = cmd.Flags().GetStringToString("flag")
	opts.abConfig, _ = cmd.Flags().GetString("ab-config")
	if !opts.useFake && opts.agentCmd == "" {
		return nil, fmt.Errorf("--agent ou --fake é obrigatório")
	}
	return opts, nil
}

func resolveVariant(abConfig, label string, flags map[string]string) (*bench.ABVariant, error) {
	if abConfig != "" {
		return bench.LoadABVariant(abConfig)
	}
	if label != "" || len(flags) > 0 {
		return &bench.ABVariant{Label: label, Flags: flags}, nil
	}
	return nil, nil
}

func defaultOutputPath(variant *bench.ABVariant) string {
	suffix := time.Now().Format("2006-01-02-150405")
	if variant != nil && variant.Label != "" {
		suffix = sanitizeLabel(variant.Label) + "-" + suffix
	}
	return fmt.Sprintf("bench/results/%s.jsonl", suffix)
}

func withSignalCancel() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Warn("recebido sinal de interrupção, finalizando...")
		cancel()
	}()
	return ctx, cancel
}

func benchIDFor(variant *bench.ABVariant) string {
	if variant != nil && variant.Label != "" {
		return variant.Label
	}
	return fmt.Sprintf("bench-%s", time.Now().Format("20060102-150405"))
}

func printBenchSummary(summary *bench.Summary, variant *bench.ABVariant, outputPath string) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  Benchmark completo: %d tarefas × %d repetições\n", summary.TotalTasks, summary.TotalRepetitions)
	if variant != nil && variant.Label != "" {
		fmt.Printf("  Variante A/B:  %s\n", variant.Label)
	}
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  sucesso:       %.1f%%\n", summary.Metrics.PassAt1*100)
	fmt.Printf("  Tempo médio:   %.1fs\n", summary.Metrics.AvgDuration)
	if summary.Metrics.TotalTokens > 0 {
		fmt.Printf("  Tokens total:  %d\n", summary.Metrics.TotalTokens)
	}
	if summary.Metrics.TotalCostUSD > 0 {
		fmt.Printf("  Custo total:   $%.4f\n", summary.Metrics.TotalCostUSD)
	}
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  Resultados: %s\n", outputPath)
	fmt.Println()
}

func writeBenchMarkdown(outputPath string, summary *bench.Summary, results []bench.Result) {
	reportPath := strings.TrimSuffix(outputPath, ".jsonl") + ".md"
	reportFile, err := os.Create(reportPath)
	if err != nil {
		return
	}
	_ = bench.WriteMarkdownReport(reportFile, summary, results)
	reportFile.Close()
	log.Info("report generated", "path", reportPath)
}

func labelOr(v *bench.ABVariant) string {
	if v == nil || v.Label == "" {
		return "-"
	}
	return v.Label
}
