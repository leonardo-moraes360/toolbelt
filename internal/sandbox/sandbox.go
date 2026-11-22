package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

var ErrUnsupported = errors.New("Landlock unavailable on this platform")

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

	// ReadOnlyWorkDir makes the workdir rule read-only (default: read-write).
	ReadOnlyWorkDir bool
	// PrivateTmp, when non-empty, is a writable directory exported as TMPDIR.
	PrivateTmp string
	// Home, when non-empty, is a writable directory exported as HOME and used as
	// the base of XDG_CONFIG_HOME, XDG_CACHE_HOME and XDG_DATA_HOME. It
	// overrides the values in Env.
	Home string
	// Path, when non-empty, is exported as PATH and overrides Env.
	Path string
	// Stdout, when non-nil, receives only the child's stdout (stderr stays in
	// Result.Output). MaxStdoutBytes caps what is forwarded: the excess is read
	// and discarded, so the child never blocks on the pipe, and
	// Result.StdoutOverflow becomes true. MaxStdoutBytes <= 0 uses
	// MaxOutputBytes.
	Stdout         io.Writer
	MaxStdoutBytes int
	// MinimalDev replaces read access to all of /dev with rules only for
	// /dev/null, /dev/zero, /dev/full, /dev/random and /dev/urandom, so /dev/pts
	// and /dev/shm stay closed.
	MinimalDev bool
	// DenyNetwork makes socket(2) and socketpair(2) fail with EPERM in the child
	// (seccomp filter installed by the trampoline; with Landlock ABI >= 6 also
	// abstract unix socket and signal scoping). New fails closed when
	// NetworkDenyAvailable is false, unless AllowUnsandboxed.
	DenyNetwork bool
}

type PathAccess struct {
	Path     string
	ReadOnly bool
}

type Sandbox struct {
	config     Config
	landlockOK bool
	// netDenyOK is false only when DenyNetwork was requested, cannot be
	// enforced and AllowUnsandboxed accepted that: the run keeps Landlock (if
	// any) but reports Sandboxed=false.
	netDenyOK bool
}

type Result struct {
	Output   string        `json:"output"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration_ns"`
	TimedOut bool          `json:"timed_out"`
	Canceled bool          `json:"canceled,omitempty"`
	// Signal names the signal that killed the child (SIGKILL, ...); ExitCode
	// is then -1 and SignalNum holds the number.
	Signal    string `json:"signal,omitempty"`
	SignalNum int    `json:"-"`
	Sandboxed bool   `json:"sandboxed"`
	// StdoutOverflow reports that stdout went past MaxStdoutBytes (only with
	// Config.Stdout).
	StdoutOverflow bool `json:"stdout_overflow,omitempty"`
}

// ErrCommand marks a command that could not be resolved to an executable
// (not found, or not executable); tb sandbox run exits 127 on it.
var ErrCommand = errors.New("cannot run command")

var signalNames = map[syscall.Signal]string{
	syscall.SIGHUP: "SIGHUP", syscall.SIGINT: "SIGINT", syscall.SIGQUIT: "SIGQUIT", syscall.SIGILL: "SIGILL",
	syscall.SIGABRT: "SIGABRT", syscall.SIGBUS: "SIGBUS", syscall.SIGFPE: "SIGFPE", syscall.SIGKILL: "SIGKILL",
	syscall.SIGSEGV: "SIGSEGV", syscall.SIGPIPE: "SIGPIPE", syscall.SIGALRM: "SIGALRM", syscall.SIGTERM: "SIGTERM",
}

func signalName(sig syscall.Signal) string {
	if n, ok := signalNames[sig]; ok {
		return n
	}
	return fmt.Sprintf("signal %d", int(sig))
}

func New(config Config) (*Sandbox, error) { //nolint:gocritic // stable public API passed by value
	if config.WorkDir == "" {
		return nil, errors.New("workdir required")
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
			return nil, fmt.Errorf("%w; use --allow-unsandboxed to opt in to running without isolation", ErrUnsupported)
		}
		_, _, probeErr := landlockSupported()
		if probeErr == nil {
			probeErr = ErrUnsupported
		}
		return nil, fmt.Errorf("Landlock unavailable (%v); use --allow-unsandboxed to opt in to running without isolation", probeErr)
	}
	s.netDenyOK = !config.DenyNetwork || (s.landlockOK && NetworkDenyAvailable())
	if !s.netDenyOK && !config.AllowUnsandboxed {
		return nil, fmt.Errorf("%w: network denial (seccomp filter) unavailable; use --allow-unsandboxed to opt in to running without isolation", ErrUnsupported)
	}
	return s, nil
}

func (s *Sandbox) Run(ctx context.Context, command string, args ...string) (*Result, error) {
	if s.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()
	}

	// Resolved before the Landlock helper starts, so a bare name and a path
	// fail the same way.
	bin, err := exec.LookPath(command)
	if err != nil {
		kind := "not executable"
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
			kind = "not found"
		}
		return nil, fmt.Errorf("%w (%s): %w", ErrCommand, kind, err)
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
	if s.config.Env != nil || s.config.hasEnvOverrides() {
		base := s.config.Env
		if base == nil {
			base = os.Environ()
		}
		cmd.Env = s.config.applyEnvOverrides(base)
	}
	return finishRun(ctx, cmd, false, &s.config)
}

