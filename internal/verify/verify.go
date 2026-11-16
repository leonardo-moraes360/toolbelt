package verify

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/guard"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
)

type Verdict string

const (
	VerdictPass     Verdict = "PASS"
	VerdictFail     Verdict = "FAIL"
	VerdictEscalate Verdict = "ESCALATE"
)

const DefaultHiddenTestCmd = "go test ./tests/hidden/..."

type Config struct {
	RepoDir       string
	BaseRef       string
	HeadRef       string
	VisibleTest   string
	HiddenTest    string
	BuildCmd      string
	LintCmd       string
	MaxIterations int
	Timeout       time.Duration
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
	HiddenPassed    bool               `json:"hidden_passed"`
	HiddenAvailable bool               `json:"hidden_available"`
	Metrics         map[string]string  `json:"metrics,omitempty"`
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
	res := &Result{Verdict: VerdictFail}
	abort, err := applyGuard(cfg, res)
	if err != nil {
		return nil, err
	}
	if abort {
		return res, nil
	}

	res.HiddenAvailable = resolveHiddenAvailable(cfg)

	for i := 1; i <= cfg.MaxIterations; i++ {
		res.Iterations = i
		passed, iterErr := runVerifyIteration(ctx, cfg, res)
		if iterErr != nil {
			return nil, iterErr
		}
		if passed {
			res.Verdict = VerdictPass
			return res, nil
		}
		log.Info("iteração de verify falhou", "n", i)
	}
	res.Verdict = VerdictEscalate
	return res, nil
}

func applyGuard(cfg *Config, res *Result) (bool, error) {
	chk := guard.NewChecker(cfg.RepoDir)
	gr, err := chk.Check(cfg.BaseRef, cfg.HeadRef)
	if err != nil {
		return false, err
	}
	res.Guard = gr
	for _, v := range gr.Violations {
		if v.Severity == guard.SeverityError {
			res.Verdict = VerdictFail
			return true, nil
		}
	}
	return false, nil
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
	visibleOK, err := runTest(ctx, cfg.RepoDir, cfg.VisibleTest, cfg.Timeout)
	if err != nil {
		return false, err
	}
	res.VisiblePassed = visibleOK
	hiddenOK := true
	if res.HiddenAvailable {
		hiddenOK, err = runTest(ctx, cfg.RepoDir, cfg.HiddenTest, cfg.Timeout)
		if err != nil {
			return false, err
		}
		res.HiddenPassed = hiddenOK
	}
	return visibleOK && hiddenOK, nil
}

func runBuildAndLint(ctx context.Context, cfg *Config, res *Result) (bool, error) {
	if cfg.BuildCmd != "" {
		res.BuildRan = true
		ok, err := runTest(ctx, cfg.RepoDir, cfg.BuildCmd, cfg.Timeout)
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
		ok, err := runTest(ctx, cfg.RepoDir, cfg.LintCmd, cfg.Timeout)
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

func runTest(ctx context.Context, dir, command string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command) //nolint:gosec // ruler configurado pelo operador
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Debug("teste falhou", "output", string(out), "error", err)
		return false, nil
	}
	return true, nil
}

func (r *Result) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func LoadHiddenSuite(repo string) bool {
	_, err := os.Stat(filepath.Join(repo, "tests", "hidden"))
	return err == nil
}
