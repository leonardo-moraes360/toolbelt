package egm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
)

// Goldfish failure reasons (R3, R15a, R16).
const (
	ReasonTimeout        = "timeout"
	ReasonStdoutOverflow = "stdout_overflow"
	ReasonMissingOutput  = "missing_output"
	ReasonToolsUsed      = "tools_used"
	ReasonEscaped        = "escaped_process"
	ReasonUnattributed   = "unattributed_orphan"
	ReasonSandboxError   = "sandbox_error"
	// ReasonInterrupted: the caller's context was canceled (SIGINT, budget);
	// the goldfish neither failed nor completed (R3, R27).
	ReasonInterrupted = "interrupted"
)

const (
	workspacePrefix = "tb-egm-"
	ownerPIDFile    = "owner.pid"
	// staleAge: an abandoned workspace this old whose owner is dead is swept.
	staleAge     = time.Hour
	stderrTailMx = 8 << 10
)

// GoldfishRequest is one goldfish (or arbiter) execution.
type GoldfishRequest struct {
	// Owner is TB_EGM_OWNER and the sandbox GroupOwner, e.g. egm:<slug>:r3:design.
	Owner string
	// Attempt is 1 or 2 (TB_EGM_ATTEMPT).
	Attempt int
	// Packet is the absolute, read-only workdir (already holds prompt.txt).
	Packet string
	// Prompt is sent on stdin.
	Prompt string
}

// GoldfishResult is the outcome of one Run.
type GoldfishResult struct {
	// Completed: the process ran and produced a payload.
	Completed bool
	// Reason explains !Completed: timeout, stdout_overflow, missing_output,
	// tools_used, escaped_process, unattributed_orphan, exit_<n>,
	// sandbox_error or interrupted.
	Reason string
	// Payload is the reviewer's text without the envelope.
	Payload []byte
	// ToolsUsed: the envelope shows tool use (R15a).
	ToolsUsed bool
	// Stderr is a tail for debugging; it never goes into the report.
	Stderr string
}

// Runner runs goldfish under one Registry and one subreaper (R3). It is safe
// for concurrent use.
type Runner struct {
	prof      *Profile
	bin       string // resolved agent binary
	args      []string
	path      string
	pass      []string
	readOnly  []string
	copies    []string // egm.home.copy, relative to realHome
	realHome  string
	credPath  string // claude only
	// apiKey: a claude API key or token reaches the goldfish through
	// egm.env.pass (the spike's first auth preference, R0), so no credential
	// file is checked or copied.
	apiKey bool
	timeout   time.Duration
	maxStdout int

	reg     *sandbox.Registry
	release func()

	// sweepMu serializes Sweep with the bookkeeping below, so a Run that
	// checks Escaped after its own sweep sees orphans another Run collected.
	sweepMu sync.Mutex
	escaped map[string]bool
	// known are the owners this runner started: an orphan declaring any
	// other owner is treated as unattributed, so it cannot pick a victim
	// that does not exist or hide behind a made-up name.
	known        map[string]bool
	unattributed bool
	running      map[*runSlot]struct{}
	closed       bool
}

// runSlot is a goldfish in flight; killed marks it as stopped because of an
// unattributed orphan.
type runSlot struct {
	cancel context.CancelFunc
	killed bool
}

// NewRunner checks the platform (Linux + Landlock + subreaper) and the
// vendor auth, sweeps stale workspaces and prepares the shared Registry.
func NewRunner(set *Settings, p *Profile) (*Runner, error) {
	if err := checkPlatform(); err != nil {
		return nil, err
	}
	r, err := newRunnerConfig(set, p)
	if err != nil {
		return nil, err
	}
	if p.Vendor == VendorClaude && !r.apiKey {
		if err := checkClaudeAuth(r.credPath); err != nil {
			return nil, err
		}
	}
	release, ok := sandbox.AcquireSubreaper()
	if !ok {
		release()
		return nil, errorf(ExitError, "no_subreaper", "the kernel refused PR_SET_CHILD_SUBREAPER: escaped processes could not be detected")
	}
	r.release = release
	r.reg = sandbox.NewRegistry(envOwner)
	r.reg.SetOwnerVar(envOwner)
	sweepStaleWorkspaces(os.TempDir(), time.Now())
	return r, nil
}