func (s *Sandbox) runIsolated(ctx context.Context, bin string, args ...string) (*Result, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("current executable: %w", err)
	}
	cfg := s.config
	if !s.netDenyOK {
		// Reached only with AllowUnsandboxed (New refuses otherwise); the result
		// says the requested isolation was not applied.
		cfg.DenyNetwork = false
	}
	env, err := prepareChildEnv(&cfg, bin, args)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, self) //nolint:gosec // re-exec of our own binary as the Landlock trampoline
	cmd.Dir = s.config.WorkDir
	cmd.Stdin = s.config.Stdin
	cmd.Env = env
	return finishRun(ctx, cmd, s.netDenyOK, &s.config)
}

func (c *Config) hasEnvOverrides() bool {
	return c.Home != "" || c.Path != "" || c.PrivateTmp != ""
}

// applyEnvOverrides returns a copy of env with HOME/XDG_*, PATH and TMPDIR
// replaced by the non-empty Home, Path and PrivateTmp fields.
func (c *Config) applyEnvOverrides(env []string) []string {
	set := map[string]string{}
	if c.Home != "" {
		set["HOME"] = c.Home
		set["XDG_CONFIG_HOME"] = filepath.Join(c.Home, ".config")
		set["XDG_CACHE_HOME"] = filepath.Join(c.Home, ".cache")
		set["XDG_DATA_HOME"] = filepath.Join(c.Home, ".local", "share")
	}
	if c.Path != "" {
		set["PATH"] = c.Path
	}
	if c.PrivateTmp != "" {
		set["TMPDIR"] = c.PrivateTmp
	}
	out := make([]string, 0, len(env)+len(set))
	for _, e := range env {
		k, _, _ := strings.Cut(e, "=")
		if _, ok := set[k]; ok {
			continue
		}
		out = append(out, e)
	}
	for _, k := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "PATH", "TMPDIR"} {
		if v, ok := set[k]; ok {
			out = append(out, k+"="+v)
		}
	}
	return out
}

// limitedWriter forwards up to max bytes to w and discards the rest without
// returning an error, so the child never blocks on the pipe.
type limitedWriter struct {
	mu       sync.Mutex
	w        io.Writer
	max      int
	written  int
	overflow bool
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(p)
	room := l.max - l.written
	if room <= 0 {
		l.overflow = true
		return n, nil
	}
	chunk := p
	if len(chunk) > room {
		chunk = chunk[:room]
		l.overflow = true
	}
	if _, err := l.w.Write(chunk); err != nil {
		l.overflow = true
	}
	l.written += len(chunk)
	return n, nil
}

func (l *limitedWriter) Overflow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.overflow
}

// MaxOutputBytes caps the child output kept in memory/log: only the tail is
// preserved.
const MaxOutputBytes = 1 << 20

const truncatedMarker = "[... earlier output truncated; last 1 MiB kept ...]\n"

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

func finishRun(ctx context.Context, cmd *exec.Cmd, sandboxed bool, cfg *Config) (*Result, error) {
	group, reg, owner := cfg.ProcessGroup, cfg.Registry, cfg.GroupOwner
	if group {
		killProcessGroup(cmd)
		// Pdeathsig (linux) applies to the thread that forks: keep this goroutine
		// pinned to the same thread until the child exits.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}
	out := &tailBuffer{max: MaxOutputBytes}
	cmd.Stderr = out
	stdout := attachStdout(cmd, cfg, out)
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
	if stdout != nil {
		res.StdoutOverflow = stdout.Overflow()
	}

	return classifyRunError(ctx, cmd, res, err)
}

// attachStdout points the child's stdout at the tail buffer, or, with
// Config.Stdout, at a limitedWriter over it (returned so the caller can read
// the overflow flag).
func attachStdout(cmd *exec.Cmd, cfg *Config, out *tailBuffer) *limitedWriter {
	if cfg.Stdout == nil {
		cmd.Stdout = out
		return nil
	}
	limit := cfg.MaxStdoutBytes
	if limit <= 0 {
		limit = MaxOutputBytes
	}
	lw := &limitedWriter{w: cfg.Stdout, max: limit}
	cmd.Stdout = lw
	return lw
}

// classifyRunError turns the child's exit into Result fields: timeout,
// cancellation and a non-zero exit are results, not errors.
func classifyRunError(ctx context.Context, cmd *exec.Cmd, res *Result, err error) (*Result, error) {
	// The agent exited 0 but left a child holding stdout/stderr: Go returns
	// ErrWaitDelay. The agent succeeded; the group was already terminated.
	if err == nil || (errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success()) {
		return res, nil
	}
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
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			res.SignalNum = int(ws.Signal())
			res.Signal = signalName(ws.Signal())
		}
		return res, nil
	}
	return nil, err
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
			return fmt.Errorf("forbidden path: %s", forbidden)
		}
	}
	if isSubpath(abs, work) || abs == work {
		return nil
	}
	return fmt.Errorf("out of scope: %s", path)
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
