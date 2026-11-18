package router

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

const (
	AlertWorkerSlow    = "worker_slow"
	AlertWorkerStuck   = "worker_stuck"
	AlertWorkerCrash   = "worker_crash"
	AlertWorkerKilled  = "worker_killed"
	AlertQueueGrowing  = "queue_growing"
	AlertGlobalTimeout = "global_timeout"
	// AlertTicketCanceled: the running ticket was canceled (tb ticket cancel);
	// the router kills the agent and drops the result.
	AlertTicketCanceled = "ticket_canceled"
	// AlertMultiDepBase: a ticket with >=2 unmerged dependencies was based on HEAD.
	AlertMultiDepBase = "multi_dep_base"
	// AlertAgentEscaped: an agent descendant escaped the process group
	// (setsid/setpgid); it was killed and the ticket fails without retry (Linux).
	AlertAgentEscaped = "agent_escaped"
	// AlertOrphanKilled: a non-agent orphan (e.g. a user hook/fsmonitor daemon
	// during worktree creation, or an unattributable escape with no agent running)
	// was terminated; no ticket is failed.
	AlertOrphanKilled = "orphan_killed"
	// AlertBoardUnreadable: a ticket YAML is invalid; workers keep running.
	AlertBoardUnreadable = "board_unreadable"
)

const (
	SevInfo     = "info"
	SevWarn     = "warn"
	SevError    = "error"
	SevCritical = "critical"
)

const (
	ActionNone     = "none"
	ActionRetry    = "retry"
	ActionEscalate = "escalate"
	ActionStop     = "stop"
)

type Alert struct {
	Time     time.Time `json:"time"`
	Kind     string    `json:"kind"`
	Severity string    `json:"severity"`
	Worker   string    `json:"worker,omitempty"`
	Ticket   string    `json:"ticket,omitempty"`
	Message  string    `json:"message"`
	Action   string    `json:"action"`
}

// alertLog writes to alerts.jsonl (append-only, goroutine-safe).
type alertLog struct {
	mu    sync.Mutex
	path  string
	count int
}

func (l *alertLog) append(a *Alert) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // internal .toolbelt/router path
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	l.count++
	return nil
}

type AlertFilter struct {
	Since time.Time // zero = no limit
	Kind  string    // empty = all
	Limit int       // >0 = only the N most recent
}

