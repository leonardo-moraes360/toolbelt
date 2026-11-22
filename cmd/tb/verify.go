package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/verify"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Adversarial verification with pluggable rulers",
}

var verifyRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs the verification loop",
	RunE:  runVerify,
}

func init() {
	verifyRunCmd.Flags().String("base", "", "base ref")
	verifyRunCmd.Flags().String("head", "", "head ref")
	verifyRunCmd.Flags().String("ticket", "", "ticket with Scope (exception to custom protected_paths in guard.yaml)")
	verifyRunCmd.Flags().String("test-cmd", "", "visible tests command line")
	verifyRunCmd.Flags().String("hidden-test-cmd", "", "hidden tests command line (runs even without tests/hidden/; if the dir exists and the cmd is empty: detects the toolchain or marks N/A)")
	verifyRunCmd.Flags().String("build-cmd", "", "build command (empty + explicit flag or --no-build to skip)")
	verifyRunCmd.Flags().String("lint-cmd", "", "lint command (empty + explicit flag or --no-lint to skip)")
	verifyRunCmd.Flags().Bool("no-build", false, "skips build_cmd (config and default)")
	verifyRunCmd.Flags().Bool("no-lint", false, "skips lint_cmd (config and default)")
	verifyRunCmd.Flags().Bool("dry-run", false, "shows the plan (cmds and directory) without executing")
	verifyRunCmd.Flags().Int("max-iter", 0, "maximum iterations")
	verifyRunCmd.Flags().String("timeout", "10m", "timeout")
	verifyRunCmd.Flags().StringP("output", "o", "", "JSON file")
	verifyRunCmd.Flags().String("ruler", "tests", "rulers, comma-separated: tests|visual (visual runs alone; unknown names exit 2)")
	verifyRunCmd.Flags().String("manifest", "", "screens manifest (.toolbelt/screens.yaml)")
	verifyRunCmd.Flags().Float64("threshold", 0.95, "visual similarity threshold")
	verifyRunCmd.Flags().Bool("skip-unavailable", true, "graceful skip (SKIPPED verdict) if browser/refs/screens are unavailable")
	verifyCmd.AddCommand(verifyRunCmd)
}

func init() {
	verifyRunCmd.Flags().String("typecheck-cmd", "", "typecheck command (empty + explicit flag or --no-typecheck to skip)")
	verifyRunCmd.Flags().Bool("no-typecheck", false, "skips typecheck_cmd (config)")
	verifyRunCmd.Flags().StringArray("check", nil, "named check name=cmd (repeatable; adds one or overrides its cmd)")
	verifyRunCmd.Flags().StringArray("no-check", nil, "skips the named check (repeatable)")
	verifyRunCmd.Flags().Bool("no-checks", false, "skips every verify.checks entry")
}

func runVerify(cmd *cobra.Command, args []string) error {
	ruler, _ := cmd.Flags().GetString("ruler")
	names, err := verify.ParseRulers(ruler)
	if err != nil {
		return err
	}
	cfg, err := verifyRunConfig(cmd)
	if err != nil {
		return err
	}
	multi, err := verify.RunRulers(context.Background(), &cfg, names)
	if err != nil {
		return err
	}
	return emitRulersResult(cmd, multi)
}

// verifyRunConfig builds the run config: R0 (verify section from the base
// ref over the trusted config) and the flags are applied inside RunRulers.
func verifyRunConfig(cmd *cobra.Command) (verify.Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return verify.Config{}, err
	}
	cfg := verify.DefaultConfig(cwd)
	trusted, err := config.LoadTrusted()
	if err != nil {
		return cfg, err
	}
	cfg.TrustedConfig = trusted
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
	overrides, err := verifyFlagOverrides(cmd)
	if err != nil {
		return cfg, err
	}
	cfg.Overrides = overrides
	if v, _ := cmd.Flags().GetInt("max-iter"); v > 0 {
		cfg.MaxIterations = v
	}
	cfg.DryRun, _ = cmd.Flags().GetBool("dry-run")
	timeout, _ := time.ParseDuration(mustStringFlag(cmd, "timeout", "10m"))
	cfg.Timeout = timeout
	manifest, _ := cmd.Flags().GetString("manifest")
	threshold, _ := cmd.Flags().GetFloat64("threshold")
	skipOK, _ := cmd.Flags().GetBool("skip-unavailable")
	cfg.Visual = &verify.VisualConfig{Manifest: manifest, Threshold: threshold, SkipOK: skipOK}
	return cfg, nil
}

