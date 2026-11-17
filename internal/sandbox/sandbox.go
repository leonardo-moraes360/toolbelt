package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

var ErrUnsupported = errors.New("Landlock indisponível nesta plataforma")

type Config struct {
	WorkDir          string
	AllowedPaths     []PathAccess
	ForbiddenPaths   []string
	Timeout          time.Duration
	AllowUnsandboxed bool
}

type PathAccess struct {
	Path     string
	ReadOnly bool
}

type Sandbox struct {
	config     Config
	landlockOK bool
}

type Result struct {
	Output    string        `json:"output"`
	ExitCode  int           `json:"exit_code"`
	Duration  time.Duration `json:"duration_ns"`
	TimedOut  bool          `json:"timed_out"`
	Sandboxed bool          `json:"sandboxed"`
}

func New(config Config) (*Sandbox, error) { //nolint:gocritic // API pública estável por valor

	if config.WorkDir == "" {
		return nil, errors.New("workdir obrigatório")
	}
	abs, err := filepath.Abs(config.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("workdir: %w", err)
	}
	config.WorkDir = abs

	s := &Sandbox{config: config}
	if runtime.GOOS == "linux" {
		s.landlockOK = landlockAvailable()
	}
	if !s.landlockOK && !config.AllowUnsandboxed {
		if runtime.GOOS != "linux" {
			return nil, fmt.Errorf("%w; use --allow-unsandboxed para optar por execução sem isolamento", ErrUnsupported)
		}
		_, _, probeErr := CheckLandlockSupport()
		if probeErr == nil {
			probeErr = ErrUnsupported
		}
		return nil, fmt.Errorf("Landlock indisponível (%v); use --allow-unsandboxed para optar por execução sem isolamento", probeErr)
	}
	return s, nil
}

func (s *Sandbox) Run(ctx context.Context, command string, args ...string) (*Result, error) {
	if s.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()
	}

	bin, err := exec.LookPath(command)
	if err != nil {
		return nil, fmt.Errorf("comando não encontrado: %w", err)
	}

	if s.landlockOK {
		return s.runIsolated(ctx, bin, args...)
	}
	return s.runPlain(ctx, bin, args...)
}

func (s *Sandbox) runPlain(ctx context.Context, bin string, args ...string) (*Result, error) {
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // sandbox invocado explicitamente pelo operador
	cmd.Dir = s.config.WorkDir
	return finishRun(ctx, cmd, false)
}

func (s *Sandbox) runIsolated(ctx context.Context, bin string, args ...string) (*Result, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("executável atual: %w", err)
	}
	env, err := prepareChildEnv(&s.config, bin, args)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, self) //nolint:gosec // re-exec do próprio binário como trampolim Landlock
	cmd.Dir = s.config.WorkDir
	cmd.Env = env
	return finishRun(ctx, cmd, true)
}

func finishRun(ctx context.Context, cmd *exec.Cmd, sandboxed bool) (*Result, error) {
	start := time.Now()
	out, err := cmd.CombinedOutput()
	res := &Result{Output: string(out), Duration: time.Since(start), Sandboxed: sandboxed}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			res.TimedOut = true
			res.ExitCode = -1
			return res, nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
			return res, nil
		}
		return nil, err
	}
	return res, nil
}

func (s *Sandbox) ValidatePath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	work, err := filepath.Abs(s.config.WorkDir)
	if err != nil {
		return err
	}
	for _, forbidden := range s.config.ForbiddenPaths {
		base := forbidden
		if !filepath.IsAbs(forbidden) {
			base = filepath.Join(work, forbidden)
		}
		if abs == base || isSubpath(abs, base) {
			return fmt.Errorf("caminho proibido: %s", forbidden)
		}
	}
	if isSubpath(abs, work) || abs == work {
		return nil
	}
	return fmt.Errorf("fora do escopo: %s", path)
}

func isSubpath(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel != ".." && !filepath.IsAbs(rel) && rel != "."
}

func landlockAvailable() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	ok, _, _ := CheckLandlockSupport()
	return ok
}
