package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/bench"
)

var fakeAgentCmd = &cobra.Command{
	Use:    bench.FakeAgentSubcommand,
	Short:  "Agente fake embutido (uso interno do harness)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		workDir, _ := cmd.Flags().GetString("workdir")
		prompt, _ := cmd.Flags().GetString("prompt")
		configPath, _ := cmd.Flags().GetString("config")
		if workDir == "" {
			return fmt.Errorf("--workdir é obrigatório")
		}
		return bench.RunFakeAgent(workDir, prompt, configPath)
	},
}

func init() {
	fakeAgentCmd.Flags().String("workdir", "", "diretório de trabalho")
	fakeAgentCmd.Flags().String("prompt", "", "prompt da tarefa")
	fakeAgentCmd.Flags().String("config", "", "arquivo de configuração JSON")
	rootCmd.AddCommand(fakeAgentCmd)
}
