package verify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

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

const errMissingCmdFmt = "comando não encontrado: %s — ajuste %s"

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
	EnvAllow       []string
	EnvPass        []string
}

type Result struct {
	Verdict         Verdict            `json:"verdict"`
	Iterations      int                `json:"iterations"`
	Guard           *guard.CheckResult `json:"guard,omitempty"`
	BuildRan        bool               `json:"build_ran"`
	BuildPassed     bool               `json:"build_passed"`
	LintRan         bool               `json:"lint_ran"`
	LintPassed      bool               `json:"lint_passed"`
	VisiblePassed   bool               `json:"visible_passed"`
	HiddenPassed    *bool              `json:"hidden_passed"`
	HiddenAvailable bool               `json:"hidden_available"`
	HeadSHA         string             `json:"head_sha,omitempty"`
	WorkDir         string             `json:"workdir,omitempty"`
	Metrics         map[string]string  `json:"metrics,omitempty"`
	Error           string             `json:"error,omitempty"`
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

func Run(ctx context.Context, cfg *Config) (*Result, error) {
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 3
	}
	res := &Result{Verdict: VerdictFail, WorkDir: cfg.RepoDir}

	headSHA, err := gitRevParse(cfg.RepoDir, cfg.HeadRef)
	if err != nil {
		return nil, fmt.Errorf("resolver --head %q: %w", cfg.HeadRef, err)
	}
	res.HeadSHA = headSHA

	abort, err := applyGuard(cfg, res)
	if err != nil {
		return nil, err
	}
	if abort {
		return res, nil
	}

	hw, err := attachHeadWorkDir(cfg.RepoDir, headSHA)
	if err != nil {
		return nil, err
	}
	defer hw.Cleanup()
	res.WorkDir = hw.WorkDir
	res.HeadSHA = hw.HeadSHA
	if hw.Temporary {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer stop()
	}
	if hw.DirtyWarn {
		log.Warn("worktree sujo com head igual ao HEAD; build/lint/test podem incluir mudanças não commitadas",
			"workdir", hw.WorkDir, "head_sha", hw.HeadSHA)
	}

	execCfg := *cfg
	execCfg.RepoDir = hw.WorkDir
	res.HiddenAvailable = resolveHiddenAvailable(&execCfg)

	for i := 1; i <= execCfg.MaxIterations; i++ {
		res.Iterations = i
		passed, iterErr := runVerifyIteration(ctx, &execCfg, res)
		if iterErr != nil {
			if IsMissingCommand(iterErr) {
				res.Verdict = VerdictFail
				res.Error = iterErr.Error()
				return res, iterErr
			}
			return nil, iterErr
		}
		if passed {
			res.Verdict = VerdictPass
			return res, nil
		}
	}
	res.Verdict = VerdictEscalate
	return res, nil
}

func applyGuard(cfg *Config, res *Result) (bool, error) {
	chk := guard.NewChecker(cfg.RepoDir)
	gr, err := chk.CheckDiff(cfg.BaseRef, cfg.HeadRef, cfg.NameStatusDiff)
	if err != nil {
		return false, err
	}
	scope := resolveVerifyScope(cfg)
	res.Guard = gr
	abort := false
	for i := range gr.Violations {
		if !guardBlocksVerify(&gr.Violations[i], scope) {
			continue
		}
		abort = true
	}
	if abort {
		res.Verdict = VerdictFail
	}
	return abort, nil
}

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
		v.Message = "path protegido (guard.yaml) no Scope do ticket — verify continua"
	} else {
		v.Message = "path protegido (guard.yaml) — verify reporta e continua (anti-cheat default ainda aborta)"
	}
	return false
}

func resolveHiddenAvailable(cfg *Config) bool {
	hiddenDir := filepath.Join(cfg.RepoDir, "tests", "hidden")
	_, dirErr := os.Stat(hiddenDir)
	dirExists := dirErr == nil
	if cfg.HiddenTest == "" && dirExists {
		cfg.HiddenTest = DefaultHiddenTestCmd
	}
	return cfg.HiddenTest != "" || dirExists
}

func runVerifyIteration(ctx context.Context, cfg *Config, res *Result) (bool, error) {
	preOK, preErr := runBuildAndLint(ctx, cfg, res)
	if preErr != nil {
		return false, preErr
	}
	if !preOK {
		log.Info("iteração de verify falhou", "n", res.Iterations, "step", "build_or_lint")
		return false, nil
	}
	visibleOK, err := runConfiguredCmd(ctx, cfg, cfg.VisibleTest, "verify.test_cmd")
	if err != nil {
		return false, err
	}
	res.VisiblePassed = visibleOK
	hiddenOK := true
	if res.HiddenAvailable {
		hiddenOK, err = runConfiguredCmd(ctx, cfg, cfg.HiddenTest, "verify.hidden_test_cmd")
		if err != nil {
			return false, err
		}
		res.HiddenPassed = boolPtr(hiddenOK)
	} else {
		res.HiddenPassed = nil
	}
	if visibleOK && hiddenOK {
		return true, nil
	}
	log.Info("iteração de verify falhou", "n", res.Iterations)
	return false, nil
}

func runBuildAndLint(ctx context.Context, cfg *Config, res *Result) (bool, error) {
	if cfg.BuildCmd != "" {
		res.BuildRan = true
		ok, err := runConfiguredCmd(ctx, cfg, cfg.BuildCmd, "verify.build_cmd")
		if err != nil {
			return false, err
		}
		res.BuildPassed = ok
		if !ok {
			return false, nil
		}
	}
	if cfg.LintCmd != "" {
		res.LintRan = true
		ok, err := runConfiguredCmd(ctx, cfg, cfg.LintCmd, "verify.lint_cmd")
		if err != nil {
			return false, err
		}
		res.LintPassed = ok
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func runConfiguredCmd(ctx context.Context, cfg *Config, command, configKey string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command) //nolint:gosec // ruler configurado pelo operador
	cmd.Dir = cfg.RepoDir
	cmd.Env = execenv.Environ(execenv.Options{Allow: cfg.EnvAllow, Pass: cfg.EnvPass})
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	if isCommandMissing(err) {
		return false, &MissingCommandError{Command: command, ConfigKey: configKey}
	}
	log.Debug("teste falhou", "output", string(out), "error", err)
	return false, nil
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

func boolPtr(v bool) *bool {
	return &v
}
