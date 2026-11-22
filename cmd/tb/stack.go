package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/envelope"
	"github.com/leonardo-moraes360/toolbelt/internal/stack"
	"github.com/leonardo-moraes360/toolbelt/internal/verify"
)

var stackCmd = &cobra.Command{
	Use:   "stack <op> [<op>...]",
	Short: "Stacks verification operations (verify/review/drift/guard/vault/egm/seccheck), not branches",
	Long: `Stacks verification operations in a single run with shared context
(diff, index of changed files, memory and config).

Not a branch stack (like Graphite or git-stack): it only orchestrates verify,
review, drift, guard and/or egm in the given order.

Operations: verify, review, drift, guard, vault, egm, seccheck.
egm gates the specs/ADRs (egm.doc_globs) added or modified in
merge-base(base, head)..head with tb egm status --at head; it is local (state
lives in $XDG_STATE_HOME), not parallel, and always runs last. With
--egm-mode ci (needs -c) it instead checks each document's committed receipt
and runs one fresh review round with CI's own config (docs/specs/egm-trust.md).
seccheck runs the registered AppSec checks (.toolbelt/secchecks.yaml, from
tb blue apply) over the working tree; regex-only, read-only, and fails on
error-severity findings unless --warn-only.
Read-only stages (guard, drift, review, vault, seccheck) run in parallel; verify stays isolated
(build/test). The JSON output preserves the requested order. --jobs N limits concurrency
(default = number of CPUs, max 32; --jobs 1 = serial). --fail-fast cancels the others
via context. The output is always a single aggregated JSON.
Without an operation, exits with a short error listing the valid ops (no Usage dump).`,
	RunE: runStack,
}

func init() {
	stackCmd.Flags().String("base", "HEAD~1", "shared base ref")
	stackCmd.Flags().String("head", "HEAD", "shared head ref")
	stackCmd.Flags().Bool("fail-fast", false, "cancels the remaining stages on the first failure")
	stackCmd.Flags().Int("jobs", 0, "concurrency limit (0=number of CPUs, max 32; 1=serial)")
	stackCmd.Flags().String("ticket", "rd.md", "path to the ticket/rd.md (review/drift/verify)")
	stackCmd.Flags().String("approved-sha", "", "approved SHA; verbatim patch-id (review/drift)")
	stackCmd.Flags().String("approved-base", "", "base of the approved range; default: merge-base(approved, base)")
	stackCmd.Flags().Bool("require-approved-sha", false, "fails if --approved-sha is missing (or review.require_approved_sha)")
	stackCmd.Flags().Bool("warn-only", false, "guard: do not fail the exit code on SeverityError")
	stackCmd.Flags().String("test-cmd", "", "verify: visible tests command line")
	stackCmd.Flags().String("hidden-test-cmd", "", "verify: hidden tests command line")
	stackCmd.Flags().String("build-cmd", "", "verify: build command (empty + explicit flag or --no-build to skip)")
	stackCmd.Flags().String("lint-cmd", "", "verify: lint command (empty + explicit flag or --no-lint to skip)")
	stackCmd.Flags().Bool("no-build", false, "verify: skips build_cmd")
	stackCmd.Flags().Bool("no-lint", false, "verify: skips lint_cmd")
	stackCmd.Flags().String("ruler", "tests", "verify: rulers, comma-separated (tests|coverage; visual is refused here)")
	stackCmd.Flags().String("typecheck-cmd", "", "verify: typecheck command (empty + explicit flag or --no-typecheck to skip)")
	stackCmd.Flags().Bool("no-typecheck", false, "verify: skips typecheck_cmd")
	stackCmd.Flags().StringArray("check", nil, "verify: named check name=cmd (repeatable; adds one or overrides its cmd)")
	stackCmd.Flags().StringArray("no-check", nil, "verify: skips the named check (repeatable)")
	stackCmd.Flags().Bool("no-checks", false, "verify: skips every verify.checks entry")
	addCoverageFlags(stackCmd, "verify: ")
	stackCmd.Flags().Bool("dry-run", false, "verify: shows the plan (cmds/dir) without executing")
	stackCmd.Flags().Int("max-iter", 0, "verify: maximum iterations")
	stackCmd.Flags().String("timeout", "10m", "verify: timeout")
	stackCmd.Flags().Int("reviewers", 0, "review: maximum goldfish")
	stackCmd.Flags().String("standards", "", "review: roles CSV (scope,quality,acceptance)")
	stackCmd.Flags().StringArray("egm-doc", nil, "egm: document added to the selection (repeatable)")
	stackCmd.Flags().Bool("strict", false, "egm: PASS_PARTIAL and an empty selection fail (in ci, PASS_PARTIAL always fails; an empty selection only with --strict)")
	stackCmd.Flags().String("egm-mode", "local", "egm: local | ci (ci needs -c, fails PASS_PARTIAL and runs a fresh round with CI's credential)")
	stackCmd.Flags().Bool("egm-allow-offline", false, "egm ci: without a credential, run a deterministic-only round capped at PASS_PARTIAL")
}