// ReadAlerts returns alerts in chronological order (missing file = empty list).
func ReadAlerts(root string, f AlertFilter) ([]Alert, error) {
	file, err := os.Open(AlertsPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return []Alert{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	out := []Alert{}
	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var a Alert
		if err := json.Unmarshal([]byte(line), &a); err != nil {
			continue // a corrupt line does not abort the read
		}
		if !f.Since.IsZero() && a.Time.Before(f.Since) {
			continue
		}
		if f.Kind != "" && a.Kind != f.Kind {
			continue
		}
		out = append(out, a)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[len(out)-f.Limit:]
	}
	return out, nil
}

var workerRe = regexp.MustCompile(`^w\d+$`)

// ValidWorkerID accepts the form w1, w2, ...
func ValidWorkerID(id string) bool { return workerRe.MatchString(id) }

type KillRequest struct {
	Worker      string    `json:"worker"`
	Reason      string    `json:"reason"`
	RequestedAt time.Time `json:"requested_at"`
}

// RequestKill writes a kill request that the router consumes on its next tick.
func RequestKill(root, worker, reason string, now time.Time) (string, error) {
	if !ValidWorkerID(worker) {
		return "", fmt.Errorf("invalid worker %q (expected w1, w2, ...)", worker)
	}
	if err := os.MkdirAll(ControlDir(root), 0o755); err != nil {
		return "", err
	}
	data, err := json.Marshal(KillRequest{Worker: worker, Reason: reason, RequestedAt: now.UTC()})
	if err != nil {
		return "", err
	}
	path := filepath.Join(ControlDir(root), "kill-"+worker+".json")
	return path, ticket.WriteFileAtomic(path, append(data, '\n'))
}

// alert also forwards the alert to progress output.
func (r *Router) alert(kind, severity, worker, tkt, msg, action string) {
	a := Alert{Time: r.now().UTC(), Kind: kind, Severity: severity, Worker: worker, Ticket: tkt, Message: msg, Action: action}
	if err := r.alerts.append(&a); err != nil {
		r.logf("warning: could not write alert: %v", err)
	}
	r.logf("[alert %s/%s] %s", severity, kind, msg)
}

func (r *Router) failureAction(attempts int) string {
	if attempts < r.cfg.Retry.MaxAttempts {
		return ActionRetry
	}
	return ActionEscalate
}

// watchdogTick runs every Options.Tick: control, slow/stuck, queue and global
// timeout. Returns true when the loop should stop (global timeout).
func (r *Router) watchdogTick() (stop bool) {
	now := r.now()
	r.processControl()
	r.sweepTick()
	for _, s := range r.slots {
		if s.ticket == "" || s.reason != "" {
			continue
		}
		if cur, err := r.store.Get(s.ticket); err == nil && (cur.Status != ticket.StatusRunning || cur.Worker != s.id) {
			s.reason = reasonCanceled
			r.alert(AlertTicketCanceled, SevWarn, s.id, s.ticket,
				fmt.Sprintf("%s was %s outside the router; stopping the agent on %s and discarding the result", s.ticket, cur.Status, s.id),
				ActionNone)
			s.cancel()
			continue
		}
		elapsed := now.Sub(s.started)
		if !s.slowAlerted && elapsed >= time.Duration(r.cfg.SlowThreshold*float64(s.timeout)) && elapsed < s.timeout {
			s.slowAlerted = true
			r.alert(AlertWorkerSlow, SevWarn, s.id, s.ticket,
				fmt.Sprintf("%s on %s for %s (limit %s)", s.ticket, s.id, elapsed.Round(time.Millisecond), s.timeout),
				ActionNone)
		}
		if elapsed >= s.timeout {
			s.reason = reasonStuck
			r.alert(AlertWorkerStuck, SevCritical, s.id, s.ticket,
				fmt.Sprintf("%s on %s exceeded the timeout of %s; canceling", s.ticket, s.id, s.timeout),
				r.failureAction(s.attempts))
			s.cancel()
		}
	}
	if now.Sub(r.startedAt) >= r.global {
		r.alert(AlertGlobalTimeout, SevCritical, "", "",
			fmt.Sprintf("global timeout of %s reached; stopping the router", r.global), ActionStop)
		r.stopReason = ReasonGlobalTimeout
		return true
	}
	return false
}

// checkQueue emits queue_growing when ready > 2*workers (once per transition).
func (r *Router) checkQueue(ready int) {
	growing := ready > 2*r.workers
	if growing && !r.queueGrowing {
		r.alert(AlertQueueGrowing, SevWarn, "", "",
			fmt.Sprintf("queue growing: %d tickets ready for %d workers", ready, r.workers), ActionNone)
	}
	r.queueGrowing = growing
}

func (r *Router) processControl() {
	for _, name := range r.killRequestNames() {
		r.handleKillRequest(name)
	}
}

func (r *Router) killRequestNames() []string {
	entries, err := os.ReadDir(ControlDir(r.opts.Root))
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "kill-") && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// handleKillRequest reads (and deletes) a kill request and, if valid, cancels the worker.
func (r *Router) handleKillRequest(name string) {
	path := filepath.Join(ControlDir(r.opts.Root), name)
	data, err := os.ReadFile(path) //nolint:gosec // inside .toolbelt/router/control
	fi, _ := os.Stat(path)
	_ = os.Remove(path)
	if err != nil {
		return
	}
	var req KillRequest
	parsed := json.Unmarshal(data, &req) == nil
	if !parsed || req.Worker == "" {
		req.Worker = strings.TrimSuffix(strings.TrimPrefix(name, "kill-"), ".json")
	}
	// A request older than the start of this run is stale (without a readable
	// timestamp, the file's mtime is used).
	stamp := req.RequestedAt
	if (!parsed || stamp.IsZero()) && fi != nil {
		stamp = fi.ModTime()
	}
	if !stamp.IsZero() && stamp.Before(r.startedAt) {
		r.logf("stale kill ignored: %s", name)
		return
	}
	s := r.slotByID(req.Worker)
	if s == nil || s.ticket == "" || s.reason != "" {
		r.logf("kill ignored: %s is not running anything", req.Worker)
		return
	}
	s.reason = reasonKilled
	s.killReason = req.Reason
	msg := fmt.Sprintf("%s on %s terminated by tb watchdog kill", s.ticket, s.id)
	if req.Reason != "" {
		msg += ": " + req.Reason
	}
	r.alert(AlertWorkerKilled, SevWarn, s.id, s.ticket, msg, r.failureAction(s.attempts))
	s.cancel()
}

// ParseSince accepts "10m"/"2h" (relative to now) or RFC3339; empty = zero.
func ParseSince(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		if d < 0 {
			return time.Time{}, fmt.Errorf("--since negative: %q", s)
		}
		return now.Add(-d), nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --since %q (use 10m, 2h or RFC3339)", s)
	}
	return t, nil
}

// clearControl removes pending kill requests (stale at the start of Run).
func clearControl(root string) {
	entries, err := os.ReadDir(ControlDir(root))
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "kill-") && strings.HasSuffix(e.Name(), ".json") {
			_ = os.Remove(filepath.Join(ControlDir(root), e.Name()))
		}
	}
}
