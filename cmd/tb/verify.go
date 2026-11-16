package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/verify"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verificação adversarial com rulers plugáveis",
}

var verifyRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Executa loop de verificação",
	RunE:  runVerify,
}

func init() {
	verifyRunCmd.Flags().String("base", "", "ref base")
	verifyRunCmd.Flags().String("head", "", "ref head")
	verifyRunCmd.Flags().String("ticket", "", "ticket com Scope (exceção de protected_paths custom do guard.yaml)")
	verifyRunCmd.Flags().String("test-cmd", "", "linha de testes visíveis")
	verifyRunCmd.Flags().String("hidden-test-cmd", "", "linha de testes ocultos (roda mesmo sem tests/hidden/; se o dir existir e o cmd estiver vazio: detecta toolchain ou marca N/A)")
	verifyRunCmd.Flags().String("build-cmd", "", "comando de build (vazio + flag explícita ou --no-build para pular)")
	verifyRunCmd.Flags().String("lint-cmd", "", "comando de lint (vazio + flag explícita ou --no-lint para pular)")
	verifyRunCmd.Flags().Bool("no-build", false, "pula build_cmd (config e default)")
	verifyRunCmd.Flags().Bool("no-lint", false, "pula lint_cmd (config e default)")
	verifyRunCmd.Flags().Bool("dry-run", false, "mostra o plano (cmds e diretório) sem executar")
	verifyRunCmd.Flags().Int("max-iter", 0, "máximo de iterações")
	verifyRunCmd.Flags().String("timeout", "10m", "timeout")
	verifyRunCmd.Flags().StringP("output", "o", "", "arquivo JSON")
	verifyRunCmd.Flags().String("ruler", "tests", "ruler: tests|visual")
	verifyRunCmd.Flags().String("manifest", "", "manifesto de telas (.toolbelt/screens.yaml)")
	verifyRunCmd.Flags().Float64("threshold", 0.95, "threshold de similaridade visual")
	verifyRunCmd.Flags().Bool("skip-unavailable", true, "skip gracioso (veredicto SKIPPED) se browser/refs/telas indisponíveis")
	verifyCmd.AddCommand(verifyRunCmd)
}

func runVerify(cmd *cobra.Command, args []string) error {
	ruler, _ := cmd.Flags().GetString("ruler")
	switch ruler {
	case "visual":
		return runVisualVerify(cmd)
	default:
		return runTestsVerify(cmd)
	}
}