func checkPlatform() error {
	if runtime.GOOS != "linux" {
		return errorf(ExitError, "no_landlock", "the llm engine requires Linux with Landlock (this is %s)", runtime.GOOS)
	}
	if ok, _, err := sandbox.CheckLandlockSupport(); !ok {
		return errorf(ExitError, "no_landlock", "the llm engine requires Landlock: %v", err)
	}
	return nil
}

// newRunnerConfig resolves everything that does not change between runs.
func newRunnerConfig(set *Settings, p *Profile) (*Runner, error) {
	timeout, err := time.ParseDuration(set.TimeoutPerReviewer)
	if err != nil || timeout <= 0 {
		return nil, errorf(ExitError, "usage", "egm.timeout_per_reviewer must be a positive duration")
	}
	// The sandbox execs the target of a symlinked binary, so Landlock must
	// allow the real directory (e.g. ~/.local/share/claude/versions).
	bin, err := filepath.EvalSymlinks(p.Argv[0])
	if err != nil {
		return nil, errorf(ExitError, "usage", "egm.agent_cmd[0]: %v", err)
	}
	home := realHome()
	copies, err := homeCopies(set.Home.Copy)
	if err != nil {
		return nil, err
	}
	r := &Runner{
		prof: p, bin: bin, args: p.Argv[1:], path: set.Path, pass: set.Env.Pass,
		copies: copies, realHome: home, timeout: timeout, maxStdout: int(set.MaxStdoutBytes),
		escaped: map[string]bool{}, known: map[string]bool{}, running: map[*runSlot]struct{}{},
	}
	if r.path == "" {
		r.path = "/usr/bin:/bin"
	}
	if p.Vendor == VendorClaude {
		r.credPath = filepath.Join(home, ".claude", ".credentials.json")
		r.apiKey = claudeAPIKey(set.Env.Pass)
	}
	r.readOnly = readOnlyPaths(bin, home, set.Home.ReadOnly)
	return r, nil
}

// homeCopies validates egm.home.copy: relative paths that stay inside HOME.
func homeCopies(list []string) ([]string, error) {
	out := make([]string, 0, len(list))
	for _, rel := range list {
		c := filepath.Clean(rel)
		if filepath.IsAbs(c) || c == "." || c == ".." || strings.HasPrefix(c, ".."+string(filepath.Separator)) {
			return nil, errorf(ExitError, "usage", "egm.home.copy entry %q must be relative to HOME", rel)
		}
		out = append(out, c)
	}
	return out, nil
}

// readOnlyPaths lists the R4 read-only rules: the binary's directory,
// egm.home.read_only (absolute or relative to HOME; missing ones are
// skipped, Landlock refuses rules on absent paths) and the realpath of
// /etc/resolv.conf when systemd-resolved points it outside /etc.
func readOnlyPaths(bin, home string, extra []string) []string {
	out := []string{filepath.Dir(bin)}
	for _, p := range extra {
		if !filepath.IsAbs(p) {
			p = filepath.Join(home, p)
		}
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			out = append(out, resolved)
		}
	}
	if target, err := filepath.EvalSymlinks("/etc/resolv.conf"); err == nil && !strings.HasPrefix(target, "/etc/") {
		out = append(out, target)
	}
	return out
}

// Close releases the subreaper. Runs after Close fail with sandbox_error.
func (r *Runner) Close() {
	r.sweepMu.Lock()
	defer r.sweepMu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	r.release()
}

// Run executes one goldfish and always removes its workspace.
func (r *Runner) Run(ctx context.Context, req *GoldfishRequest) *GoldfishResult {
	ws, err := newWorkspace(os.TempDir())
	if err != nil {
		return sandboxFailure(err)
	}
	defer ws.remove()
	extra, err := r.prepareHome(ws.home)
	if err != nil {
		return sandboxFailure(err)
	}
	var stdout bytes.Buffer
	cfg := r.sandboxConfig(req, ws, &stdout, extra)
	sb, err := sandbox.New(cfg)
	if err != nil {
		return sandboxFailure(err)
	}
	slot, runCtx, ok := r.begin(ctx, req.Owner)
	if !ok {
		return sandboxFailure(errors.New("runner closed"))
	}
	res, runErr := sb.Run(runCtx, r.bin, r.args...)
	r.end(slot)
	r.Sweep()
	return r.classify(ctx, req, slot, res, runErr, stdout.Bytes())
}

