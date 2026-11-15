package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
	"github.com/leonardo-moraes360/toolbelt/internal/version"
)

var (
	cfgFile    string
	jsonOutput bool
	verbose    bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "tb",
	Short: "Toolbelt - Kit de ferramentas para agentes de código IA",
	Long: `Toolbelt (tb) é um kit de ferramentas para melhorar agentes de código IA.

Inclui sandbox seguro, verificação adversarial, memória persistente,
grafo de dependências, roteamento multi-agente e benchmark.

Exemplo:
  tb init                    Inicializa configuração local
  tb bench run               Executa benchmark baseline
  tb version                 Mostra versão`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Inicializa logging
		log.Init(jsonOutput)

		// Carrega configuração se não for comando init/version/help
		if cmd.Name() != "init" && cmd.Name() != "version" && cmd.Name() != "help" {
			_, err := config.Load()
			if err != nil {
				log.Debug("config not loaded", "error", err)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "arquivo de configuração")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "saída em JSON")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "saída verbose")

	// Adiciona subcomandos
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(benchCmd)

	// Stubs para comandos futuros
	rootCmd.AddCommand(sandboxCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(memCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(routerCmd)
	rootCmd.AddCommand(mcpCmd)
}

// versionCmd mostra informações de versão
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Mostra versão do toolbelt",
	Run: func(cmd *cobra.Command, args []string) {
		info := version.Get()
		if jsonOutput {
			fmt.Printf("{\"version\":%q,\"commit\":%q,\"build_time\":%q,\"go_version\":%q}\n",
				info.Version, info.Commit, info.BuildTime, info.GoVersion)
		} else {
			fmt.Println(info.String())
		}
	},
}

// initCmd inicializa configuração local
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Inicializa configuração local do toolbelt",
	Long: `Cria o diretório .toolbelt/ e arquivo config.yaml no diretório atual.

Exemplo:
  tb init              Cria configuração básica
  tb init --global     Cria configuração global em ~/.config/toolbelt`,
	RunE: func(cmd *cobra.Command, args []string) error {
		global, _ := cmd.Flags().GetBool("global")

		var dir string
		if global {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("getting home directory: %w", err)
			}
			dir = homeDir + "/.config/toolbelt"
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
			dir = cwd + "/.toolbelt"
		}

		if err := config.Init(dir); err != nil {
			return fmt.Errorf("initializing config: %w", err)
		}

		log.Info("configuração inicializada", "path", dir)
		return nil
	},
}

func init() {
	initCmd.Flags().Bool("global", false, "inicializa configuração global")
}

// Stubs para comandos futuros

var sandboxCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "Sandbox isolado para execução de código (não implementado)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️  Comando sandbox ainda não implementado")
		fmt.Println("   Será implementado na Fase 1")
	},
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verificação adversarial de código (não implementado)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️  Comando verify ainda não implementado")
		fmt.Println("   Será implementado na Fase 2")
	},
}

var memCmd = &cobra.Command{
	Use:   "mem",
	Short: "Memória persistente do agente (não implementado)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️  Comando mem ainda não implementado")
		fmt.Println("   Será implementado na Fase 3")
	},
}

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Grafo de dependências do código (não implementado)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️  Comando graph ainda não implementado")
		fmt.Println("   Será implementado na Fase 4")
	},
}

var routerCmd = &cobra.Command{
	Use:   "router",
	Short: "Roteamento multi-agente (não implementado)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️  Comando router ainda não implementado")
		fmt.Println("   Será implementado na Fase 5")
	},
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Servidor MCP para integração com IDEs (não implementado)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️  Comando mcp ainda não implementado")
		fmt.Println("   Será implementado na Fase 6")
	},
}
