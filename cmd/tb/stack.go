package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/stack"
)

var stackCmd = &cobra.Command{
	Use:   "stack <op> [<op>...]",
	Short: "Stacks verification operations (verify/review/drift/guard/egm), not branches",
	Long: `Stacks verification operations in a single run with shared context
(diff, index of changed files, memory and config).

Not a branch stack (like Graphite or git-stack): it only orchestrates verify,
review, drift, guard and/or egm in the given order.

Operations: verify, review, drift, guard, egm.
egm gates the specs/ADRs (egm.doc_globs) added or modified in
merge-base(base, head)..head with tb egm status --at head; it is local (state
lives in $XDG_STATE_HOME), not parallel, and always runs last.
Read-only stages (guard, drift, review) run in parallel; verify stays isolated
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
	stackCmd.Flags().Bool("dry-run", false, "verify: shows the plan (cmds/dir) without executing")
	stackCmd.Flags().Int("max-iter", 0, "verify: maximum iterations")
	stackCmd.Flags().String("timeout", "10m", "verify: timeout")
	stackCmd.Flags().Int("reviewers", 0, "review: maximum goldfish")
	stackCmd.Flags().String("standards", "", "review: roles CSV (scope,quality,acceptance)")
	stackCmd.Flags().StringArray("egm-doc", nil, "egm: document added to the selection (repeatable)")
	stackCmd.Flags().Bool("strict", false, "egm: PASS_PARTIAL and an empty selection fail")
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
	testCmd, _ := cmd.Flags().GetString("test-cmd")
	hiddenCmd, _ := cmd.Flags().GetString("hidden-test-cmd")
	buildCmd, _ := cmd.Flags().GetString("build-cmd")
	lintCmd, _ := cmd.Flags().GetString("lint-cmd")
	noBuild, _ := cmd.Flags().GetBool("no-build")
	noLint, _ := cmd.Flags().GetBool("no-lint")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	maxIter, _ := cmd.Flags().GetInt("max-iter")
	reviewers, _ := cmd.Flags().GetInt("reviewers")
	standardsCSV, _ := cmd.Flags().GetString("standards")
	egmDocs, _ := cmd.Flags().GetStringArray("egm-doc")
	strict, _ := cmd.Flags().GetBool("strict")

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
		TestCmd:            testCmd,
		HiddenTestCmd:      hiddenCmd,
		BuildCmd:           buildCmd,
		LintCmd:            lintCmd,
		BuildCmdSet:        cmd.Flags().Changed("build-cmd"),
		LintCmdSet:         cmd.Flags().Changed("lint-cmd"),
		NoBuild:            noBuild,
		NoLint:             noLint,
		DryRun:             dryRun,
		MaxIter:            maxIter,
		Timeout:            timeout,
		Reviewers:          reviewers,
		Standards:          splitCSV(standardsCSV),
		EnvAllow:           envAllow,
		EnvPass:            envPass,
		EgmDocs:            egmDocs,
		Strict:             strict,
	}
	res, err := stack.Run(context.Background(), opts)
	if err != nil {
		return err
	}
	return emitStackResult(res)
}

func emitStackResult(res *stack.Result) error {
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	if code := stack.ExitCode(res); code != 0 {
		os.Exit(code)
	}
	return nil
}