// prepareHome installs the vendor auth and egm.home.copy in the disposable
// HOME and returns the vendor's extra env.
func (r *Runner) prepareHome(home string) ([]string, error) {
	for _, rel := range r.copies {
		if err := copyHomeFile(filepath.Join(r.realHome, rel), filepath.Join(home, rel)); err != nil {
			return nil, err
		}
	}
	if r.prof.Vendor != VendorClaude {
		return nil, nil
	}
	if r.apiKey {
		dir := filepath.Join(home, ".claude")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
		return []string{"CLAUDE_CONFIG_DIR=" + dir}, nil
	}
	if err := installClaudeCredential(r.credPath, home); err != nil {
		return nil, fmt.Errorf("installing the claude credential: %w", err)
	}
	return []string{"CLAUDE_CONFIG_DIR=" + filepath.Join(home, ".claude")}, nil
}

func (r *Runner) sandboxConfig(req *GoldfishRequest, ws *workspace, stdout *bytes.Buffer, extra []string) sandbox.Config {
	allowed := make([]sandbox.PathAccess, 0, len(r.readOnly))
	for _, p := range r.readOnly {
		allowed = append(allowed, sandbox.PathAccess{Path: p, ReadOnly: true})
	}
	return sandbox.Config{
		WorkDir:         req.Packet,
		ReadOnlyWorkDir: true,
		PrivateTmp:      ws.tmp,
		Home:            ws.home,
		Path:            r.path,
		AllowedPaths:    allowed,
		Env:             goldfishEnv(req.Owner, req.Attempt, r.pass, extra...),
		Stdin:           strings.NewReader(req.Prompt),
		Stdout:          stdout,
		MaxStdoutBytes:  r.maxStdout,
		MinimalDev:      true,
		ProcessGroup:    true,
		Registry:        r.reg,
		GroupOwner:      req.Owner,
		Timeout:         r.timeout,
	}
}

// begin registers a goldfish in flight so an unattributed orphan can kill it.
func (r *Runner) begin(ctx context.Context, owner string) (*runSlot, context.Context, bool) {
	runCtx, cancel := context.WithCancel(ctx)
	slot := &runSlot{cancel: cancel}
	r.sweepMu.Lock()
	defer r.sweepMu.Unlock()
	r.known[owner] = true
	if r.closed {
		cancel()
		return nil, nil, false
	}
	r.running[slot] = struct{}{}
	return slot, runCtx, true
}

func (r *Runner) end(slot *runSlot) {
	r.sweepMu.Lock()
	delete(r.running, slot)
	r.sweepMu.Unlock()
	slot.cancel()
}

// Sweep collects escaped processes and attributes them (R3). It runs after
// each goldfish; the caller also calls it at the end of phase 2.
func (r *Runner) Sweep() {
	r.sweepMu.Lock()
	defer r.sweepMu.Unlock()
	orphans := r.reg.Sweep()
	for i := range orphans {
		r.attributeLocked(&orphans[i])
	}
}

// attributeLocked records the owner an orphan declares in its own env
// first, then the owner of the group it hid in. Without an owner the
// goldfish still running are killed: any of them may be the culprit.
func (r *Runner) attributeLocked(o *sandbox.Orphan) {
	owner := o.Vars[envOwner]
	if owner == "" {
		owner = o.GroupOwner
	}
	if owner != "" && r.known[owner] {
		r.escaped[owner] = true
		return
	}
	r.unattributed = true
	for slot := range r.running {
		slot.killed = true
		slot.cancel()
	}
}

// Escaped reports whether an orphan was attributed to owner.
func (r *Runner) Escaped(owner string) bool {
	r.sweepMu.Lock()
	defer r.sweepMu.Unlock()
	return r.escaped[owner]
}

// ResetUnattributed clears the flag at the start of a phase, so an orphan
// left by an arbiter does not void the reviewers of the round (R3 scopes the
// rule to phase 2).
func (r *Runner) ResetUnattributed() {
	r.sweepMu.Lock()
	r.unattributed = false
	r.sweepMu.Unlock()
}

