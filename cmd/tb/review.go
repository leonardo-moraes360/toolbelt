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
	Short: "Revisão fresh-context (Elephant-Goldfish)",
	Long: `Revisa um diff contra ticket/rd.md e invariantes de tb mem.

Goldfish distintos (contexto fresco cada um):
  scope       — escopo declarado e paths forbidden (+ invariantes de tb mem)
  quality     — TODO/FIXME/XXX/HACK em comentários, funções grandes, código sem testes
  acceptance  — critérios de aceite evidenciados em testes/arquivos (inclui Test*/test/it)

Também reporta drift de escopo e commit base.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReview,
}

func init() {
	reviewCmd.Flags().String("base", "HEAD~1", "commit/ref base")
	reviewCmd.Flags().String("head", "HEAD", "commit/ref head")
	reviewCmd.Flags().String("approved-sha", "", "SHA aprovado; compara patch-id verbatim (sensível a whitespace) com o head")
	reviewCmd.Flags().Bool("require-approved-sha", false, "falha se --approved-sha estiver ausente (ou review.require_approved_sha)")
	reviewCmd.Flags().String("ticket", "rd.md", "caminho do ticket/rd.md")
	reviewCmd.Flags().Int("reviewers", 3, "máximo de goldfish distintos (não clona o mesmo)")
	reviewCmd.Flags().String("standards", "scope,quality,acceptance", "papéis: scope,quality,acceptance (aliases: invariants,completeness)")
	reviewCmd.Flags().String("output", "", "arquivo de saída JSON")
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
		return fmt.Errorf("isolamento de contexto fresco falhou; revisão abortada")
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
		return fmt.Errorf("patch do head diverge do SHA aprovado")
	}
	if rep.Drift != nil && (len(rep.Drift.OutOfScope) > 0 || len(rep.Drift.ForbiddenHit) > 0 || rep.Drift.BaseMismatch) {
		return fmt.Errorf("drift detectado no escopo/base")
	}
	for i := range rep.Findings {
		f := &rep.Findings[i]
		if f.Severity == review.SeverityCritical || f.Severity == review.SeverityError {
			return fmt.Errorf("revisão encontrou findings bloqueantes")
		}
	}
	return nil
}

func pinnedApprovalMismatch(p *review.PinnedApproval) bool {
	return p.Diverged()
}

func printReviewReport(rep *review.Report) {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf(" RELATÓRIO DE REVISÃO: %s\n", rep.Target)
	fmt.Printf(" Base..Head: %s..%s | Revisores: %d | Fresh: %v\n", rep.BaseRef, rep.HeadRef, rep.Reviewers, rep.FreshOK)
	fmt.Println("═══════════════════════════════════════════════════════════")
	printPinnedApproval(rep.PinnedApproval)
	printDriftSection(rep.Drift)
	if len(rep.Findings) == 0 {
		fmt.Println("Nenhum finding.")
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
			fmt.Printf("  papel=%s papéis_acordo=%d\n", role, f.Agreement)
		} else {
			fmt.Printf("  papel=%s\n", role)
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
		fmt.Printf("  base divergente: declarado=%s atual=%s\n", d.DeclaredBase, d.ActualBase)
	}
	for _, f := range d.OutOfScope {
		fmt.Printf("  fora do scope: %s\n", f)
	}
	for _, f := range d.ForbiddenHit {
		fmt.Printf("  proibido: %s\n", f)
	}
	if !d.BaseMismatch && len(d.OutOfScope) == 0 && len(d.ForbiddenHit) == 0 {
		fmt.Println("  nenhum")
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
