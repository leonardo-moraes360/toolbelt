package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
	_ "github.com/leonardo-moraes360/toolbelt/internal/registry"
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

Inclui sandbox seguro, verificação adversarial, memória persistent,
grafo de dependências, roteamento multi-agente e benchmark.

Exemplo:
  tb init                    Inicializa configuração local
  tb bench run               Executa benchmark baseline
  tb version                 Mostra versão`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Inicializa logging
		log.Init(jsonOutput)

		config.SetFile(cfgFile)

		// Carrega configuração se não for command init/version/help
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

	// Adiciona subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(benchCmd)

	// Stubs para commands futuros
	rootCmd.AddCommand(sandboxCmd)
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(guardCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(memCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(driftCmd)
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

Detecta a toolchain do repositório (go.mod, package.json, pyproject/uv,
Cargo.toml, Makefile) e preenche verify.test_cmd, e quando houver marcadores
claros também verify.build_cmd e verify.lint_cmd (scripts build/lint,
golangci, ruff/mypy, cargo build/clippy, targets make).

Se AGENTS.md ou CLAUDE.md existirem na raiz do repositório, o init lê esses
arquivos, registra os caminhos em .toolbelt/agent_docs.yaml e extrai dicas
curtas de execução (ignora fences de código longos; nunca sobrescreve). O
binário de verify.test_cmd é checado no PATH com fallback (ex. uv → pytest).
Use --write-agents para semear um AGENTS.md mínimo que cita a toolchain
detectada (verify.test_cmd e, se houver, build_cmd/lint_cmd) quando o
arquivo ainda não existe.

Exemplo:
  tb init                 Cria configuração básica
  tb init --write-agents  Também semeia AGENTS.md se ausente
  tb init --global        Cria configuração global em ~/.config/toolbelt`,
	RunE: func(cmd *cobra.Command, args []string) error {
		global, _ := cmd.Flags().GetBool("global")
		writeAgents, _ := cmd.Flags().GetBool("write-agents")

		var dir string
		var repoRoot string
		if global {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("getting home directory: %w", err)
			}
			dir = filepath.Join(homeDir, ".config", "toolbelt")
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
			repoRoot = cwd
			dir = filepath.Join(cwd, config.DefaultConfigDir)
		}

		detected, err := config.InitDetailed(dir)
		if err != nil {
			return fmt.Errorf("initializing config: %w", err)
		}

		log.Info("configuração inicializada", "path", dir)
		if detected.Cmd != "" {
			log.Info("toolchain detectada", "test_cmd", detected.Cmd)
		}
		if detected.Warning != "" {
			log.Warn(detected.Warning)
		}

		if !global && repoRoot != "" {
			docs, err := config.HandleAgentDocs(repoRoot, dir, writeAgents)
			if err != nil {
				return fmt.Errorf("processando AGENTS.md/CLAUDE.md: %w", err)
			}
			if docs.Seeded {
				log.Info("AGENTS.md semeado", "path", filepath.Join(repoRoot, config.AgentsMDName))
			}
			if len(docs.Sources) > 0 {
				log.Info("documentação de agentes encontrada",
					"sources", strings.Join(docs.Sources, ","),
					"record", docs.Record)
				if len(docs.Hints) > 0 {
					log.Info("dicas de execução em AGENTS.md/CLAUDE.md",
						"hints", strings.Join(docs.Hints, " | "))
				}
			} else if writeAgents {
				log.Info("nenhum AGENTS.md/CLAUDE.md após seed")
			} else {
				log.Info("nenhum AGENTS.md/CLAUDE.md na raiz; use --write-agents para semear")
			}
		}

		return nil
	},
}

func init() {
	initCmd.Flags().Bool("global", false, "inicializa configuração global")
	initCmd.Flags().Bool("write-agents", false, "semeia AGENTS.md mínimo se ausente (nunca sobrescreve)")
}

var routerCmd = &cobra.Command{
	Use:   "router",
	Short: "Roteamento multi-agente (não implementado)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️  Command router ainda não implementado")
		fmt.Println("   Será implementado na Fase 5")
	},
}
