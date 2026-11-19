package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/bench"
)

var rootCmd = &cobra.Command{
	Use:   "fake-agent",
	Short: "Deterministic fake agent for benchmark tests",
	Long: `Fake agent that simulates a coding agent for tests.

Used by the benchmark harness to validate the system without
needing a real LLM. Behavior is configurable via a JSON
file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, _ := cmd.Flags().GetString("workdir")
		prompt, _ := cmd.Flags().GetString("prompt")
		configPath, _ := cmd.Flags().GetString("config")

		if workDir == "" {
			return fmt.Errorf("--workdir is required")
		}

		return bench.RunFakeAgent(workDir, prompt, configPath)
	},
}

func init() {
	rootCmd.Flags().String("workdir", "", "working directory")
	rootCmd.Flags().String("prompt", "", "task prompt")
	rootCmd.Flags().String("config", "", "JSON configuration file")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