func runTestsVerify(cmd *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg := verify.DefaultConfig(cwd)
	if fileCfg, loadErr := config.Load(); loadErr == nil {
		if fileCfg.Verify.TestCmd != "" {
			cfg.VisibleTest = fileCfg.Verify.TestCmd
		}
		if fileCfg.Verify.HiddenTestCmd != "" {
			cfg.HiddenTest = fileCfg.Verify.HiddenTestCmd
		}
		if fileCfg.Verify.BuildCmd != "" {
			cfg.BuildCmd = fileCfg.Verify.BuildCmd
		}
		if fileCfg.Verify.LintCmd != "" {
			cfg.LintCmd = fileCfg.Verify.LintCmd
		}
		cfg.EnvAllow = append(cfg.EnvAllow, fileCfg.Env.Allow...)
		cfg.EnvPass = append(cfg.EnvPass, fileCfg.Env.Pass...)
	}
	cfg.EnvAllow = append(cfg.EnvAllow, envAllow...)
	cfg.EnvPass = append(cfg.EnvPass, envPass...)
	if v, _ := cmd.Flags().GetString("base"); v != "" {
		cfg.BaseRef = v
	}
	if v, _ := cmd.Flags().GetString("head"); v != "" {
		cfg.HeadRef = v
	}
	if v, _ := cmd.Flags().GetString("ticket"); v != "" {
		cfg.TicketPath = v
	}
	if v, _ := cmd.Flags().GetString("test-cmd"); v != "" {
		cfg.VisibleTest = v
	}
	if cmd.Flags().Changed("hidden-test-cmd") {
		v, _ := cmd.Flags().GetString("hidden-test-cmd")
		cfg.HiddenTest = v
	}
	buildCmd, _ := cmd.Flags().GetString("build-cmd")
	lintCmd, _ := cmd.Flags().GetString("lint-cmd")
	noBuild, _ := cmd.Flags().GetBool("no-build")
	noLint, _ := cmd.Flags().GetBool("no-lint")
	verify.ApplyCmdOverrides(&cfg, buildCmd, cmd.Flags().Changed("build-cmd"), noBuild, lintCmd, cmd.Flags().Changed("lint-cmd"), noLint)
	if v, _ := cmd.Flags().GetInt("max-iter"); v > 0 {
		cfg.MaxIterations = v
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	cfg.DryRun = dryRun
	timeout, _ := time.ParseDuration(mustStringFlag(cmd, "timeout", "10m"))
	cfg.Timeout = timeout
	res, err := verify.Run(context.Background(), &cfg)
	if err != nil {
		return err
	}
	return emitVerifyResult(cmd, res.Verdict, res)
}

func runVisualVerify(cmd *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	manifest, _ := cmd.Flags().GetString("manifest")
	threshold, _ := cmd.Flags().GetFloat64("threshold")
	skipOK, _ := cmd.Flags().GetBool("skip-unavailable")
	cfg := verify.VisualConfig{
		Manifest:  manifest,
		Threshold: threshold,
		SkipOK:    skipOK,
	}
	rep, err := verify.RunVisual(context.Background(), cwd, cfg)
	if err != nil {
		return err
	}
	if !jsonOutput {
		printVisualReport(rep)
	}
	return emitVerifyResult(cmd, rep.Verdict, rep)
}

func emitVerifyResult(cmd *cobra.Command, verdict verify.Verdict, payload any) error {
	outPath, _ := cmd.Flags().GetString("output")
	if err := writeVerifyPayload(outPath, payload); err != nil {
		return err
	}
	printTestsSummary(payload)
	if verifyShouldFail(verdict) {
		os.Exit(1)
	}
	return nil
}

func verifyShouldFail(verdict verify.Verdict) bool {
	return verdict != verify.VerdictPass && verdict != verify.VerdictSkipped
}

func writeVerifyPayload(outPath string, payload any) error {
	if outPath == "" && !jsonOutput {
		return nil
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if outPath != "" {
		return os.WriteFile(outPath, data, 0o600)
	}
	fmt.Println(string(data))
	return nil
}

func printTestsSummary(payload any) {
	if jsonOutput {
		return
	}
	res, ok := payload.(*verify.Result)
	if !ok {
		return
	}
	if res.DryRun && res.Plan != nil {
		fmt.Printf("Dry-run: dir=%s\n", res.Plan.Dir)
		for step, cmd := range res.Plan.Commands {
			fmt.Printf("  %s: %s\n", step, cmd)
		}
		return
	}
	fmt.Printf("Veredicto: %s (iterações: %d)\n", res.Verdict, res.Iterations)
	if res.HeadSHA != "" {
		fmt.Printf("Head: %s\n", res.HeadSHA)
	}
	if res.WorkDir != "" {
		fmt.Printf("Workdir: %s\n", res.WorkDir)
	}
	if res.BuildRan {
		fmt.Printf("Build: %v\n", res.BuildPassed)
	}
	if res.LintRan {
		fmt.Printf("Lint: %v\n", res.LintPassed)
	}
	hidden := "n/a"
	if res.HiddenPassed != nil {
		hidden = fmt.Sprintf("%v", *res.HiddenPassed)
	}
	fmt.Printf("Visível: %v  Oculto: %s (disponível: %v)\n", res.VisiblePassed, hidden, res.HiddenAvailable)
	if res.Reason != "" && res.Verdict != verify.VerdictPass {
		fmt.Printf("Motivo: %s\n", res.Reason)
	}
}

func printVisualReport(rep *verify.VisualReport) {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println(" VERIFICAÇÃO VISUAL")
	fmt.Println("═══════════════════════════════════════════════════════════")
	if rep.Skipped {
		fmt.Printf("Skip gracioso: %s\n", rep.SkipReason)
		if len(rep.Screens) == 0 {
			fmt.Printf("Veredicto: %s\n", rep.Verdict)
			return
		}
	}
	if rep.Browser != "" {
		fmt.Printf("Browser: %s\n", rep.Browser)
	}
	fmt.Printf("Threshold: %.0f%%\n", rep.Threshold*100)
	for _, s := range rep.Screens {
		status := "FAIL"
		if s.Passed {
			status = "PASS"
		}
		if s.Skipped {
			status = "SKIP"
		}
		fmt.Printf("  [%s] %s/%s similarity=%.1f%% size_match=%v", status, s.Screen, s.Mode, s.Similarity*100, s.SizeMatch)
		if s.SkipReason != "" {
			fmt.Printf(" (%s)", s.SkipReason)
		}
		fmt.Println()
	}
	fmt.Printf("Veredicto: %s\n", rep.Verdict)
}