// verifyFlagOverrides reads the command flags shared by tb verify run and
// tb stack: an explicit empty build/lint/typecheck flag disables that step.
func verifyFlagOverrides(cmd *cobra.Command) (*verify.FlagOverrides, error) {
	flags := cmd.Flags()
	str := func(name string) verify.CmdOverride {
		v, _ := flags.GetString(name)
		return verify.CmdOverride{Cmd: v, Set: flags.Changed(name)}
	}
	o := &verify.FlagOverrides{
		TestCmd:       str("test-cmd"),
		HiddenTestCmd: str("hidden-test-cmd"),
		BuildCmd:      str("build-cmd"),
		LintCmd:       str("lint-cmd"),
		TypecheckCmd:  str("typecheck-cmd"),
	}
	o.NoBuild, _ = flags.GetBool("no-build")
	o.NoLint, _ = flags.GetBool("no-lint")
	o.NoTypecheck, _ = flags.GetBool("no-typecheck")
	o.NoChecks, _ = flags.GetBool("no-checks")
	o.SkipChecks, _ = flags.GetStringArray("no-check")
	specs, _ := flags.GetStringArray("check")
	checks, err := parseCheckFlags(specs)
	if err != nil {
		return nil, err
	}
	o.Checks = checks
	return o, nil
}

// parseCheckFlags splits each --check name=cmd at the first '='.
func parseCheckFlags(specs []string) ([]config.VerifyCheck, error) {
	checks := make([]config.VerifyCheck, 0, len(specs))
	for _, spec := range specs {
		name, cmdLine, ok := strings.Cut(spec, "=")
		name, cmdLine = strings.TrimSpace(name), strings.TrimSpace(cmdLine)
		if !ok || name == "" || cmdLine == "" {
			return nil, &verify.UsageError{Msg: fmt.Sprintf("invalid --check %q: want name=cmd", spec)}
		}
		checks = append(checks, config.VerifyCheck{Name: name, Cmd: cmdLine})
	}
	return checks, nil
}

// emitRulersResult prints the payload (single ruler: its own shape) and
// exits with the composed code. Text mode keeps each ruler's old summary;
// a missing command prints only the error, as before.
func emitRulersResult(cmd *cobra.Command, multi *verify.MultiResult) error {
	outPath, _ := cmd.Flags().GetString("output")
	if err := writeVerifyPayload(outPath, multi.Payload()); err != nil {
		return err
	}
	if !jsonOutput {
		printVerifyNotices(multi.Warnings, multi.Overrides)
		for _, rr := range multi.Results {
			printRulerText(rr)
		}
	}
	if code := multi.ExitCode(); code != 0 {
		os.Exit(code)
	}
	return nil
}

// printVerifyNotices prints R0 warnings and the applied overrides to stderr,
// for tb verify run and tb stack in text mode.
func printVerifyNotices(warnings, overrides []string) {
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}
	if len(overrides) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: overrides applied: %s\n", strings.Join(overrides, ", "))
	}
}

func printRulerText(rr *verify.RulerResult) {
	switch p := rr.Payload.(type) {
	case *verify.VisualReport:
		printVisualReport(p)
	case *verify.Result:
		if rr.ExitCode == 127 {
			fmt.Fprintf(os.Stderr, "Error: %s\n", p.Error)
			return
		}
		printTestsSummary(p)
	}
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
	fmt.Printf("Verdict: %s (iterations: %d)\n", res.Verdict, res.Iterations)
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
	fmt.Printf("Visible: %v  Hidden: %s (available: %v)\n", res.VisiblePassed, hidden, res.HiddenAvailable)
	if res.Reason != "" && res.Verdict != verify.VerdictPass {
		fmt.Printf("Reason: %s\n", res.Reason)
	}
}

func printVisualReport(rep *verify.VisualReport) {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println(" VISUAL VERIFICATION")
	fmt.Println("═══════════════════════════════════════════════════════════")
	if rep.Skipped {
		fmt.Printf("Graceful skip: %s\n", rep.SkipReason)
		if len(rep.Screens) == 0 {
			fmt.Printf("Verdict: %s\n", rep.Verdict)
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
	fmt.Printf("Verdict: %s\n", rep.Verdict)
}
