package verify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/execenv"
	"github.com/leonardo-moraes360/toolbelt/internal/guard"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
	"github.com/leonardo-moraes360/toolbelt/internal/review"
)

type Verdict string

const (
	VerdictPass     Verdict = "PASS"
	VerdictFail     Verdict = "FAIL"
	VerdictEscalate Verdict = "ESCALATE"
	VerdictSkipped  Verdict = "SKIPPED"
)

const DefaultHiddenTestCmd = "go test ./tests/hidden/..."

const errMissingCmdFmt = "command not found: %s — adjust %s"

type Config struct {
	RepoDir        string
	BaseRef        string
	HeadRef        string
	VisibleTest    string
	HiddenTest     string
	BuildCmd       string
	LintCmd        string
	MaxIterations  int
	Timeout        time.Duration
	Scope          []string
	TicketPath     string
	NameStatusDiff string
	DryRun         bool
	EnvAllow       []string
	EnvPass        []string
	// TypecheckCmd and Checks are gate steps after build (R2, R3).
	TypecheckCmd string
	Checks       []config.VerifyCheck
	// TrustedConfig enables R0 in Run/RunRulers: verify's commands and env
	// allow/pass come from this config (-c alone, or the global config under
	// the repository file committed at the base ref; never the working tree),
	// then Overrides apply. Nil keeps the commands already in Config.
	TrustedConfig *config.Config
	Overrides     *FlagOverrides
	// Visual configures the visual ruler; nil uses its defaults.
	Visual *VisualConfig
}

