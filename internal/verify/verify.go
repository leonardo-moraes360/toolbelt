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
	MaxIterations int
	Timeout       time.Duration
}

type Result struct {
	Verdict         Verdict            `json:"verdict"`
	Iterations      int                `json:"iterations"`
	Guard           *guard.CheckResult `json:"guard,omitempty"`
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
	chk := guard.NewChecker(cfg.RepoDir)
	gr, err := chk.Check(cfg.BaseRef, cfg.HeadRef)
	if err != nil {
		return nil, err
	}
	res.Guard = gr
	for _, v := range gr.Violations {
		if v.Severity == guard.SeverityError {
			res.Verdict = VerdictFail
			return res, nil
		}
	}

	hiddenDir := filepath.Join(cfg.RepoDir, "tests", "hidden")
	_, dirErr := os.Stat(hiddenDir)
	dirExists := dirErr == nil
	if cfg.HiddenTest == "" && dirExists {
		cfg.HiddenTest = DefaultHiddenTestCmd
	}
	res.HiddenAvailable = cfg.HiddenTest != "" || dirExists

	for i := 1; i <= cfg.MaxIterations; i++ {
		res.Iterations = i
		visibleOK, err := runTest(ctx, cfg.RepoDir, cfg.VisibleTest, cfg.Timeout)
		if err != nil {
			return nil, err
		}
		res.VisiblePassed = visibleOK
		hiddenOK := true
		if res.HiddenAvailable {
			hiddenOK, err = runTest(ctx, cfg.RepoDir, cfg.HiddenTest, cfg.Timeout)
			if err != nil {
				return nil, err
			}
			res.HiddenPassed = hiddenOK
		}
		if visibleOK && hiddenOK {
			res.Verdict = VerdictPass
			return res, nil
		}
		log.Info("iteração de verify falhou", "n", i)
	}
	res.Verdict = VerdictEscalate
	return res, nil
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
