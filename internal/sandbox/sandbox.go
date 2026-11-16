package sandbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

var ErrUnsupported = errors.New("Landlock indisponível nesta plataforma")

type Config struct {
	WorkDir        string
	AllowedPaths   []PathAccess
	ForbiddenPaths []string
	Timeout        time.Duration
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

func New(config Config) (*Sandbox, error) {
	if config.WorkDir == "" {
		return nil, errors.New("workdir obrigatório")
	}
	s := &Sandbox{config: config}
	if runtime.GOOS == "linux" {
		s.landlockOK = landlockAvailable()
		if !s.landlockOK {
			slog.Warn("Landlock indisponível; execução sem isolamento de filesystem")
		}
	} else {
		slog.Warn("sandbox Landlock só no Linux", "os", runtime.GOOS)
	}
	return s, nil
}

func (s *Sandbox) Run(ctx context.Context, command string, args ...string) (*Result, error) {
	if s.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, command, args...) //nolint:gosec // sandbox invocado explicitamente pelo operador
	cmd.Dir = s.config.WorkDir

	if s.landlockOK {
		_ = applyLandlock(s.config.WorkDir, s.config.AllowedPaths, s.config.ForbiddenPaths)
	}

	start := time.Now()
	out, err := cmd.CombinedOutput()
	res := &Result{Output: string(out), Duration: time.Since(start), Sandboxed: s.landlockOK}

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
