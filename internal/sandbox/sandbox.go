package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var ErrUnsupported = errors.New("Landlock indisponível nesta plataforma")

type Config struct {
	WorkDir          string
	AllowedPaths     []PathAccess
	ForbiddenPaths   []string
	Timeout          time.Duration
	AllowUnsandboxed bool
	// Env, when non-nil, is the child's full environment (use execenv.Filter
	// to strip secrets). nil keeps the previous behavior: the child inherits the
	// current process environment.
	Env []string
	// ProcessGroup puts the child in its own process group and kills the whole
	// group on context cancellation/timeout (used by the router/watchdog).
	ProcessGroup bool
	// Registry, when non-nil (only with ProcessGroup), tracks the child's pid and,
	// after the agent exits, waits for the group to drain so descendants that
	// escaped (setsid/setpgid) show up as orphans in Registry.Sweep. nil
	// (default, `tb sandbox run`) keeps the previous behavior.
	Registry *Registry
	// GroupOwner identifies the group's owner (the ticket) in the Registry:
	// orphaned descendants found in this group whose environment names another
	// owner are treated as an escape (see Registry.sweepLocked). Only applies with
	// ProcessGroup and Registry.
	GroupOwner string
	// Stdin, when non-nil, becomes the child's standard input (it survives the
	// Landlock trampoline: syscall.Exec preserves descriptors).
	Stdin io.Reader
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
	Canceled  bool          `json:"canceled,omitempty"`
	Sandboxed bool          `json:"sandboxed"`
}

func New(config Config) (*Sandbox, error) { //nolint:gocritic // stable public API passed by value
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
		_, _, probeErr := landlockSupported()
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
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // sandbox explicitly invoked by the operator
	cmd.Dir = s.config.WorkDir
	cmd.Stdin = s.config.Stdin
	if s.config.Env != nil {
		cmd.Env = s.config.Env
	}
	return finishRun(ctx, cmd, false, s.config.ProcessGroup, s.config.Registry, s.config.GroupOwner)
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
	cmd := exec.CommandContext(ctx, self) //nolint:gosec // re-exec of our own binary as the Landlock trampoline
	cmd.Dir = s.config.WorkDir
	cmd.Stdin = s.config.Stdin
	cmd.Env = env
	return finishRun(ctx, cmd, true, s.config.ProcessGroup, s.config.Registry, s.config.GroupOwner)
}

// MaxOutputBytes caps the child output kept in memory/log: only the tail is
// preserved.
const MaxOutputBytes = 1 << 20

const truncatedMarker = "[... saída anterior truncada; mantidos os últimos 1 MiB ...]\n"

// tailBuffer is a goroutine-safe io.Writer that keeps only the last max bytes
// written. It never returns an error (no backpressure on the child).
type tailBuffer struct {
	mu        sync.Mutex
	max       int
	buf       []byte
	truncated bool
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > 2*t.max {
		n := copy(t.buf, t.buf[len(t.buf)-t.max:])
		t.buf = t.buf[:n]
		t.truncated = true
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	b := t.buf
	if len(b) > t.max {
		b = b[len(b)-t.max:]
		t.truncated = true
	}
	if t.truncated {
		return truncatedMarker + string(b)
	}
	return string(b)
}

func finishRun(ctx context.Context, cmd *exec.Cmd, sandboxed, group bool, reg *Registry, owner string) (*Result, error) {
	if group {
		killProcessGroup(cmd)
		// Pdeathsig (linux) applies to the thread that forks: keep this goroutine
		// pinned to the same thread until the child exits.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}
	out := &tailBuffer{max: MaxOutputBytes}
	cmd.Stdout = out
	cmd.Stderr = out
	start := time.Now()
	var err error
	if group && reg != nil {
		err = reg.RunOwned(cmd, true, owner)
	} else {
		err = cmd.Run()
	}
	if group {
		// On every path (success, failure, cancellation) nothing in the group may
		// outlive the agent: background grandchildren are terminated.
		reapProcessGroup(cmd)
		if reg != nil && cmd.Process != nil {
			settleGroup(reg, cmd.Process.Pid)
		}
	}
	res := &Result{Output: out.String(), Duration: time.Since(start), Sandboxed: sandboxed}

	// The agent exited 0 but left a child holding stdout/stderr: Go returns
	// ErrWaitDelay. The agent succeeded; the group was already terminated above.
	if err != nil && errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
		err = nil
	}

	if err != nil {
		switch ctx.Err() {
		case context.DeadlineExceeded:
			res.TimedOut = true
			res.ExitCode = -1
			return res, nil
		case context.Canceled:
			res.Canceled = true
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

// isSubpath reports whether path is a strict descendant of base (base itself
// does not count). filepath.Rel results that climb up ("..", "../x") are
// excluded; names that merely start with ".." (e.g. "..x") are legitimate
// children.
func isSubpath(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." || filepath.IsAbs(rel) {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func landlockAvailable() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	ok, _, _ := landlockSupported()
	return ok
}

// landlockSupported is the Landlock probe used by New. It is a variable so
// tests can simulate a host without Landlock (see OverrideLandlockProbe); in
// production it is always CheckLandlockSupport.
var landlockSupported = CheckLandlockSupport

// OverrideLandlockProbe replaces the Landlock probe and returns a function that
// restores it. Tests ONLY (this package's and packages that need to exercise
// fail-closed, like the router): it verifies that without Landlock and without
// AllowUnsandboxed nothing runs, regardless of the CI kernel.
func OverrideLandlockProbe(fn func() (bool, int, error)) (restore func()) {
	prev := landlockSupported
	landlockSupported = fn
	return func() { landlockSupported = prev }
}
