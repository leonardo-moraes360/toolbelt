package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/egm"
)

var (
	egmEngine    string
	egmReviewers []string
	egmDryRun    bool
	egmRestart   bool
	egmStrict    bool
	egmAt        string
	egmResolve   egm.ResolveOptions
)

var egmCmd = &cobra.Command{
	Use:   "egm",
	Short: "Reviews design documents until the review converges (Elephant-Goldfish Model)",
	Long: `tb egm reviews specs and ADRs with isolated reviewers and repeats the round
until it converges over the same content (docs/specs/egm-nativo.md).

Two engines (--engine auto|llm|deterministic). Both run the checkers
(refs-exist, required-sections, plan-files-listed) and the anti-injection scan.
The llm engine also runs the 9 reviewers of the consolidated preset as
goldfish: one-shot egm.agent_cmd processes in a Landlock sandbox with a
read-only packet, a disposable HOME, a minimal env and no tools (Linux).
Without egm.agent_cmd, auto uses the deterministic engine, where the LLM
personas are unavailable and the best verdict is PASS_PARTIAL.

State lives outside the repository, in $XDG_STATE_HOME/tb/egm/<repo-id>/<slug>/.
The config comes from the user config (or -c), never from the merged config;
the repository's .toolbelt/config.yaml can only harden it.`,
}

var egmReviewCmd = &cobra.Command{
	Use:   "review <doc>",
	Short: "Runs round 1 of the first cycle (or a new cycle with --restart)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEgm(cmd, func(ctx context.Context, o egm.Options) (*egm.Outcome, error) {
			o.Restart = egmRestart
			return egm.Review(ctx, &o)
		}, args[0])
	},
}

var egmLoopCmd = &cobra.Command{
	Use:   "loop",
	Short: "Advances the review loop",
}

var egmLoopNextCmd = &cobra.Command{
	Use:   "next <doc>",
	Short: "Runs the next round; refused when nothing changed and blocking findings are open",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEgm(cmd, func(ctx context.Context, o egm.Options) (*egm.Outcome, error) {
			return egm.LoopNext(ctx, &o)
		}, args[0])
	},
}

var egmResolveCmd = &cobra.Command{
	Use:   "resolve <doc>",
	Short: "Marks lineages as fixed, declined or force-settled",
	Example: `  tb egm resolve docs/specs/x.md --fix F-1,F-2
  tb egm resolve docs/specs/x.md --decline F-4
  tb egm resolve docs/specs/x.md --force-settle F-3 --reason "accepted risk"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEgm(cmd, func(ctx context.Context, o egm.Options) (*egm.Outcome, error) {
			r := egmResolve
			r.Options = o
			return egm.Resolve(ctx, &r)
		}, args[0])
	},
}

var egmStatusCmd = &cobra.Command{
	Use:   "status <doc>",
	Short: "Shows the verdict for the worktree content, or a commit with --at (exit 0 PASS, 1 FAIL, 3 not passing)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEgm(cmd, func(ctx context.Context, o egm.Options) (*egm.Outcome, error) {
			o.Strict, o.At = egmStrict, egmAt
			return egm.Status(ctx, &o)
		}, args[0])
	},
}

var egmReceiptCmd = &cobra.Command{
	Use:   "receipt <doc>",
	Short: "Writes the committed receipt of the author's claims, read by the CI mode (needs PASS or PASS_PARTIAL)",
	Long: `tb egm receipt writes .toolbelt/egm/receipts/<slug>.json: the document and
source hashes plus the claims (lineages at major or above that the arbiter
withdrew, that were declined or force-settled), each with lines in the current
document (docs/specs/egm-trust.md, T-R1). Commit it with the document. A
receipt is a claim, not a proof: the CI mode re-checks every field.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEgm(cmd, func(ctx context.Context, o egm.Options) (*egm.Outcome, error) {
			return egm.WriteReceipt(ctx, &o)
		}, args[0])
	},
}

func init() {
	for _, c := range []*cobra.Command{egmReviewCmd, egmLoopNextCmd} {
		c.Flags().StringVar(&egmEngine, "engine", egm.EngineAuto, "auto | llm | deterministic")
		c.Flags().StringSliceVar(&egmReviewers, "reviewers", nil, "restricts the reviewers (coverage becomes partial)")
		c.Flags().BoolVar(&egmDryRun, "dry-run", false, "prints the worst-case agent runs and the budget; writes nothing")
	}
	egmReviewCmd.Flags().BoolVar(&egmRestart, "restart", false, "opens a new cycle (needs a changed content; downgrades the cycle)")
	egmStatusCmd.Flags().BoolVar(&egmStrict, "strict", false, "PASS_PARTIAL exits 1")
	egmStatusCmd.Flags().StringVar(&egmAt, "at", "", "gate mode: evaluates the document, sources and config at this commit (R7a)")
	f := egmResolveCmd.Flags()
	f.StringSliceVar(&egmResolve.Fix, "fix", nil, "lineages fixed by an edit (F-1,F-2)")
	f.StringSliceVar(&egmResolve.Decline, "decline", nil, "nit/major lineages declined (a major is a downgrade)")
	f.StringVar(&egmResolve.ForceSettle, "force-settle", "", "lineage settled without a fix (downgrade; needs --reason)")
	f.StringVar(&egmResolve.Reason, "reason", "", "reason for --force-settle")
	f.StringVar(&egmResolve.Contest, "contest", "", "lineage contested at the arbiter, which runs on the next loop next (llm engine)")
	f.StringVar(&egmResolve.Evidence, "evidence", "", "path:line of the contest evidence")

	egmLoopCmd.AddCommand(egmLoopNextCmd)
	egmCmd.AddCommand(egmReviewCmd, egmLoopCmd, egmResolveCmd, egmStatusCmd, egmReceiptCmd)
	rootCmd.AddCommand(egmCmd)
}

// isEgmCommand reports whether cmd is under `tb egm`; the root then skips
// config.Load (R31). The test is the command chain, not the leaf name, which
// collides with `tb review` and others.
func isEgmCommand(cmd *cobra.Command) bool {
	return strings.HasPrefix(cmd.CommandPath(), "tb egm")
}

func runEgm(cmd *cobra.Command, fn func(context.Context, egm.Options) (*egm.Outcome, error), doc string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	o := egm.Options{Doc: doc, Cwd: cwd, ConfigFile: cfgFile, Engine: egmEngine, Reviewers: egmReviewers, DryRun: egmDryRun}
	out, err := fn(ctx, o)
	stop()
	if err != nil {
		return egmFail(err)
	}
	printEgm(out)
	if out.ExitCode != 0 {
		os.Exit(out.ExitCode)
	}
	return nil
}

func egmFail(err error) error {
	code, reason := egm.ExitError, ""
	var e *egm.Error
	if errors.As(err, &e) {
		code, reason = e.Code, e.Reason
	}
	if jsonOutput {
		printErrorBody(err, reason, code)
	} else {
		fmt.Fprintln(os.Stderr, "tb egm:", err)
	}
	os.Exit(code)
	return nil
}

func printEgm(out *egm.Outcome) {
	if jsonOutput {
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return
	}
	if out.Report != nil {
		fmt.Print(out.Report.Markdown())
	}
	if out.Message != "" {
		fmt.Println()
		fmt.Println(out.Message)
	}
}