func runStack(cmd *cobra.Command, args []string) error {
	ops, err := stack.ParseOps(args)
	if err != nil {
		op := ""
		if len(args) > 0 {
			op = args[0]
		}
		return emitStackResult(&stack.Result{
			OK: false,
			Steps: []stack.StepResult{{
				Op:       op,
				OK:       false,
				ExitCode: 1,
				Error:    err.Error(),
			}},
		})
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	timeoutStr, _ := cmd.Flags().GetString("timeout")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}
	base, _ := cmd.Flags().GetString("base")
	head, _ := cmd.Flags().GetString("head")
	failFast, _ := cmd.Flags().GetBool("fail-fast")
	jobs, _ := cmd.Flags().GetInt("jobs")
	ticket, _ := cmd.Flags().GetString("ticket")
	approved, _ := cmd.Flags().GetString("approved-sha")
	approvedBase, _ := cmd.Flags().GetString("approved-base")
	warnOnly, _ := cmd.Flags().GetBool("warn-only")
	vo, err := verifyFlagOverrides(cmd)
	if err != nil {
		return err
	}
	ruler, _ := cmd.Flags().GetString("ruler")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	maxIter, _ := cmd.Flags().GetInt("max-iter")
	reviewers, _ := cmd.Flags().GetInt("reviewers")
	standardsCSV, _ := cmd.Flags().GetString("standards")
	egmDocs, _ := cmd.Flags().GetStringArray("egm-doc")
	strict, _ := cmd.Flags().GetBool("strict")
	egmMode, _ := cmd.Flags().GetString("egm-mode")
	egmOffline, _ := cmd.Flags().GetBool("egm-allow-offline")

	opts := &stack.Options{
		RepoDir:            cwd,
		Base:               base,
		Head:               head,
		Ops:                ops,
		FailFast:           failFast,
		Jobs:               jobs,
		Ticket:             ticket,
		ApprovedSHA:        approved,
		ApprovedBase:       approvedBase,
		RequireApprovedSHA: requireApprovedSHAFrom(cmd),
		WarnOnly:           warnOnly,
		TestCmd:            vo.TestCmd.Cmd,
		HiddenTestCmd:      vo.HiddenTestCmd.Cmd,
		BuildCmd:           vo.BuildCmd.Cmd,
		LintCmd:            vo.LintCmd.Cmd,
		BuildCmdSet:        vo.BuildCmd.Set,
		LintCmdSet:         vo.LintCmd.Set,
		NoBuild:            vo.NoBuild,
		NoLint:             vo.NoLint,
		Ruler:              ruler,
		TypecheckCmd:       vo.TypecheckCmd.Cmd,
		TypecheckCmdSet:    vo.TypecheckCmd.Set,
		NoTypecheck:        vo.NoTypecheck,
		Checks:             vo.Checks,
		SkipChecks:         vo.SkipChecks,
		NoChecks:           vo.NoChecks,
		CoverageCmd:        vo.CoverageCmd.Cmd,
		CoverageCmdSet:     vo.CoverageCmd.Set,
		CoveragePattern:    vo.CoveragePattern.Cmd,
		CoveragePatternSet: vo.CoveragePattern.Set,
		MinCoverage:        vo.MinCoverage,
		DryRun:             dryRun,
		MaxIter:            maxIter,
		Timeout:            timeout,
		Reviewers:          reviewers,
		Standards:          splitCSV(standardsCSV),
		EnvAllow:           envAllow,
		EnvPass:            envPass,
		EgmDocs:            egmDocs,
		Strict:             strict,
		EgmMode:            egmMode,
		EgmConfig:          cfgFile,
		EgmAllowOffline:    egmOffline,
	}
	res, err := stack.Run(context.Background(), opts)
	if err != nil {
		return err
	}
	return emitStackResult(res)
}

func emitStackResult(res *stack.Result) error {
	if err := emitJSONIndent(envelope.Stack, res); err != nil {
		return err
	}
	if !jsonOutput {
		printStackVerifyNotices(res)
	}
	if code := stack.ExitCode(res); code != 0 {
		os.Exit(code)
	}
	return nil
}

// printStackVerifyNotices prints the verify step's warnings (ignored keys,
// base == head, symlinked config) and applied overrides to stderr.
func printStackVerifyNotices(res *stack.Result) {
	for _, step := range res.Steps {
		if step.Op != string(stack.OpVerify) {
			continue
		}
		switch p := step.Result.(type) {
		case *verify.Result:
			printVerifyNotices(p.Warnings, p.Overrides)
		case *verify.MultiResult:
			printVerifyNotices(p.Warnings, p.Overrides)
		}
	}
}
