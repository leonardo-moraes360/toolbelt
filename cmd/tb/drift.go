package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/envelope"
	"github.com/leonardo-moraes360/toolbelt/internal/review"
)

var driftCmd = &cobra.Command{
	Use:   "drift [ticket]",
	Short: "Detects scope and base commit drift",
	Long: `Compares files changed in base...head (three-dot / merge-base)
against the scope declared in the ticket/rd.md and checks that the base is an ancestor of head.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDrift,
}

func init() {
	driftCmd.Flags().String("base", "HEAD~1", "base commit/ref")
	driftCmd.Flags().String("head", "HEAD", "head commit/ref")
	driftCmd.Flags().String("approved-sha", "", "approved SHA; compares verbatim patch-id (whitespace-sensitive) with head")
	driftCmd.Flags().String("approved-base", "", "base of the approved range; default: merge-base(approved, base)")
	driftCmd.Flags().Bool("require-approved-sha", false, "fails if --approved-sha is missing (or review.require_approved_sha)")
	driftCmd.Flags().String("ticket", "rd.md", "path to the ticket/rd.md")
	driftCmd.Flags().String("output", "", "JSON output file")
}

func runDrift(cmd *cobra.Command, args []string) error {
	cfg, err := driftConfigFromFlags(cmd, args)
	if err != nil {
		return err
	}
	rep, err := review.Run(cfg)
	if err != nil {
		return err
	}
	if err := emitDrift(cmd, rep.Drift); err != nil {
		return err
	}
	return driftBlockingError(rep.Drift)
}

func driftConfigFromFlags(cmd *cobra.Command, args []string) (*review.Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ticketPath, _ := cmd.Flags().GetString("ticket")
	if len(args) > 0 {
		ticketPath = args[0]
	}
	base, _ := cmd.Flags().GetString("base")
	head, _ := cmd.Flags().GetString("head")
	approved, _ := cmd.Flags().GetString("approved-sha")
	approvedBase, _ := cmd.Flags().GetString("approved-base")
	baseSet := cmd.Flags().Changed("base")

	ticket, err := review.LoadTicket(ticketPath)
	if err != nil {
		return nil, err
	}
	if !baseSet && ticket != nil && ticket.BaseCommit != "" {
		base = ticket.BaseCommit
	}

	cfg := review.DefaultConfig(cwd)
	cfg.BaseRef = base
	cfg.BaseExplicit = baseSet
	cfg.HeadRef = head
	cfg.ApprovedSHA = approved
	cfg.ApprovedBase = approvedBase
	cfg.RequireApprovedSHA = requireApprovedSHAFrom(cmd)
	cfg.TicketPath = ticketPath
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	return &cfg, nil
}

func emitDrift(cmd *cobra.Command, drift *review.DriftReport) error {
	outPath, _ := cmd.Flags().GetString("output")
	if jsonOutput || outPath != "" {
		return emitJSONOrFile(envelope.Drift, drift, outPath)
	}
	printDriftReport(drift)
	return nil
}

func printDriftReport(drift *review.DriftReport) {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println(" DRIFT REPORT")
	fmt.Println("═══════════════════════════════════════════════════════════")
	if drift == nil {
		fmt.Println("No drift data.")
		return
	}
	printPinnedApproval(drift.PinnedApproval)
	fmt.Printf("Base: declared=%s actual=%s merge_base=%s\n", drift.DeclaredBase, drift.ActualBase, drift.MergeBase)
	if drift.BaseMismatch {
		fmt.Println("⚠ base_commit/base is not an ancestor of head (merge-base --is-ancestor)")
	}
	fmt.Printf("Out of scope (%d):\n", len(drift.OutOfScope))
	for _, f := range drift.OutOfScope {
		fmt.Printf("  - %s\n", f)
	}
	fmt.Printf("Forbidden touched (%d):\n", len(drift.ForbiddenHit))
	for _, f := range drift.ForbiddenHit {
		fmt.Printf("  - %s\n", f)
	}
	if len(drift.OutOfScope) == 0 && len(drift.ForbiddenHit) == 0 && !drift.BaseMismatch &&
		!pinnedApprovalMismatch(drift.PinnedApproval) {
		fmt.Println("OK: no scope/base drift")
	}
}

func driftBlockingError(drift *review.DriftReport) error {
	if drift == nil {
		return nil
	}
	if pinnedApprovalMismatch(drift.PinnedApproval) {
		return fmt.Errorf("head patch diverges from the approved SHA")
	}
	if len(drift.OutOfScope) > 0 || len(drift.ForbiddenHit) > 0 || drift.BaseMismatch {
		return fmt.Errorf("drift detected")
	}
	return nil
}
