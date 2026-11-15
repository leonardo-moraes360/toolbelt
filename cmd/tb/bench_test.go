package main

import (
	"os"
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
