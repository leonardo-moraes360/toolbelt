package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/review"
)

var driftCmd = &cobra.Command{
	Use:   "drift [ticket]",
	Short: "Detecta drift de escopo e commit base",
	Long: `Compara arquivos alterados em base..head com o scope declarado
no ticket/rd.md e verifica divergência do commit base.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDrift,
}

func init() {
	driftCmd.Flags().String("base", "HEAD~1", "commit/ref base")
	driftCmd.Flags().String("head", "HEAD", "commit/ref head")
	driftCmd.Flags().String("approved-sha", "", "SHA aprovado; compara patch-id verbatim (sensível a whitespace) com o head")
	driftCmd.Flags().Bool("require-approved-sha", false, "falha se --approved-sha estiver ausente (ou review.require_approved_sha)")
	driftCmd.Flags().String("ticket", "rd.md", "caminho do ticket/rd.md")
	driftCmd.Flags().String("output", "", "arquivo de saída JSON")
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

	ticket, err := review.LoadTicket(ticketPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if ticket != nil && ticket.BaseCommit != "" && base == "HEAD~1" {
		base = ticket.BaseCommit
	}

	cfg := review.DefaultConfig(cwd)
	cfg.BaseRef = base
	cfg.HeadRef = head
	cfg.ApprovedSHA = approved
	cfg.RequireApprovedSHA = requireApprovedSHAFrom(cmd)
	cfg.TicketPath = ticketPath
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	return &cfg, nil
}

func emitDrift(cmd *cobra.Command, drift *review.DriftReport) error {
	outPath, _ := cmd.Flags().GetString("output")
	if jsonOutput || outPath != "" {
		data, err := json.MarshalIndent(drift, "", "  ")
		if err != nil {
			return err
		}
		if outPath != "" {
			return os.WriteFile(outPath, data, 0o600)
		}
		fmt.Println(string(data))
		return nil
	}
	printDriftReport(drift)
	return nil
}

func printDriftReport(drift *review.DriftReport) {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println(" RELATÓRIO DE DRIFT")
	fmt.Println("═══════════════════════════════════════════════════════════")
	if drift == nil {
		fmt.Println("Sem dados de drift.")
		return
	}
	printPinnedApproval(drift.PinnedApproval)
	fmt.Printf("Base: %s → %s\n", drift.DeclaredBase, drift.ActualBase)
	if drift.BaseMismatch {
		fmt.Println("⚠ commit base diverge do declarado no ticket")
	}
	fmt.Printf("Fora do scope (%d):\n", len(drift.OutOfScope))
	for _, f := range drift.OutOfScope {
		fmt.Printf("  - %s\n", f)
	}
	fmt.Printf("Proibidos tocados (%d):\n", len(drift.ForbiddenHit))
	for _, f := range drift.ForbiddenHit {
		fmt.Printf("  - %s\n", f)
	}
	if len(drift.OutOfScope) == 0 && len(drift.ForbiddenHit) == 0 && !drift.BaseMismatch &&
		!pinnedApprovalMismatch(drift.PinnedApproval) {
		fmt.Println("OK: sem drift de escopo/base")
	}
}

func driftBlockingError(drift *review.DriftReport) error {
	if drift == nil {
		return nil
	}
	if pinnedApprovalMismatch(drift.PinnedApproval) {
		return fmt.Errorf("patch do head diverge do SHA aprovado")
	}
	if len(drift.OutOfScope) > 0 || len(drift.ForbiddenHit) > 0 || drift.BaseMismatch {
		return fmt.Errorf("drift detectado")
	}
	return nil
}
