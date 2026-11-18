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
	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
)

var benchCmd = &cobra.Command{
	Use:   "bench",
	Short: "Benchmark harness for coding agents",
	Long: `Benchmark system to measure the performance of coding agents.

Runs a set of tasks against a configurable external agent and
collects metrics such as pass@1, tokens, time and cost.

Supports A/B comparison between configs/feature flags on the same set
of tasks (success, time and tokens in the JSONL and in the Markdown report).

Subcommands:
  run      Runs the full benchmark
  list     Lists available tasks
  report   Generates a report from existing results
  compare  Compares two benchmarks or A/B configs`,
}

func init() {
	benchCmd.AddCommand(benchRunCmd)
	benchCmd.AddCommand(benchListCmd)
	benchCmd.AddCommand(benchReportCmd)
	benchCmd.AddCommand(benchCompareCmd)

	benchRunCmd.Flags().StringP("agent", "a", "", "agent command (or bench.default_agent / --fake)")
	benchRunCmd.Flags().Bool("fake", false, "uses the fake agent for tests")
	benchRunCmd.Flags().StringP("tasks-dir", "t", "bench/tasks", "tasks directory")
	benchRunCmd.Flags().StringP("fixtures-dir", "f", "bench/fixtures", "fixtures directory")
	benchRunCmd.Flags().StringP("output", "o", "", "JSONL output file")
	benchRunCmd.Flags().IntP("repetitions", "r", 1, "repetitions per task")
	benchRunCmd.Flags().StringSlice("task", nil, "specific task IDs (optional)")
	benchRunCmd.Flags().String("fake-config", "", "fake agent configuration file")
	benchRunCmd.Flags().String("label", "", "A/B variant label (e.g. graph-on)")
	benchRunCmd.Flags().StringToString("flag", nil, "feature flags key=value (ex: --flag graph=on)")
	benchRunCmd.Flags().String("ab-config", "", "A/B variant YAML file")

	benchListCmd.Flags().StringP("tasks-dir", "t", "bench/tasks", "tasks directory")

	benchReportCmd.Flags().StringP("input", "i", "", "JSONL input file")
	benchReportCmd.Flags().StringP("output", "o", "", "Markdown output file")

	benchCompareCmd.Flags().String("before", "", "JSONL file of variant A")
	benchCompareCmd.Flags().String("after", "", "JSONL file of variant B")
	benchCompareCmd.Flags().String("config-a", "", "YAML of variant A (runs + compares)")
	benchCompareCmd.Flags().String("config-b", "", "YAML of variant B (runs + compares)")
	benchCompareCmd.Flags().StringP("agent", "a", "", "agent command when running A/B configs (or bench.default_agent / --fake)")
	benchCompareCmd.Flags().Bool("fake", false, "uses the fake agent when running A/B configs")
	benchCompareCmd.Flags().StringP("tasks-dir", "t", "bench/tasks", "tasks directory")
	benchCompareCmd.Flags().StringP("fixtures-dir", "f", "bench/fixtures", "fixtures directory")
	benchCompareCmd.Flags().IntP("repetitions", "r", 1, "repetitions per task")
	benchCompareCmd.Flags().StringSlice("task", nil, "specific task IDs")
	benchCompareCmd.Flags().StringP("output", "o", "", "Markdown output file")
	benchCompareCmd.Flags().String("output-dir", "bench/results", "directory for the A/B configs' JSONL")
}

var benchRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs the benchmark",
	Long: `Runs the benchmark against a coding agent.

The agent is specified as a shell command that receives the prompt
and working directory via placeholders:
  {prompt}  - task prompt (interpolated with safe quoting)
  {workdir} - temporary working directory (safe quoting)
  {timeout} - configured timeout (safe quoting)

Also exports TB_BENCH_PROMPT, TB_BENCH_WORKDIR and TB_BENCH_TIMEOUT.

Example:
  tb bench run --agent "cursor-agent {prompt} --workdir {workdir}"
  tb bench run --fake --label graph-off --flag graph=off
  tb bench run --fake --ab-config bench/ab/graph-off.yaml
  tb bench run -r 1   # uses bench.default_agent / tasks_dir from config.yaml
  tb -c alt.yaml bench run -r 1   # -c feeds config.Load()`,
	RunE: runBenchmark,
}

var benchListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists available tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		tasksDir := resolveBenchPath(cmd, "tasks-dir", benchTasksDir(cfg), "bench/tasks")

		tasks, err := bench.LoadTasksFromDir(tasksDir)
		if err != nil {
			return fmt.Errorf("loading tasks: %w", err)
		}

		if len(tasks) == 0 {
			fmt.Println("No tasks found in", tasksDir)
			return nil
		}

		fmt.Printf("Tasks in %s:\n\n", tasksDir)
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
		fmt.Printf("\nTotal: %d tasks\n", len(tasks))

		return nil
	},
}

var benchReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generates a Markdown report of results",
	RunE: func(cmd *cobra.Command, args []string) error {
		input, _ := cmd.Flags().GetString("input")
		output, _ := cmd.Flags().GetString("output")

		if input == "" {
			return fmt.Errorf("--input is required")
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
	Short: "Compares two benchmarks or A/B configs",
	Long: `Compares success, time and token metrics between two A/B sides.

Modes:
  1. Existing JSONL files:
       tb bench compare --before a.jsonl --after b.jsonl
  2. Configs A/B (runs both on the same set of tasks):
       tb bench compare --config-a bench/ab/graph-off.yaml \
                        --config-b bench/ab/graph-on.yaml --fake
       tb bench compare --config-a bench/ab/graph-off.yaml \
                        --config-b bench/ab/graph-on.yaml \
                        --agent "cursor-agent {prompt} --workdir {workdir}"`,
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
		return fmt.Errorf("provide --before/--after or --config-a/--config-b")
	}

	if len(beforeResults) == 0 || len(afterResults) == 0 {
		return fmt.Errorf("empty results for A/B comparison")
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
	agentCmd, _ := cmd.Flags().GetString("agent")
	agentCmd, err = resolveAgentCommand(agentCmd, useFake)
	if err != nil {
		return nil, nil, err
	}
	cfg, _ := config.Load()
	tasksDir := resolveBenchPath(cmd, "tasks-dir", benchTasksDir(cfg), "bench/tasks")
	fixturesDir := resolveBenchPath(cmd, "fixtures-dir", benchFixturesDir(cfg), "bench/fixtures")
	repetitions, _ := cmd.Flags().GetInt("repetitions")
	taskIDs, _ := cmd.Flags().GetStringSlice("task")
	outputDir := resolveBenchPath(cmd, "output-dir", benchOutputDir(cfg), "bench/results")

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

	resultsA, err := runVariant(ctx, useFake, agentCmd, tasksDir, fixturesDir, outputDir, repetitions, taskIDs, variantA)
	if err != nil {
		return nil, nil, fmt.Errorf("variant A: %w", err)
	}
	resultsB, err := runVariant(ctx, useFake, agentCmd, tasksDir, fixturesDir, outputDir, repetitions, taskIDs, variantB)
	if err != nil {
		return nil, nil, fmt.Errorf("variant B: %w", err)
	}
	return resultsA, resultsB, nil
}

func runVariant(
	ctx context.Context,
	useFake bool,
	agentCmd string,
	tasksDir, fixturesDir, outputDir string,
	repetitions int,
	taskIDs []string,
	variant *bench.ABVariant,
) ([]bench.Result, error) {
	agent := defaultAgent(useFake, agentCmd)
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
	log.Info("A/B variant saved", "label", variant.Label, "path", outPath)
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
			Command:      bench.FakeAgentCommand(""),
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
		agent.Command = bench.FakeAgentCommand(opts.fakeConfig)
	}

	variant, err := resolveVariant(opts.abConfig, opts.label, opts.flags)
	if err != nil {
		return err
	}
	bench.ApplyVariant(&agent, variant)
	outputPath := opts.outputPath
	if outputPath == "" {
		outputPath = defaultOutputPath(variant, opts.outputDir)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	ctx, cancel := withSignalCancel()
	defer cancel()

	runner := bench.NewRunner(agent, opts.tasksDir, opts.fixturesDir, filepath.Dir(outputPath), opts.repetitions)
	log.Info("starting benchmark",
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
	return agentCommandNotFoundErr(results, agent.Command)
}

func agentCommandNotFoundErr(results []bench.Result, agentCmd string) error {
	for i := range results {
		if results[i].ExitCode == 127 {
			cmd := strings.TrimSpace(agentCmd)
			if cmd == "" {
				cmd = results[i].AgentID
			}
			return fmt.Errorf("agent command not found (exit 127): %s", cmd)
		}
	}
	return nil
}

type benchRunOpts struct {
	agentCmd    string
	useFake     bool
	tasksDir    string
	fixturesDir string
	outputDir   string
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
	opts.repetitions, _ = cmd.Flags().GetInt("repetitions")
	opts.taskIDs, _ = cmd.Flags().GetStringSlice("task")
	opts.fakeConfig, _ = cmd.Flags().GetString("fake-config")
	opts.label, _ = cmd.Flags().GetString("label")
	opts.flags, _ = cmd.Flags().GetStringToString("flag")
	opts.abConfig, _ = cmd.Flags().GetString("ab-config")

	cfg, _ := config.Load()
	opts.tasksDir = resolveBenchPath(cmd, "tasks-dir", benchTasksDir(cfg), "bench/tasks")
	opts.fixturesDir = resolveBenchPath(cmd, "fixtures-dir", benchFixturesDir(cfg), "bench/fixtures")
	opts.outputDir = benchOutputDir(cfg)
	if opts.outputDir == "" {
		opts.outputDir = "bench/results"
	}
	if cmd.Flags().Changed("output") {
		opts.outputPath, _ = cmd.Flags().GetString("output")
	}

	var err error
	opts.agentCmd, err = resolveAgentCommand(opts.agentCmd, opts.useFake)
	if err != nil {
		return nil, err
	}
	return opts, nil
}

func benchTasksDir(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Bench.TasksDir)
}

func benchFixturesDir(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Bench.FixturesDir)
}

