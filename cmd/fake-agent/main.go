package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/bench"
)

var rootCmd = &cobra.Command{
	Use:   "fake-agent",
	Short: "Agente fake deterministico para testes de benchmark",
	Long: `Agente fake que simula um agente de código para testes.

Usado pelo harness de benchmark para validar o sistema sem
necessidade de um LLM real. Comportamento configurável via
arquivo JSON.`,
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
	rootCmd.Flags().String("workdir", "", "diretório de trabalho")
	rootCmd.Flags().String("prompt", "", "prompt da tarefa")
	rootCmd.Flags().String("config", "", "arquivo de configuração JSON")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
