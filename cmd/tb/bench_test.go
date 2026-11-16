package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
)

func TestResolveAgentCommandUsesDefaultAgent(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	cfg := config.DefaultConfig()
	cfg.Bench.DefaultAgent = "echo AGENT_RAN {prompt}"
	if err := config.Save(cfg, config.LocalConfigPath()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := resolveAgentCommand("", false)
	if err != nil {
		t.Fatalf("resolveAgentCommand: %v", err)
	}
	if got != "echo AGENT_RAN {prompt}" {
		t.Fatalf("got %q, want default_agent from config", got)
	}
}

func TestResolveAgentCommandFlagOverridesDefault(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	cfg := config.DefaultConfig()
	cfg.Bench.DefaultAgent = "echo FROM_CONFIG {prompt}"
	if err := config.Save(cfg, config.LocalConfigPath()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := resolveAgentCommand("echo FROM_FLAG {prompt}", false)
	if err != nil {
		t.Fatalf("resolveAgentCommand: %v", err)
	}
	if got != "echo FROM_FLAG {prompt}" {
		t.Fatalf("got %q, want flag override", got)
	}
}

func TestResolveAgentCommandRequiresAgentWithoutConfig(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	_, err := resolveAgentCommand("", false)
	if err == nil {
		t.Fatal("expected error when agent and default_agent are empty")
	}
	if !strings.Contains(err.Error(), "bench.default_agent") {
		t.Fatalf("error should mention bench.default_agent, got %v", err)
	}
}

func TestResolveAgentCommandFakeSkipsDefault(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	cfg := config.DefaultConfig()
	cfg.Bench.DefaultAgent = "echo SHOULD_NOT_USE {prompt}"
	if err := config.Save(cfg, config.LocalConfigPath()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := resolveAgentCommand("", true)
	if err != nil {
		t.Fatalf("resolveAgentCommand: %v", err)
	}
	if got != "" {
		t.Fatalf("fake mode should ignore default_agent, got %q", got)
	}
}

func TestParseBenchRunFlagsHonorsDefaultAgent(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	cfg := config.DefaultConfig()
	cfg.Bench.DefaultAgent = "echo hi {prompt}"
	if err := config.Save(cfg, config.LocalConfigPath()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newBenchRunTestCmd()
	opts, err := parseBenchRunFlags(cmd)
	if err != nil {
		t.Fatalf("parseBenchRunFlags: %v", err)
	}
	if opts.agentCmd != "echo hi {prompt}" {
		t.Fatalf("agentCmd = %q, want default_agent", opts.agentCmd)
	}
}

func TestParseBenchRunFlagsStillRequiresAgentWithoutDefault(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	cmd := newBenchRunTestCmd()
	_, err := parseBenchRunFlags(cmd)
	if err == nil {
		t.Fatal("expected error without --agent/--fake/default_agent")
	}
}

func TestBenchCompareHasAgentFlag(t *testing.T) {
	f := benchCompareCmd.Flags().Lookup("agent")
	if f == nil {
		t.Fatal("bench compare must expose --agent")
	}
}

func TestParseBenchRunFlagsHonorsConfigDirs(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	tasks := filepath.Join(tmp, "custom-tasks")
	fixtures := filepath.Join(tmp, "custom-fixtures")
	outDir := filepath.Join(tmp, "custom-out")

	cfg := config.DefaultConfig()
	cfg.Bench.DefaultAgent = "echo hi {prompt}"
	cfg.Bench.TasksDir = tasks
	cfg.Bench.FixturesDir = fixtures
	cfg.Bench.OutputDir = outDir
	if err := config.Save(cfg, config.LocalConfigPath()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cmd := newBenchRunTestCmd()
	opts, err := parseBenchRunFlags(cmd)
	if err != nil {
		t.Fatalf("parseBenchRunFlags: %v", err)
	}
	if opts.tasksDir != tasks {
		t.Fatalf("tasksDir = %q, want %q", opts.tasksDir, tasks)
	}
	if opts.fixturesDir != fixtures {
		t.Fatalf("fixturesDir = %q, want %q", opts.fixturesDir, fixtures)
	}
	if opts.outputDir != outDir {
		t.Fatalf("outputDir = %q, want %q", opts.outputDir, outDir)
	}
}

func TestParseBenchRunFlagsCLIOverridesConfigDirs(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	cfg := config.DefaultConfig()
	cfg.Bench.DefaultAgent = "echo hi {prompt}"
	cfg.Bench.TasksDir = filepath.Join(tmp, "from-yaml-tasks")
	cfg.Bench.FixturesDir = filepath.Join(tmp, "from-yaml-fixtures")
	if err := config.Save(cfg, config.LocalConfigPath()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cliTasks := filepath.Join(tmp, "from-cli-tasks")
	cliFixtures := filepath.Join(tmp, "from-cli-fixtures")
	cmd := newBenchRunTestCmd()
	if err := cmd.Flags().Set("tasks-dir", cliTasks); err != nil {
		t.Fatalf("Set tasks-dir: %v", err)
	}
	if err := cmd.Flags().Set("fixtures-dir", cliFixtures); err != nil {
		t.Fatalf("Set fixtures-dir: %v", err)
	}

	opts, err := parseBenchRunFlags(cmd)
	if err != nil {
		t.Fatalf("parseBenchRunFlags: %v", err)
	}
	if opts.tasksDir != cliTasks {
		t.Fatalf("tasksDir = %q, want CLI override %q", opts.tasksDir, cliTasks)
	}
	if opts.fixturesDir != cliFixtures {
		t.Fatalf("fixturesDir = %q, want CLI override %q", opts.fixturesDir, cliFixtures)
	}
}

func TestResolveAgentCommandUsesConfigFileFlag(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	cfgPath := filepath.Join(tmp, "alt-config.yaml")
	cfg := config.DefaultConfig()
	cfg.Bench.DefaultAgent = "echo FROM_C_FLAG {prompt}"
	if err := config.Save(cfg, cfgPath); err != nil {
		t.Fatalf("Save: %v", err)
	}

	config.SetFile(cfgPath)
	t.Cleanup(func() { config.SetFile("") })

	got, err := resolveAgentCommand("", false)
	if err != nil {
		t.Fatalf("resolveAgentCommand: %v", err)
	}
	if got != "echo FROM_C_FLAG {prompt}" {
		t.Fatalf("got %q, want default_agent from -c file", got)
	}
}

func TestParseBenchRunFlagsUsesConfigFileFlagDirs(t *testing.T) {
	tmp := t.TempDir()
	chdir(t, tmp)

	tasks := filepath.Join(tmp, "abs-tasks")
	fixtures := filepath.Join(tmp, "abs-fixtures")
	outDir := filepath.Join(tmp, "abs-out")
	cfgPath := filepath.Join(tmp, "alt-config.yaml")

	cfg := config.DefaultConfig()
	cfg.Bench.DefaultAgent = "echo hi {prompt}"
	cfg.Bench.TasksDir = tasks
	cfg.Bench.FixturesDir = fixtures
	cfg.Bench.OutputDir = outDir
	if err := config.Save(cfg, cfgPath); err != nil {
		t.Fatalf("Save: %v", err)
	}

	config.SetFile(cfgPath)
	t.Cleanup(func() { config.SetFile("") })

	cmd := newBenchRunTestCmd()
	opts, err := parseBenchRunFlags(cmd)
	if err != nil {
		t.Fatalf("parseBenchRunFlags: %v", err)
	}
	if opts.tasksDir != tasks {
		t.Fatalf("tasksDir = %q, want %q from -c", opts.tasksDir, tasks)
	}
	if opts.fixturesDir != fixtures {
		t.Fatalf("fixturesDir = %q, want %q from -c", opts.fixturesDir, fixtures)
	}
	if opts.outputDir != outDir {
		t.Fatalf("outputDir = %q, want %q from -c", opts.outputDir, outDir)
	}
}

func TestResolveBenchPathPrecedence(t *testing.T) {
	cmd := newBenchRunTestCmd()
	got := resolveBenchPath(cmd, "tasks-dir", "/from/config", "bench/tasks")
	if got != "/from/config" {
		t.Fatalf("without Changed, want config value, got %q", got)
	}
	if err := cmd.Flags().Set("tasks-dir", "/from/cli"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got = resolveBenchPath(cmd, "tasks-dir", "/from/config", "bench/tasks")
	if got != "/from/cli" {
		t.Fatalf("with Changed, want CLI, got %q", got)
	}
}

func TestDefaultOutputPathUsesOutputDir(t *testing.T) {
	path := defaultOutputPath(nil, "/tmp/bench-out")
	dir := filepath.Dir(path)
	if dir != "/tmp/bench-out" {
		t.Fatalf("dir = %q, want /tmp/bench-out", dir)
	}
	if filepath.Ext(path) != ".jsonl" {
		t.Fatalf("ext = %q, want .jsonl", filepath.Ext(path))
	}
}

func newBenchRunTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "run"}
	cmd.Flags().StringP("agent", "a", "", "")
	cmd.Flags().Bool("fake", false, "")
	cmd.Flags().StringP("tasks-dir", "t", "bench/tasks", "")
	cmd.Flags().StringP("fixtures-dir", "f", "bench/fixtures", "")
	cmd.Flags().StringP("output", "o", "", "")
	cmd.Flags().IntP("repetitions", "r", 1, "")
	cmd.Flags().StringSlice("task", nil, "")
	cmd.Flags().String("fake-config", "", "")
	cmd.Flags().String("label", "", "")
	cmd.Flags().StringToString("flag", nil, "")
	cmd.Flags().String("ab-config", "", "")
	return cmd
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%s): %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
}
