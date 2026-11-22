package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
	_ "github.com/leonardo-moraes360/toolbelt/internal/registry"
	"github.com/leonardo-moraes360/toolbelt/internal/vault"
	"github.com/leonardo-moraes360/toolbelt/internal/verify"
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
		os.Exit(exitCodeFor(err))
	}
}

// exitCodeFor maps a typed usage error (bad --ruler, unknown --no-check) to
// 2; every other error keeps 1.
func exitCodeFor(err error) int {
	if verify.IsUsageError(err) || errors.Is(err, bus.ErrSessionInUse) {
		return 2
	}
	return 1
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

		if cmd.Name() != "init" && cmd.Name() != "version" && cmd.Name() != "help" && !isEgmCommand(cmd) {
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
			fmt.Printf("{\"version\":%q,\"commit\":%q,\"build_time\":%q,\"go_version\":%q,\"embed\":%q}\n",
				info.Version, info.Commit, info.BuildTime, info.GoVersion, info.Embed)
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
verify.test_cmd; when there are clear markers it also fills verify.build_cmd,
verify.typecheck_cmd and verify.lint_cmd (build/lint scripts, golangci, ruff,
cargo build/clippy, make targets; typecheck: a Node typecheck script or
tsconfig.json with a local tsc, mypy/pyright configured and on PATH). Without a test marker, test_cmd stays
empty with a warning — it never assumes go test without go.mod.

If AGENTS.md or CLAUDE.md exist at the repository root, init reads these
files, records their paths in .toolbelt/agent_docs.yaml and extracts short
run hints (ignores long code fences; never overwrites). The
verify.test_cmd binary is checked on PATH with a fallback (e.g. uv → pytest).
Use --write-agents to seed a minimal AGENTS.md that cites the detected
toolchain (verify.test_cmd and, if present, build_cmd/lint_cmd) when the
file does not exist yet.

Named checks (verify.checks) are suggested, never added silently: govulncheck
for Go (when on PATH) and clippy for Rust (when cargo clippy -V works). Use
--write-checks to write them into a new config.yaml.

Example:
  tb init                 Creates a basic configuration
  tb init --write-agents  Also seeds AGENTS.md if missing
  tb init --write-checks  Also writes the suggested verify.checks
  tb init --global        Creates global configuration in ~/.config/toolbelt`,
	RunE: func(cmd *cobra.Command, args []string) error {
		global, _ := cmd.Flags().GetBool("global")
		writeAgents, _ := cmd.Flags().GetBool("write-agents")
		writeChecks, _ := cmd.Flags().GetBool("write-checks")

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

		initRes, err := config.InitWithOptions(dir, config.InitOptions{WriteChecks: writeChecks})
		if err != nil {
			return fmt.Errorf("initializing config: %w", err)
		}
		detected := initRes.Test

		var docs *config.AgentDocsResult
		var excludePath string
		var vaultInit initVault
		if !global && repoRoot != "" {
			vaultInit, err = ensureInitVault(repoRoot, dir)
			if err != nil {
				return err
			}
			excludePath, err = config.EnsureLocalGitExclude(repoRoot, vaultInit.exclude...)
			if err != nil {
				return fmt.Errorf("updating .git/info/exclude: %w", err)
			}
			vaultInit.gitignored = vaultGitignored(repoRoot, vaultInit.path)
			docs, err = config.HandleAgentDocs(repoRoot, dir, writeAgents)
			if err != nil {
				return fmt.Errorf("processing AGENTS.md/CLAUDE.md: %w", err)
			}
		}

		if jsonOutput {
			payload := buildInitJSON(dir, detected, docs)
			if suggested := unwrittenChecks(initRes); len(suggested) > 0 {
				payload["suggested_checks"] = suggested
			}
			if vaultInit.gitignored {
				payload["vault_gitignored"] = vaultInit.path
			}
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
		logInitChecks(initRes)

		if excludePath != "" {
			log.Info("local state ignored via .git/info/exclude",
				"path", excludePath,
				"patterns", strings.Join(append(append([]string{}, config.LocalCacheExcludePatterns...), vaultInit.exclude...), ","))
		}
		if vaultInit.gitignored {
			log.Warn("vault_gitignored: an ignore rule (.gitignore, info/exclude or a global excludes file) ignores the vault, so notes are not versioned; remove it to share them",
				"vault", vaultInit.path)
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
		addInitVerifyJSON(payload, &fileCfg.Verify)
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
	initCmd.Flags().Bool("write-checks", false, "writes the suggested verify.checks (govulncheck, clippy) into a new config.yaml")
}

// addInitVerifyJSON copies the verify commands init wrote; empty ones are omitted
// except test_cmd, which is always present.
func addInitVerifyJSON(payload map[string]any, v *config.VerifyConfig) {
	payload["test_cmd"] = v.TestCmd
	for key, val := range map[string]string{
		"build_cmd":       v.BuildCmd,
		"typecheck_cmd":   v.TypecheckCmd,
		"lint_cmd":        v.LintCmd,
		"hidden_test_cmd": v.HiddenTestCmd,
	} {
		if val != "" {
			payload[key] = val
		}
	}
	if len(v.Checks) > 0 {
		payload["checks"] = initChecksJSON(v.Checks)
	}
}

// unwrittenChecks are the detected checks init only suggests: all of them
// without --write-checks, or when config.yaml already existed.
func unwrittenChecks(res config.InitResult) []map[string]string {
	if res.ChecksWritten {
		return nil
	}
	return initChecksJSON(res.SuggestedChecks)
}

func initChecksJSON(checks []config.VerifyCheck) []map[string]string {
	if len(checks) == 0 {
		return nil
	}
	out := make([]map[string]string, 0, len(checks))
	for _, c := range checks {
		out = append(out, map[string]string{"name": c.Name, "cmd": c.Cmd})
	}
	return out
}

func logInitChecks(res config.InitResult) {
	for _, c := range res.SuggestedChecks {
		switch {
		case res.ChecksWritten:
			log.Info("check written", "name", c.Name, "cmd", c.Cmd)
		case res.Created:
			log.Info("suggested check (use --write-checks to add it)", "name", c.Name, "cmd", c.Cmd)
		default:
			log.Info("suggested check (config.yaml exists; add it under verify.checks)", "name", c.Name, "cmd", c.Cmd)
		}
	}
}

// initVault is what tb init does for the vault (docs/specs/vault.md, R2).
type initVault struct {
	path       string
	exclude    []string
	gitignored bool
}

// ensureInitVault reads vault.path from the repository config tb init just
// wrote or found, creates a non-default vault directory, and returns the
// .obsidian/ exclude for it. An invalid vault.path is reported and skipped.
func ensureInitVault(repoRoot, dir string) (initVault, error) {
	cfg, err := config.LoadFile(filepath.Join(dir, config.DefaultConfigFile))
	if err != nil {
		log.Warn("vault config ignored: config unreadable", "error", err)
		return initVault{}, nil
	}
	v := cfg.Vault
	if err := v.Validate(); err != nil {
		log.Warn("vault config ignored", "error", err)
		return initVault{}, nil
	}
	// One component at a time, refusing symlinks: a cloned repository
	// cannot make tb init create the vault outside it.
	if err := vault.EnsureDir(repoRoot, v.Path); err != nil {
		return initVault{}, fmt.Errorf("creating vault %s: %w", v.Path, err)
	}
	return initVault{path: filepath.ToSlash(filepath.Clean(v.Path)), exclude: []string{config.VaultExcludePattern(v.Path)}}, nil
}

// vaultGitignored reports whether git ignores a note in the vault: after
// tb init's exclude no longer lists the vault, that is a .gitignore (or
// global excludes) rule. Without git, it reports false.
func vaultGitignored(repoRoot, vaultPath string) bool {
	if vaultPath == "" {
		return false
	}
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return false
	}
	probe := path.Join(vaultPath, "tb-probe-note.md")
	cmd := exec.Command(gitBin, "-C", repoRoot, "check-ignore", "-q", "--no-index", "--", probe) //nolint:gosec // fixed git subcommand, probe path from validated config
	return cmd.Run() == nil
}
