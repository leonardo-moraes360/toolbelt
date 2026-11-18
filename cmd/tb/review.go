package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/review"
)

var reviewCmd = &cobra.Command{
	Use:   "review [target]",
	Short: "Fresh-context review (Elephant-Goldfish)",
	Long: `Reviews a diff against ticket/rd.md and tb mem invariants.

Distinct goldfish (each with fresh context):
  scope       — declared scope and forbidden paths (+ tb mem invariants)
  quality     — TODO/FIXME/XXX/HACK in comments, large functions, code without tests
  acceptance  — acceptance criteria evidenced in tests/files (includes Test*/test/it)

Also reports scope and base commit drift.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReview,
}

func init() {
	reviewCmd.Flags().String("base", "HEAD~1", "commit/ref base")
	reviewCmd.Flags().String("head", "HEAD", "commit/ref head")
	reviewCmd.Flags().String("approved-sha", "", "approved SHA; compares verbatim patch-id (whitespace-sensitive) with head")
	reviewCmd.Flags().String("approved-base", "", "base of the approved range; default: merge-base(approved, base)")
	reviewCmd.Flags().Bool("require-approved-sha", false, "fails if --approved-sha is missing (or review.require_approved_sha)")
	reviewCmd.Flags().String("ticket", "rd.md", "path to the ticket/rd.md")
	reviewCmd.Flags().Int("reviewers", 3, "maximum distinct goldfish (does not clone the same one)")
	reviewCmd.Flags().String("standards", "scope,quality,acceptance", "roles: scope,quality,acceptance (aliases: invariants,completeness)")
	reviewCmd.Flags().String("output", "", "JSON output file")
}

func runReview(cmd *cobra.Command, args []string) error {
	cfg, err := reviewConfigFromFlags(cmd, args)
	if err != nil {
		return err
	}
	rep, err := review.Run(cfg)
	if err != nil {
		return err
	}
	if !rep.FreshOK {
		return fmt.Errorf("fresh context isolation failed; review aborted")
	}
	if err := emitReview(cmd, rep); err != nil {
		return err
	}
	return reviewBlockingError(rep)
}

func reviewConfigFromFlags(cmd *cobra.Command, args []string) (*review.Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	cfg := review.DefaultConfig(cwd)
	cfg.BaseRef, _ = cmd.Flags().GetString("base")
	cfg.BaseExplicit = cmd.Flags().Changed("base")
	cfg.HeadRef, _ = cmd.Flags().GetString("head")
	cfg.ApprovedSHA, _ = cmd.Flags().GetString("approved-sha")
	cfg.ApprovedBase, _ = cmd.Flags().GetString("approved-base")
	cfg.RequireApprovedSHA = requireApprovedSHAFrom(cmd)
	cfg.TicketPath, _ = cmd.Flags().GetString("ticket")
	cfg.Reviewers, _ = cmd.Flags().GetInt("reviewers")
	stds, _ := cmd.Flags().GetString("standards")
	cfg.Standards = splitCSV(stds)
	if len(args) > 0 {
		cfg.TicketPath = args[0]
	}
	return &cfg, nil
}

func emitReview(cmd *cobra.Command, rep *review.Report) error {
	outPath, _ := cmd.Flags().GetString("output")
	if !(jsonOutput || outPath != "") {
		printReviewReport(rep)
		return nil
	}
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	if outPath != "" {
		return os.WriteFile(outPath, data, 0o600)
	}
	fmt.Println(string(data))
	return nil
}

func reviewBlockingError(rep *review.Report) error {
	if pinnedApprovalMismatch(rep.PinnedApproval) {
		return fmt.Errorf("head patch diverges from the approved SHA")
	}
	if rep.Drift != nil && (len(rep.Drift.OutOfScope) > 0 || len(rep.Drift.ForbiddenHit) > 0 || rep.Drift.BaseMismatch) {
		return fmt.Errorf("drift detected in scope/base")
	}
	for i := range rep.Findings {
		f := &rep.Findings[i]
		if f.Severity == review.SeverityCritical || f.Severity == review.SeverityError {
			return fmt.Errorf("review found blocking findings")
		}
	}
	return nil
}

func pinnedApprovalMismatch(p *review.PinnedApproval) bool {
	return p.Diverged()
}

func printReviewReport(rep *review.Report) {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf(" REVIEW REPORT: %s\n", rep.Target)
	fmt.Printf(" Base..Head: %s..%s | Reviewers: %d | Fresh: %v\n", rep.BaseRef, rep.HeadRef, rep.Reviewers, rep.FreshOK)
	fmt.Println("═══════════════════════════════════════════════════════════")
	printPinnedApproval(rep.PinnedApproval)
	printDriftSection(rep.Drift)
	if len(rep.Findings) == 0 {
		fmt.Println("No findings.")
		return
	}
	fmt.Println("FINDINGS:")
	for i := range rep.Findings {
		f := &rep.Findings[i]
		role := f.Role
		if role == "" {
			role = f.Category
		}
		fmt.Printf("  [%s] (%s/%s) %s", f.Severity, f.Reviewer, f.Category, f.Message)
		if f.File != "" {
			fmt.Printf(" [%s]", f.File)
		}
		if f.Agreement > 1 {
			fmt.Printf("  role=%s agreeing_roles=%d\n", role, f.Agreement)
		} else {
			fmt.Printf("  role=%s\n", role)
		}
	}
}

func printPinnedApproval(p *review.PinnedApproval) {
	if p == nil {
		return
	}
	fmt.Println("PINNED APPROVAL:")
	fmt.Printf("  checked: %v\n", p.Checked)
	if !p.Checked {
		fmt.Println()
		return
	}
	fmt.Printf("  approved_sha: %s\n", p.ApprovedSHA)
	fmt.Printf("  approved_base: %s\n", p.ApprovedBase)
	fmt.Printf("  head_base: %s\n", p.HeadBase)
	fmt.Printf("  patch_id: %s\n", p.PatchID)
	fmt.Printf("  head_patch_id: %s\n", p.HeadPatchID)
	if p.Match != nil {
		fmt.Printf("  match: %v\n", *p.Match)
	}
	if p.Diverged() && p.RangeDiffSummary != "" {
		fmt.Println("  range-diff:")
		for _, line := range strings.Split(p.RangeDiffSummary, "\n") {
			fmt.Printf("    %s\n", line)
		}
	}
	fmt.Println()
}

func printDriftSection(d *review.DriftReport) {
	if d == nil {
		return
	}
	fmt.Println("DRIFT:")
	if d.BaseMismatch {
		fmt.Printf("  base mismatch: declared=%s actual=%s\n", d.DeclaredBase, d.ActualBase)
	}
	for _, f := range d.OutOfScope {
		fmt.Printf("  out of scope: %s\n", f)
	}
	for _, f := range d.ForbiddenHit {
		fmt.Printf("  forbidden: %s\n", f)
	}
	if !d.BaseMismatch && len(d.OutOfScope) == 0 && len(d.ForbiddenHit) == 0 {
		fmt.Println("  none")
	}
	fmt.Println()
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func requireApprovedSHAFrom(cmd *cobra.Command) bool {
	if v, _ := cmd.Flags().GetBool("require-approved-sha"); v {
		return true
	}
	fileCfg, err := config.Load()
	if err != nil {
		return false
	}
	return fileCfg.Review.RequireApprovedSHA
}