func benchOutputDir(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Bench.OutputDir)
}

func resolveBenchPath(cmd *cobra.Command, flagName, configVal, fallback string) string {
	if cmd.Flags().Changed(flagName) {
		v, _ := cmd.Flags().GetString(flagName)
		if strings.TrimSpace(v) != "" {
			return v
		}
		return fallback
	}
	if configVal != "" {
		return configVal
	}
	if v, _ := cmd.Flags().GetString(flagName); strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func resolveAgentCommand(flagAgent string, useFake bool) (string, error) {
	if useFake {
		return "", nil
	}
	if strings.TrimSpace(flagAgent) != "" {
		return flagAgent, nil
	}
	if cfg, err := config.Load(); err == nil {
		if agent := strings.TrimSpace(cfg.Bench.DefaultAgent); agent != "" {
			return agent, nil
		}
	}
	return "", fmt.Errorf("--agent or --fake is required (or set bench.default_agent in the config)")
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

func defaultOutputPath(variant *bench.ABVariant, outputDir string) string {
	if strings.TrimSpace(outputDir) == "" {
		outputDir = "bench/results"
	}
	suffix := time.Now().Format("2006-01-02-150405")
	if variant != nil && variant.Label != "" {
		suffix = sanitizeLabel(variant.Label) + "-" + suffix
	}
	return filepath.Join(outputDir, suffix+".jsonl")
}

func withSignalCancel() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Warn("interrupt signal received, shutting down...")
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
	fmt.Printf("  Benchmark complete: %d tasks × %d repetitions\n", summary.TotalTasks, summary.TotalRepetitions)
	if variant != nil && variant.Label != "" {
		fmt.Printf("  A/B variant:   %s\n", variant.Label)
	}
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  success:       %.1f%%\n", summary.Metrics.PassAt1*100)
	fmt.Printf("  Avg time:      %.1fs\n", summary.Metrics.AvgDuration)
	if summary.Metrics.TotalTokens > 0 {
		fmt.Printf("  Total tokens:  %d\n", summary.Metrics.TotalTokens)
	}
	if summary.Metrics.TotalCostUSD > 0 {
		fmt.Printf("  Total cost:    $%.4f\n", summary.Metrics.TotalCostUSD)
	}
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  Results: %s\n", outputPath)
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
