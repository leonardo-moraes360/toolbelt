package main

import (
	"encoding/json"
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
	envAllow   []string
	envPass    []string
)

func main() {
	rejectUnknownSubcommands(rootCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "tb",
	Short: "Toolbelt - Toolkit for AI coding agents",
	Long: `Toolbelt (tb) is a toolkit for improving AI coding agents.

Includes a secure sandbox, adversarial verification, persistent memory,
dependency graph, multi-agent routing and benchmark.

Example:
  tb init                    Initializes local configuration
  tb bench run               Runs baseline benchmark
  tb version                 Shows version
  tb --version               Shows version (global flag)`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		log.Init(jsonOutput)

		config.SetFile(cfgFile)

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
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "configuration file")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "JSON output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringArrayVar(&envAllow, "env", nil, "allows an environment variable in the child (repeatable; does not bypass secret blocking)")
	rootCmd.PersistentFlags().StringArrayVar(&envPass, "pass-env", nil, "explicit opt-in to pass a secret (*TOKEN*/SECRET/KEY/PASSWORD) to the child")

	rootCmd.Version = version.Get().String()
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.InitDefaultVersionFlag()

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(benchCmd)

	rootCmd.AddCommand(sandboxCmd)
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(guardCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(memCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(driftCmd)
	rootCmd.AddCommand(stackCmd)
	rootCmd.AddCommand(ticketCmd)
	rootCmd.AddCommand(boardCmd)
	rootCmd.AddCommand(routerCmd)
	rootCmd.AddCommand(watchdogCmd)
	rootCmd.AddCommand(mcpCmd)
}

func rejectUnknownSubcommands(cmd *cobra.Command) {
	for _, c := range cmd.Commands() {
		if c.HasSubCommands() {
			c.SilenceUsage = true
			if !c.Runnable() && c.Args == nil {
				c.Args = cobra.NoArgs
				c.RunE = parentHelpRunE
			}
		}
		rejectUnknownSubcommands(c)
	}
}

func parentHelpRunE(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Shows the toolbelt version",
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

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initializes local toolbelt configuration",
	Long: `Creates the .toolbelt/ directory and a config.yaml file in the current directory.

Detects the repository toolchain (go.mod, package.json + lockfiles
pnpm/npm/yarn/bun, pyproject/uv, Cargo.toml, Makefile) and fills in
verify.test_cmd; when there are clear markers it also fills verify.build_cmd and
verify.lint_cmd (build/typecheck/lint scripts, golangci, ruff/mypy,
cargo build/clippy, make targets). Without a test marker, test_cmd stays
empty with a warning — it never assumes go test without go.mod.

If AGENTS.md or CLAUDE.md exist at the repository root, init reads these
files, records their paths in .toolbelt/agent_docs.yaml and extracts short
run hints (ignores long code fences; never overwrites). The
verify.test_cmd binary is checked on PATH with a fallback (e.g. uv → pytest).
Use --write-agents to seed a minimal AGENTS.md that cites the detected
toolchain (verify.test_cmd and, if present, build_cmd/lint_cmd) when the
file does not exist yet.

Example:
  tb init                 Creates a basic configuration
  tb init --write-agents  Also seeds AGENTS.md if missing
  tb init --global        Creates global configuration in ~/.config/toolbelt`,
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

		var docs *config.AgentDocsResult
		var excludePath string
		if !global && repoRoot != "" {
			excludePath, err = config.EnsureLocalGitExclude(repoRoot)
			if err != nil {
				return fmt.Errorf("updating .git/info/exclude: %w", err)
			}
			docs, err = config.HandleAgentDocs(repoRoot, dir, writeAgents)
			if err != nil {
				return fmt.Errorf("processing AGENTS.md/CLAUDE.md: %w", err)
			}
		}

		if jsonOutput {
			payload := buildInitJSON(dir, detected, docs)
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(payload)
		}

		log.Info("configuration initialized", "path", dir)
		if detected.Cmd != "" {
			log.Info("toolchain detected", "test_cmd", detected.Cmd)
		}
		if detected.Warning != "" {
			log.Warn(detected.Warning)
		}

		if excludePath != "" {
			log.Info("local state ignored via .git/info/exclude",
				"path", excludePath,
				"patterns", strings.Join(config.LocalCacheExcludePatterns, ","))
		}

		if docs != nil {
			if docs.Seeded {
				log.Info("AGENTS.md seeded", "path", filepath.Join(repoRoot, config.AgentsMDName))
			}
			if len(docs.Sources) > 0 {
				log.Info("agent documentation found",
					"sources", strings.Join(docs.Sources, ","),
					"record", docs.Record)
				if len(docs.Hints) > 0 {
					log.Info("run hints in AGENTS.md/CLAUDE.md",
						"hints", strings.Join(docs.Hints, " | "))
				}
			} else if writeAgents {
				log.Info("no AGENTS.md/CLAUDE.md after seed")
			} else {
				log.Info("no AGENTS.md/CLAUDE.md at the root; use --write-agents to seed")
			}
		}

		return nil
	},
}

func buildInitJSON(dir string, detected config.DetectedCmd, docs *config.AgentDocsResult) map[string]any {
	payload := map[string]any{
		"path": dir,
	}
	if fileCfg, err := config.LoadFile(filepath.Join(dir, config.DefaultConfigFile)); err == nil {
		payload["test_cmd"] = fileCfg.Verify.TestCmd
		if fileCfg.Verify.BuildCmd != "" {
			payload["build_cmd"] = fileCfg.Verify.BuildCmd
		}
		if fileCfg.Verify.LintCmd != "" {
			payload["lint_cmd"] = fileCfg.Verify.LintCmd
		}
		if fileCfg.Verify.HiddenTestCmd != "" {
			payload["hidden_test_cmd"] = fileCfg.Verify.HiddenTestCmd
		}
	} else if detected.Cmd != "" {
		payload["test_cmd"] = detected.Cmd
	}
	if detected.Warning != "" {
		payload["warning"] = detected.Warning
	}
	if docs != nil {
		sources := docs.Sources
		if sources == nil {
			sources = []string{}
		}
		agents := map[string]any{
			"seeded":  docs.Seeded,
			"sources": sources,
		}
		if len(docs.Hints) > 0 {
			agents["hints"] = docs.Hints
		}
		if docs.Record != "" {
			agents["record"] = docs.Record
		}
		payload["agents"] = agents
	}
	return payload
}

func init() {
	initCmd.Flags().Bool("global", false, "initializes global configuration")
	initCmd.Flags().Bool("write-agents", false, "seeds a minimal AGENTS.md if missing (never overwrites)")
}
