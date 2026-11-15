package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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

Subcommands:
  run      Executa benchmark completo
  list     Lista tarefas disponíveis
  report   Gera relatório de resultados existentes
  compare  Compara dois benchmarks`,
}

func init() {
	benchCmd.AddCommand(benchRunCmd)
	benchCmd.AddCommand(benchListCmd)
	benchCmd.AddCommand(benchReportCmd)
	benchCmd.AddCommand(benchCompareCmd)

	// Flags do run
	benchRunCmd.Flags().StringP("agent", "a", "", "command do agente (obrigatório ou use --fake)")
	benchRunCmd.Flags().Bool("fake", false, "usa agente fake para testes")
	benchRunCmd.Flags().StringP("tasks-dir", "t", "bench/tasks", "diretório das tarefas")
	benchRunCmd.Flags().StringP("fixtures-dir", "f", "bench/fixtures", "diretório das fixtures")
	benchRunCmd.Flags().StringP("output", "o", "", "arquivo de saída JSONL")
	benchRunCmd.Flags().IntP("repetitions", "r", 1, "repetições por tarefa")
	benchRunCmd.Flags().StringSlice("task", nil, "IDs específicos de tarefas (opcional)")
	benchRunCmd.Flags().String("fake-config", "", "arquivo de configuração do agente fake")

	// Flags do report
	benchReportCmd.Flags().StringP("input", "i", "", "arquivo JSONL de entrada")
	benchReportCmd.Flags().StringP("output", "o", "", "arquivo Markdown de saída")

	// Flags do compare
	benchCompareCmd.Flags().String("before", "", "arquivo JSONL do benchmark anterior")
	benchCompareCmd.Flags().String("after", "", "arquivo JSONL do benchmark atual")
	benchCompareCmd.Flags().StringP("output", "o", "", "arquivo Markdown de saída")
}

var benchRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Executa benchmark",
	Long: `Executa benchmark contra um agente de código.

O agente é especificado como um command shell que recebe o prompt
e diretório de trabalho via placeholders:
  {prompt}  - prompt da tarefa
  {workdir} - diretório de trabalho temporário
  {timeout} - timeout configurado

Exemplo:
  tb bench run --agent "cursor-agent {prompt} --workdir {workdir}"
  tb bench run --fake                   # usa agente fake para testes
  tb bench run --fake -r 3              # 3 repetições com fake`,
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
			fmt.Printf("  %-20s [%s] %s - %s\n", t.ID, diff, cat, t.Name)
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
		benchID := "report"
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
	Short: "Compara dois benchmarks",
	RunE: func(cmd *cobra.Command, args []string) error {
		beforePath, _ := cmd.Flags().GetString("before")
		afterPath, _ := cmd.Flags().GetString("after")
		output, _ := cmd.Flags().GetString("output")

		if beforePath == "" || afterPath == "" {
			return fmt.Errorf("--before e --after são obrigatórios")
		}

		beforeResults, err := bench.ReadJSONL(beforePath)
		if err != nil {
			return fmt.Errorf("reading before results: %w", err)
		}

		afterResults, err := bench.ReadJSONL(afterPath)
		if err != nil {
			return fmt.Errorf("reading after results: %w", err)
		}

		beforeSummary := bench.CalculateSummary("before", beforeResults[0].AgentID, beforeResults)
		afterSummary := bench.CalculateSummary("after", afterResults[0].AgentID, afterResults)

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

		return bench.WriteComparisonReport(w, beforeSummary, afterSummary)
	},
}

func runBenchmark(cmd *cobra.Command, args []string) error {
	agentCmd, _ := cmd.Flags().GetString("agent")
	useFake, _ := cmd.Flags().GetBool("fake")
	tasksDir, _ := cmd.Flags().GetString("tasks-dir")
	fixturesDir, _ := cmd.Flags().GetString("fixtures-dir")
	outputPath, _ := cmd.Flags().GetString("output")
	repetitions, _ := cmd.Flags().GetInt("repetitions")
	taskIDs, _ := cmd.Flags().GetStringSlice("task")
	fakeConfig, _ := cmd.Flags().GetString("fake-config")

	// Valida arguments
	if !useFake && agentCmd == "" {
		return fmt.Errorf("--agent ou --fake é obrigatório")
	}

	// Configura agente
	var agent bench.AgentConfig
	if useFake {
		// Encontra o binário fake-agent
		fakeAgentCmd := "go run ./cmd/fake-agent"
		if fakeConfig != "" {
			fakeAgentCmd += " --config " + fakeConfig
		}
		fakeAgentCmd += " --workdir {workdir} --prompt {prompt}"

		agent = bench.AgentConfig{
			ID:           "fake",
			Command:      fakeAgentCmd,
			OutputParser: "json",
		}
	} else {
		agent = bench.AgentConfig{
			ID:           "external",
			Command:      agentCmd,
			OutputParser: "json",
		}
	}

	// Configura output
	if outputPath == "" {
		outputPath = fmt.Sprintf("bench/results/%s.jsonl", time.Now().Format("2006-01-02-150405"))
	}

	// Garante diretório de output
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Configura contexto com cancelamento
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Warn("recebido sinal de interrupção, finalizando...")
		cancel()
	}()

	// Cria runner
	runner := bench.NewRunner(agent, tasksDir, fixturesDir, filepath.Dir(outputPath), repetitions)

	// Executa
	log.Info("iniciando benchmark",
		"agent", agent.ID,
		"tasks_dir", tasksDir,
		"repetitions", repetitions,
		"output", outputPath)

	startTime := time.Now()
	results, err := runner.Run(ctx, taskIDs)
	endTime := time.Now()

	if err != nil && len(results) == 0 {
		return fmt.Errorf("benchmark failed: %w", err)
	}

	// Salva resultados
	if err := bench.WriteJSONL(results, outputPath); err != nil {
		return fmt.Errorf("writing results: %w", err)
	}

	// Gera sumário
	benchID := fmt.Sprintf("bench-%s", time.Now().Format("20060102-150405"))
	summary := bench.CalculateSummary(benchID, agent.ID, results)
	summary.StartTime = startTime
	summary.EndTime = endTime

	// Mostra resumo
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  Benchmark completo: %d tarefas × %d repetições\n", summary.TotalTasks, summary.TotalRepetitions)
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  pass@1:        %.1f%%\n", summary.Metrics.PassAt1*100)
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

	// Gera relatório Markdown
	reportPath := outputPath[:len(outputPath)-6] + ".md"
	reportFile, err := os.Create(reportPath)
	if err == nil {
		_ = bench.WriteMarkdownReport(reportFile, summary, results)
		reportFile.Close()
		log.Info("report generated", "path", reportPath)
	}

	return nil
}
