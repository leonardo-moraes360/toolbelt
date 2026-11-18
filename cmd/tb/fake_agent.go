package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/bench"
)

var fakeAgentCmd = &cobra.Command{
	Use:    bench.FakeAgentSubcommand,
	Short:  "Built-in fake agent (internal harness use)",
	Hidden: true,
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
	fakeAgentCmd.Flags().String("workdir", "", "working directory")
	fakeAgentCmd.Flags().String("prompt", "", "task prompt")
	fakeAgentCmd.Flags().String("config", "", "JSON configuration file")
	rootCmd.AddCommand(fakeAgentCmd)
}