type Result struct {
	Verdict     Verdict            `json:"verdict"`
	Iterations  int                `json:"iterations"`
	Guard       *guard.CheckResult `json:"guard,omitempty"`
	BuildRan    bool               `json:"build_ran"`
	BuildPassed bool               `json:"build_passed"`
	LintRan     bool               `json:"lint_ran"`
	LintPassed  bool               `json:"lint_passed"`
	// Typecheck keys are omitempty, unlike lint_ran, so a run without a
	// typecheck_cmd keeps the pre-ruler JSON keys.
	TypecheckRan    bool              `json:"typecheck_ran,omitempty"`
	TypecheckPassed *bool             `json:"typecheck_passed,omitempty"`
	Checks          []CheckResult     `json:"checks,omitempty"`
	VisiblePassed   bool              `json:"visible_passed"`
	HiddenPassed    *bool             `json:"hidden_passed"`
	HiddenAvailable bool              `json:"hidden_available"`
	Reason          string            `json:"reason,omitempty"`
	Error           string            `json:"error,omitempty"`
	HeadSHA         string            `json:"head_sha,omitempty"`
	WorkDir         string            `json:"workdir,omitempty"`
	Metrics         map[string]string `json:"metrics,omitempty"`
	DryRun          bool              `json:"dry_run,omitempty"`
	Plan            *Plan             `json:"plan,omitempty"`
	// FilteredEnv lists the variable names (never values) the env allowlist
	// removed from the child commands; set only when the verdict is not PASS.
	FilteredEnv []string `json:"filtered_env,omitempty"`
	// ConfigSource is "none" when R0 found no repository config at the base
	// ref; omitted in the normal case.
	ConfigSource string   `json:"config_source,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
	SkipKind     SkipKind `json:"skip_kind,omitempty"`
}

type Plan struct {
	Dir      string            `json:"dir"`
	Commands map[string]string `json:"commands"`
}

type cmdCache struct {
	gates  []*gate
	checks []*gate
	// lastFailSig is the failure signature of the previous iteration's tests;
	// an identical signature means a deterministic failure, so stop retrying.
	lastFailSig string
}

type MissingCommandError struct {
	Command   string
	ConfigKey string
}

func (e *MissingCommandError) Error() string {
	return fmt.Sprintf(errMissingCmdFmt, e.Command, e.ConfigKey)
}

func (e *MissingCommandError) ExitCode() int {
	return 127
}

func IsMissingCommand(err error) bool {
	var e *MissingCommandError
	return errors.As(err, &e)
}

func DefaultConfig(repo string) Config {
	return Config{
		RepoDir:       repo,
		BaseRef:       "HEAD~1",
		HeadRef:       "HEAD",
		VisibleTest:   "go test ./...",
		HiddenTest:    "",
		MaxIterations: 3,
		Timeout:       10 * time.Minute,
	}
}

// Run runs the tests ruler alone; its payload and errors are those of
// RunRulers(ctx, cfg, []string{"tests"}).
func Run(ctx context.Context, cfg *Config) (*Result, error) {
	ctx, st, cleanup, err := prepareRun(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return runTests(ctx, st)
}

func runTests(ctx context.Context, st *RunState) (*Result, error) {
	cfg := st.Config
	res := &Result{
		Verdict: VerdictFail, WorkDir: st.WorkDir, HeadSHA: st.HeadSHA, Guard: st.Guard,
		ConfigSource: st.ConfigSource, Warnings: st.Warnings,
	}
	if st.GuardAborted {
		res.Reason = st.GuardReason
		res.Error = res.Reason
		return res, nil
	}
	execCfg := *cfg
	execCfg.RepoDir = st.WorkDir
	res.HiddenAvailable = resolveHiddenAvailable(&execCfg)

	if cfg.DryRun {
		return dryRunResult(res, &execCfg), nil
	}
	out, runErr := runVerifyLoops(ctx, &execCfg, res)
	if out != nil && out.Verdict != VerdictPass {
		out.FilteredEnv = execenv.Dropped(os.Environ(), execenv.Options{Allow: cfg.EnvAllow, Pass: cfg.EnvPass})
	}
	return out, runErr
}

func dryRunResult(res *Result, execCfg *Config) *Result {
	res.DryRun = true
	res.Plan = BuildPlan(execCfg)
	res.Verdict = VerdictSkipped
	res.Reason = "dry-run: plan generated without executing"
	return res
}

func runVerifyLoops(ctx context.Context, cfg *Config, res *Result) (*Result, error) {
	cache := &cmdCache{gates: sequentialGates(cfg), checks: checkGates(cfg)}
	for i := 1; i <= cfg.MaxIterations; i++ {
		res.Iterations = i
		passed, stopIter, iterErr := runVerifyIteration(ctx, cfg, res, cache)
		if iterErr != nil {
			if IsMissingCommand(iterErr) {
				res.Verdict = VerdictFail
				if res.Reason == "" {
					res.Reason = iterErr.Error()
				}
				res.Error = iterErr.Error()
				return res, iterErr
			}
			if res.Reason == "" {
				res.Reason = iterErr.Error()
				res.Error = res.Reason
			}
			return nil, iterErr
		}
		if passed {
			res.Verdict = VerdictPass
			res.Reason = ""
			res.Error = ""
			return res, nil
		}
		if stopIter {
			res.Verdict = VerdictEscalate
			return res, nil
		}
	}
	res.Verdict = VerdictEscalate
	if res.Reason == "" {
		res.Reason = fmt.Sprintf("ESCALATE after %d iterations without PASS", res.Iterations)
		res.Error = res.Reason
	}
	return res, nil
}

func BuildPlan(cfg *Config) *Plan {
	_ = resolveHiddenAvailable(cfg)
	cmds := map[string]string{}
	if cfg.BuildCmd != "" {
		cmds["build"] = cfg.BuildCmd
	}
	if cfg.TypecheckCmd != "" {
		cmds["typecheck"] = cfg.TypecheckCmd
	}
	if cfg.LintCmd != "" {
		cmds["lint"] = cfg.LintCmd
	}
	for _, c := range cfg.Checks {
		cmds["check:"+c.Name] = c.Cmd
	}
	if cfg.VisibleTest != "" {
		cmds["test"] = cfg.VisibleTest
	}
	if cfg.HiddenTest != "" {
		cmds["hidden"] = cfg.HiddenTest
	}
	return &Plan{Dir: cfg.RepoDir, Commands: cmds}
}

// applyGuard runs guard and reports whether a blocking finding aborts
// verify, with the reason to report.
func applyGuard(cfg *Config) (gr *guard.CheckResult, abort bool, reason string, err error) {
	chk := guard.NewChecker(cfg.RepoDir)
	gr, err = chk.CheckDiff(cfg.BaseRef, cfg.HeadRef, cfg.NameStatusDiff)
	if err != nil {
		return nil, false, "", err
	}
	scope := resolveVerifyScope(cfg)
	reason = "guard aborted verify (protected path)"
	for i := range gr.Violations {
		if !guardBlocksVerify(&gr.Violations[i], scope) {
			continue
		}
		abort = true
		if gr.Violations[i].Type == violationVerifyConfigChanged {
			reason = "guard aborted verify (verify_config_changed: " + gr.Violations[i].Message + ")"
		}
	}
	if !abort {
		return gr, false, "", nil
	}
	return gr, true, reason, nil
}

// violationVerifyConfigChanged is guard's R0 rule; named here by value so
// verify does not depend on guard exporting it.
const violationVerifyConfigChanged guard.ViolationType = "verify_config_changed"

func resolveVerifyScope(cfg *Config) []string {
	if len(cfg.Scope) > 0 {
		return cfg.Scope
	}
	if cfg.TicketPath == "" {
		return nil
	}
	ticket, err := review.LoadTicket(cfg.TicketPath)
	if err != nil || ticket == nil {
		return nil
	}
	return ticket.Scope
}

func guardBlocksVerify(v *guard.Violation, scope []string) bool {
	if v.Severity != guard.SeverityError {
		return false
	}
	if v.Type != guard.ViolationProtectedPath {
		return true
	}
	if v.Source != guard.SourceCustom {
		return true
	}
	if len(scope) > 0 && !review.PathInScope(v.File, scope) {
		return true
	}
	v.Severity = guard.SeverityWarning
	if len(scope) > 0 {
		v.Message = "protected path (guard.yaml) in ticket Scope — verify continues"
	} else {
		v.Message = "protected path (guard.yaml) — verify reports and continues (default anti-cheat still aborts)"
	}
	return false
}

func resolveHiddenAvailable(cfg *Config) bool {
	hiddenDir := filepath.Join(cfg.RepoDir, "tests", "hidden")
	_, dirErr := os.Stat(hiddenDir)
	dirExists := dirErr == nil
	if cfg.HiddenTest != "" {
		return true
	}
	if !dirExists {
		return false
	}
	cmd := detectHiddenTestCmd(cfg.RepoDir, hiddenDir)
	if cmd == "" {
		return false
	}
	cfg.HiddenTest = cmd
	return true
}

func detectHiddenTestCmd(repoDir, hiddenDir string) string {
	entries, err := os.ReadDir(hiddenDir)
	if err != nil {
		return ""
	}
	hasGo, hasPy, hasJS := false, false, false
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".go":
			hasGo = true
		case ".py":
			hasPy = true
		case ".js", ".mjs", ".cjs", ".ts", ".tsx":
			hasJS = true
		}
	}
	if hasGo {
		if fileExists(filepath.Join(repoDir, "go.mod")) {
			return DefaultHiddenTestCmd
		}
		return ""
	}
	if hasPy {
		return "pytest tests/hidden"
	}
	if hasJS {
		return "node --test tests/hidden"
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runVerifyIteration(ctx context.Context, cfg *Config, res *Result, cache *cmdCache) (passed, stop bool, err error) {
	preOK, preErr := runGates(ctx, cfg, res, cache)
	if preErr != nil {
		return false, true, preErr
	}
	if !preOK {
		log.Info("verify iteration failed", "n", res.Iterations, "step", "gate")
		return false, true, nil
	}
	visibleOK, out, err := runConfiguredCmd(ctx, cfg, cfg.VisibleTest, "verify.test_cmd")
	if err != nil {
		setFailure(res, "verify.test_cmd", err.Error())
		return false, true, err
	}
	res.VisiblePassed = visibleOK
	failSig := ""
	if !visibleOK {
		setFailure(res, "verify.test_cmd", failReason("verify.test_cmd", out))
		if sig := failureSignature(out); sig != "" {
			failSig = "test\x00" + sig
		}
	}
	hiddenOK, hiddenSig, err := runHiddenStep(ctx, cfg, res)
	if err != nil {
		return false, true, err
	}
	failSig += hiddenSig
	if visibleOK && hiddenOK {
		return true, false, nil
	}
	log.Info("verify iteration failed", "n", res.Iterations)
	if failSig != "" && failSig == cache.lastFailSig {
		res.Reason = "deterministic failure (identical across iterations " + fmt.Sprint(res.Iterations-1) + " and " + fmt.Sprint(res.Iterations) + "): " + res.Reason
		res.Error = res.Reason
		return false, true, nil
	}
	cache.lastFailSig = failSig
	return false, false, nil
}

func runHiddenStep(ctx context.Context, cfg *Config, res *Result) (ok bool, sig string, err error) {
	if !res.HiddenAvailable {
		res.HiddenPassed = nil
		return true, "", nil
	}
	ok, out, err := runConfiguredCmd(ctx, cfg, cfg.HiddenTest, "verify.hidden_test_cmd")
	if err != nil {
		setFailure(res, "verify.hidden_test_cmd", err.Error())
		return false, "", err
	}
	res.HiddenPassed = boolPtr(ok)
	if ok {
		return true, "", nil
	}
	setFailure(res, "verify.hidden_test_cmd", failReason("verify.hidden_test_cmd", out))
	if s := failureSignature(out); s != "" {
		sig = "\x00hidden\x00" + s
	}
	return false, sig, nil
}

func setFailure(res *Result, step, reason string) {
	if reason == "" {
		reason = step + " failed"
	}
	res.Reason = reason
	res.Error = reason
}

const failReasonMax = 1024

func failReason(step, output string) string {
	out := strings.TrimSpace(stripANSI(output))
	if out == "" {
		return step + " failed"
	}
	return step + " failed: " + excerpt(out, failReasonMax)
}

func runConfiguredCmd(ctx context.Context, cfg *Config, command, configKey string) (ok bool, output string, err error) {
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command) //nolint:gosec // operator-configured ruler
	cmd.Dir = cfg.RepoDir
	cmd.Env = withNoColor(execenv.Environ(execenv.Options{Allow: cfg.EnvAllow, Pass: cfg.EnvPass}))
	out, runErr := cmd.CombinedOutput()
	output = string(out)
	if runErr == nil {
		return true, output, nil
	}
	if isCommandMissing(runErr) {
		return false, output, &MissingCommandError{Command: command, ConfigKey: configKey}
	}
	log.Debug("test failed", "output", output, "error", runErr)
	return false, output, nil
}

func isCommandMissing(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		return code == 126 || code == 127
	}
	return false
}

func (r *Result) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func LoadHiddenSuite(repo string) bool {
	_, err := os.Stat(filepath.Join(repo, "tests", "hidden"))
	return err == nil
}

func ApplyCmdOverrides(cfg *Config, buildCmd string, buildSet, noBuild bool, lintCmd string, lintSet, noLint bool) {
	if noBuild || (buildSet && buildCmd == "") {
		cfg.BuildCmd = ""
	} else if buildSet {
		cfg.BuildCmd = buildCmd
	}
	if noLint || (lintSet && lintCmd == "") {
		cfg.LintCmd = ""
	} else if lintSet {
		cfg.LintCmd = lintCmd
	}
}

func boolPtr(v bool) *bool {
	return &v
}