// Unattributed reports whether an orphan without owner was seen; the round
// is then not clean (R3).
func (r *Runner) Unattributed() bool {
	r.sweepMu.Lock()
	defer r.sweepMu.Unlock()
	return r.unattributed
}

func (r *Runner) wasKilled(slot *runSlot) bool {
	r.sweepMu.Lock()
	defer r.sweepMu.Unlock()
	return slot.killed
}

// classify turns the sandbox result into a GoldfishResult. Order matters:
// isolation failures (no retry) win over transient ones (retry, R16).
func (r *Runner) classify(ctx context.Context, req *GoldfishRequest, slot *runSlot, res *sandbox.Result, runErr error, stdout []byte) *GoldfishResult {
	if runErr != nil {
		return sandboxFailure(runErr)
	}
	out := &GoldfishResult{Stderr: tail(res.Output, stderrTailMx)}
	switch {
	case r.wasKilled(slot):
		out.Reason = ReasonUnattributed
	case r.Escaped(req.Owner):
		out.Reason = ReasonEscaped
	case res.TimedOut:
		out.Reason = ReasonTimeout
	case res.Canceled || ctx.Err() != nil:
		out.Reason = ReasonInterrupted
	case res.StdoutOverflow:
		out.Reason = ReasonStdoutOverflow
	case res.ExitCode != 0:
		out.Reason = "exit_" + strconv.Itoa(res.ExitCode)
	default:
		r.fillPayload(out, stdout)
	}
	return out
}

func (r *Runner) fillPayload(out *GoldfishResult, stdout []byte) {
	payload, used, err := extractPayload(r.prof.Output, stdout)
	out.ToolsUsed = used
	switch {
	case used:
		out.Reason = ReasonToolsUsed
	case err != nil || len(bytes.TrimSpace(payload)) == 0:
		out.Reason = ReasonMissingOutput
	default:
		out.Completed, out.Payload = true, payload
	}
}

func sandboxFailure(err error) *GoldfishResult {
	return &GoldfishResult{Reason: ReasonSandboxError, Stderr: err.Error()}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// workspace is the disposable HOME and TMPDIR of one goldfish (R4).
type workspace struct {
	root, home, tmp string
}

func newWorkspace(base string) (*workspace, error) {
	root, err := os.MkdirTemp(base, workspacePrefix) // 0700
	if err != nil {
		return nil, err
	}
	ws := &workspace{root: root, home: filepath.Join(root, "home"), tmp: filepath.Join(root, "tmp")}
	for _, d := range []string{ws.home, ws.tmp} {
		if err := os.Mkdir(d, 0o700); err != nil {
			ws.remove()
			return nil, err
		}
	}
	pid := []byte(strconv.Itoa(os.Getpid()))
	if err := os.WriteFile(filepath.Join(root, ownerPIDFile), pid, 0o600); err != nil {
		ws.remove()
		return nil, err
	}
	return ws, nil
}

func (ws *workspace) remove() { _ = os.RemoveAll(ws.root) }

// copyHomeFile copies one egm.home.copy file with 0600; only regular files,
// so a symlink cannot pull something else from the real HOME.
func copyHomeFile(src, dst string) error {
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("egm.home.copy: %s is not a regular file", src)
	}
	b, err := os.ReadFile(src) //nolint:gosec // path from the user config, checked above
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o600)
}

// sweepStaleWorkspaces removes tb-egm-* left by a crashed tb: older than
// staleAge and whose owner.pid is dead. Directories without owner.pid are
// not ours (e.g. the R0 spike) and are left alone.
func sweepStaleWorkspaces(base string, now time.Time) {
	matches, _ := filepath.Glob(filepath.Join(base, workspacePrefix+"*"))
	for _, dir := range matches {
		if staleWorkspace(dir, now) {
			_ = os.RemoveAll(dir)
		}
	}
}

func staleWorkspace(dir string, now time.Time) bool {
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() || now.Sub(fi.ModTime()) < staleAge {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(dir, ownerPIDFile)) //nolint:gosec // our own workspace layout
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return false
	}
	return !pidAlive(pid)
}

// pidAlive uses signal 0; EPERM means the pid exists under another uid.
func pidAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
