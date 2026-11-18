package router

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/execenv"
	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
	"github.com/leonardo-moraes360/toolbelt/internal/workspace"
)

const (
	ReasonCompleted     = "completed"
	ReasonOnce          = "once"
	ReasonDrained       = "drained"
	ReasonInterrupted   = "interrupted"
	ReasonGlobalTimeout = "global_timeout"
	ReasonStalled       = "stalled"
)

// maxBoardFailures: consecutive ticks with an unreadable board before giving up.
const maxBoardFailures = 30

const (
	ViaBoard  = "board"
	ViaDirect = "direct"
)

const (
	reasonStuck    = "stuck"
	reasonKilled   = "killed"
	reasonShutdown = "shutdown"
	// reasonCanceled: the ticket is no longer running/owned by this worker (tb ticket cancel).
	reasonCanceled = "canceled"
)

type Options struct {
	Root             string              // repository root (where .toolbelt lives)
	Config           config.RouterConfig // zero value = defaults
	Workers          int                 // >0 overrides Config.MaxWorkers
	Strategy         string              // non-empty overrides Config.Strategy
	AgentCmd         []string            // non-empty overrides Config.AgentCmd
	Once             bool                // one scheduling round, then exit
	AllowUnsandboxed bool                // explicit opt-in (no Landlock)
	EnvOptions       execenv.Options     // allow/pass rules for child env vars
	Environ          []string            // base environment (nil = os.Environ())
	Now              func() time.Time    // injectable clock (nil = time.Now)
	Tick             time.Duration       // watchdog/loop interval (0 = 1s)
	Out              io.Writer           // human-readable progress (nil = discard)
}

type Assignment struct {
	Ticket  string `json:"ticket"`
	Worker  string `json:"worker"`
	Via     string `json:"via"` // board | direct
	Base    string `json:"base"`
	Attempt int    `json:"attempt"`
}

type Summary struct {
	Reason      string       `json:"reason"`
	Assignments []Assignment `json:"assignments"`
	Done        []string     `json:"done"`
	Failed      []string     `json:"failed"`
	Retried     []string     `json:"retried"`
	Requeued    []string     `json:"requeued"`
	// Unschedulable lists pending tickets that will never run (dependencies
	// failed/canceled/missing) when Reason == stalled.
	Unschedulable []string `json:"unschedulable,omitempty"`
	Alerts        int      `json:"alerts"`
	DurationMS    int64    `json:"duration_ms"`
}

type slot struct {
	id          string
	ticket      string
	title       string
	started     time.Time
	timeout     time.Duration
	attempts    int
	cancel      context.CancelFunc
	reason      string
	killReason  string
	slowAlerted bool
}

type workerDone struct {
	slot      *slot
	id        string
	exit      int
	err       error
	canceled  bool
	timedOut  bool
	base      string
	head      string
	branch    string
	workspace string
	log       string
	alerts    []pendingAlert // escapes detected by execute; emitted in complete
}

// Router: all mutable state (slots, alerts, summary) belongs to the Run
// goroutine; workers only send workerDone over a channel.
type Router struct {
	opts     Options
	cfg      config.RouterConfig
	workers  int
	strategy ticket.Strategy
	argv     []string
	store    *ticket.Store
	wsm      *workspace.Manager
	alerts   *alertLog
	now      func() time.Time

	ttl       time.Duration
	perTicket time.Duration
	global    time.Duration
	tick      time.Duration

	slots        []*slot
	done         chan workerDone
	startedAt    time.Time
	stopReason   string
	round1       bool
	queueGrowing bool
	lastState    []byte
	boardFails   int
	maxBoardFail int
	escapedMu    sync.Mutex
	escapedTkts  map[string]bool   // tickets marked by an escape (consumed in execute/complete)
	active       map[string]string // ticket -> worker with a running agent (escape suspects)
	summary      Summary
	identOnce    sync.Once
	ident        []string // -c user.name/user.email for commits (see identityArgs)
}

// New validates options without touching tickets.
func New(opts *Options) (*Router, error) {
	if opts.Root == "" {
		return nil, errors.New("raiz do repositório obrigatória")
	}
	cfg := opts.Config
	if cfg.MaxWorkers == 0 {
		cfg = config.DefaultRouterConfig()
	}
	if opts.Workers > 0 {
		cfg.MaxWorkers = opts.Workers
	}
	if opts.Strategy != "" {
		cfg.Strategy = opts.Strategy
	}
	if len(opts.AgentCmd) > 0 {
		cfg.AgentCmd = opts.AgentCmd
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	strategy, err := ticket.ParseStrategy(cfg.Strategy)
	if err != nil {
		return nil, err
	}
	r := &Router{
		opts: *opts, cfg: cfg, workers: cfg.MaxWorkers, strategy: strategy,
		argv: cfg.AgentCmd, now: opts.Now, tick: opts.Tick, maxBoardFail: maxBoardFailures,
	}
	if r.now == nil {
		r.now = time.Now
	}
	if r.tick <= 0 {
		r.tick = time.Second
	}
	r.ttl, _ = time.ParseDuration(cfg.DirectDelivery.SubscriptionTTL)
	r.perTicket, _ = time.ParseDuration(cfg.Timeouts.PerTicket)
	r.global, _ = time.ParseDuration(cfg.Timeouts.Global)
	r.store = ticket.Open(opts.Root)
	r.store.SetClock(r.now)
	r.alerts = &alertLog{path: AlertsPath(opts.Root)}
	for i := 1; i <= r.workers; i++ {
		r.slots = append(r.slots, &slot{id: fmt.Sprintf("w%d", i)})
	}
	return r, nil
}

func (r *Router) Config() config.RouterConfig { return r.cfg }

func (r *Router) logf(format string, a ...any) {
	if r.opts.Out != nil {
		fmt.Fprintf(r.opts.Out, format+"\n", a...)
	}
}

func (r *Router) slotByID(id string) *slot {
	for _, s := range r.slots {
		if s.id == id {
			return s
		}
	}
	return nil
}

func (r *Router) running() int {
	n := 0
	for _, s := range r.slots {
		if s.ticket != "" {
			n++
		}
	}
	return n
}

// Run loops until there is no work, drain, --once, the global timeout, or ctx
// cancellation (which requeues running tickets).
func (r *Router) Run(ctx context.Context) (*Summary, error) {
	if err := r.preflight(); err != nil {
		return nil, err
	}
	// atomic singleton: the flock lives with the process; state.json pid is informational only.
	release, err := acquireLock(LockPath(r.opts.Root))
	if err != nil {
		return nil, err
	}
	defer release()
	if err := r.initRuntime(); err != nil {
		return nil, err
	}

	r.done = make(chan workerDone, r.workers)
	r.startedAt = r.now()
	// Kill requests from previous runs (or written with no active router) are
	// stale: they must not kill the first worker of this run.
	clearControl(r.opts.Root)
	// Linux: descendants that escape the group (setsid) are reparented to us and
	// killed by procs.Sweep. Other platforms: process group only.
	releaseSubreaper, haveSubreaper := sandbox.AcquireSubreaper()
	defer func() {
		if haveSubreaper {
			r.sweepTick() // last sweep before giving up the subreaper role
		}
		releaseSubreaper()
	}()
	if err := r.recover(); err != nil {
		return nil, err
	}
	r.logf("router iniciado: %d workers, estratégia %s", r.workers, r.strategy)

	ticker := time.NewTicker(r.tick)
	defer ticker.Stop()

	for {
		stop, err := r.step()
		if err != nil {
			return r.finish(), err
		}
		if stop || r.waitEvent(ctx, ticker) {
			break
		}
	}
	return r.finish(), nil
}

// preflight checks what must be in place before touching any state.
func (r *Router) preflight() error {
	if len(r.argv) == 0 {
		return errors.New("agent_cmd não configurado (use router.agent_cmd em .toolbelt/config.yaml ou --agent-cmd)")
	}
	// fail-closed: without Landlock and without explicit opt-in, nothing is touched.
	if _, err := sandbox.New(sandbox.Config{WorkDir: r.opts.Root, AllowUnsandboxed: r.opts.AllowUnsandboxed}); err != nil {
		return err
	}
	if _, err := gitOut(r.opts.Root, "rev-parse", "--verify", "HEAD"); err != nil {
		return fmt.Errorf("repositório git sem commit (HEAD): %w", err)
	}
	r.identityArgs() // identity read once, with normal config, before the hardened git
	return os.MkdirAll(Dir(r.opts.Root), 0o755)
}

func (r *Router) initRuntime() error {
	wsm, err := newWorkspaceManager(r.opts.Root)
	if err != nil {
		return err
	}
	r.wsm = wsm
	for _, d := range []string{LogsDir(r.opts.Root), ControlDir(r.opts.Root)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// step returns err != nil to stop the router with a failure (board unreadable
// for too long).
func (r *Router) step() (stop bool, err error) {
	b, err := ticket.LoadBoard(r.store)
	if err != nil {
		return r.boardFailure(err)
	}
	r.boardFails = 0
	r.schedule(b)
	if reason, fin := r.finished(b); fin {
		r.stopReason = reason
		return true, nil
	}
	r.writeState(b, StateRunning)
	return false, nil
}

// boardFailure handles transiently invalid YAML (manual edit in progress):
// keeps workers running and retries on the next tick; gives up only after
// maxBoardFailures consecutive failures.
func (r *Router) boardFailure(err error) (stop bool, ferr error) {
	r.boardFails++
	if r.boardFails == 1 {
		r.alert(AlertBoardUnreadable, SevWarn, "", "",
			fmt.Sprintf("board ilegível (%v); workers mantidos, nova tentativa no próximo tick", err), ActionNone)
	}
	if r.boardFails >= r.maxBoardFail {
		r.shutdown(ReasonStalled)
		return true, err
	}
	return false, nil
}

// waitEvent blocks on a worker finishing, the watchdog tick or ctx
// cancellation. Returns true when the loop should stop.
func (r *Router) waitEvent(ctx context.Context, ticker *time.Ticker) bool {
	select {
	case ev := <-r.done:
		r.complete(&ev)
	case <-ticker.C:
		if r.watchdogTick() {
			r.shutdown(ReasonGlobalTimeout)
			return true
		}
	case <-ctx.Done():
		r.shutdown(ReasonInterrupted)
		return true
	}
	return false
}

func (r *Router) finish() *Summary {
	if r.stopReason == "" {
		r.stopReason = ReasonCompleted
	}
	r.summary.Reason = r.stopReason
	r.summary.Alerts = r.alerts.count
	r.summary.DurationMS = r.now().Sub(r.startedAt).Milliseconds()
	if b, err := ticket.LoadBoard(r.store); err == nil {
		r.writeState(b, StateStopped)
	}
	r.logf("router encerrado (%s)", r.summary.Reason)
	s := r.summary
	return &s
}

// recover returns running tickets orphaned by a previous router to pending and
// clears old subscriptions (direct delivery only holds within one run).
func (r *Router) recover() error {
	list, err := r.store.List(ticket.Filter{})
	if err != nil {
		return err
	}
	for i := range list {
		t := &list[i]
		switch {
		case t.Status == ticket.StatusRunning:
			if _, err := r.store.Update(t.ID, func(x *ticket.Ticket) error {
				x.Status = ticket.StatusPending
				x.Worker = ""
				x.Subscription = nil
				return nil
			}); err != nil {
				return err
			}
			r.logf("%s estava running sem router ativo; devolvido para pending", t.ID)
		case t.Status == ticket.StatusPending && t.Subscription != nil:
			if _, err := r.store.Update(t.ID, func(x *ticket.Ticket) error {
				x.Subscription = nil
				return nil
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Router) finished(b *ticket.Board) (string, bool) {
	if r.running() > 0 {
		return "", false
	}
	if r.opts.Once && r.round1 {
		return ReasonOnce, true
	}
	if b.Control.Draining {
		return ReasonDrained, true
	}
	cands := b.Candidates(r.strategy)
	if len(cands) == 0 {
		// Nothing ready or running: remaining pending tickets will never run (they
		// depend on failed/canceled/missing) => stalled, not "completed".
		var stuck []string
		for i := range b.Tickets {
			if b.Tickets[i].Status == ticket.StatusPending {
				stuck = append(stuck, b.Tickets[i].ID)
			}
		}
		if len(stuck) > 0 {
			r.summary.Unschedulable = stuck
			return ReasonStalled, true
		}
		return ReasonCompleted, true
	}
	if b.Control.Paused {
		return "", false // wait for resume
	}
	// candidates exist, nothing is running and schedule assigned nothing: no progress is possible
	return ReasonStalled, true
}

// subLive: a subscription is valid when live and for a worker that exists in this router.
func (r *Router) subLive(t *ticket.Ticket, now time.Time) (string, bool) {
	if !r.cfg.DirectDelivery.Enabled || !t.Subscription.Live(now) || r.slotByID(t.Subscription.Worker) == nil {
		return "", false
	}
	return t.Subscription.Worker, true
}

func (r *Router) schedule(b *ticket.Board) {
	if r.opts.Once && r.round1 {
		return
	}
	r.round1 = true
	if !b.Assignable() {
		return
	}
	now := r.now()
	cands := b.Candidates(r.strategy)
	byID := make(map[string]*ticket.Ticket, len(b.Tickets))
	for i := range b.Tickets {
		byID[b.Tickets[i].ID] = &b.Tickets[i]
	}
	taken := r.heldTickets()
	head := ""
	for _, s := range r.slots {
		if s.ticket != "" {
			continue
		}
		pick, via := r.pickCandidate(s, cands, taken, now)
		if pick == nil {
			continue
		}
		taken[pick.ID] = true
		if head == "" {
			head, _ = gitOut(r.opts.Root, "rev-parse", "HEAD")
		}
		r.start(s, pick, via, r.baseFor(s, pick, head, byID))
	}
}

// heldTickets returns tickets still held by some slot. A ticket canceled and
// returned to pending while its old agent is still shutting down must not be
// assigned again (createWorkspace would delete the exiting agent's worktree).
func (r *Router) heldTickets() map[string]bool {
	taken := map[string]bool{}
	for _, s := range r.slots {
		if s.ticket != "" {
			taken[s.ticket] = true
		}
	}
	return taken
}

// pickCandidate picks slot s's ticket: first one reserved for s (direct
// delivery); otherwise the first one not reserved for another live worker.
func (r *Router) pickCandidate(s *slot, cands []ticket.Ticket, taken map[string]bool, now time.Time) (pick *ticket.Ticket, via string) {
	for i := range cands {
		t := &cands[i]
		if taken[t.ID] {
			continue
		}
		if w, ok := r.subLive(t, now); ok && w == s.id {
			return t, ViaDirect
		}
	}
	for i := range cands {
		t := &cands[i]
		if taken[t.ID] {
			continue
		}
		if w, ok := r.subLive(t, now); ok && w != s.id {
			continue // reserved for another live worker
		}
		return t, ViaBoard
	}
	return nil, ViaBoard
}

// baseFor picks the workspace base commit. Owner's decision: with ONE
// dependency, the worktree starts from its HeadCommit, even without direct
// delivery (merging is not a prerequisite). With >=2 dependencies there is no
// single base: use HEAD and warn if any is unmerged (multi_dep_base).
func (r *Router) baseFor(s *slot, pick *ticket.Ticket, head string, byID map[string]*ticket.Ticket) string {
	base := head
	switch {
	case len(pick.Dependencies) == 1:
		if dep, ok := byID[pick.Dependencies[0]]; ok && dep.Result != nil && dep.Result.HeadCommit != "" {
			base = dep.Result.HeadCommit
		}
	case len(pick.Dependencies) >= 2:
		var unmerged []string
		for _, d := range pick.Dependencies {
			if dep, ok := byID[d]; !ok || dep.MergedAt == nil {
				unmerged = append(unmerged, d)
			}
		}
		if len(unmerged) > 0 {
			r.alert(AlertMultiDepBase, SevWarn, s.id, pick.ID,
				fmt.Sprintf("%s tem %d dependências e base em HEAD; dependências sem merge: %s (rode `tb merge` nelas antes)",
					pick.ID, len(pick.Dependencies), strings.Join(unmerged, ", ")),
				ActionNone)
		}
	}
	return base
}

func (r *Router) start(s *slot, t *ticket.Ticket, via, base string) {
	now := r.now()
	var attempts int
	_, err := r.store.Update(t.ID, func(x *ticket.Ticket) error {
		if x.Status != ticket.StatusPending {
			return fmt.Errorf("%s não está mais pending (%s)", x.ID, x.Status)
		}
		x.Status = ticket.StatusRunning
		x.Worker = s.id
		x.Attempts++
		x.Subscription = nil
		started := now.UTC()
		x.Result = &ticket.Result{BaseCommit: base, Branch: "tb-workspace/" + x.ID, StartedAt: &started}
		attempts = x.Attempts
		return nil
	})
	if err != nil {
		r.logf("não atribuí %s: %v", t.ID, err)
		return
	}
	timeout := t.TimeoutDuration()
	if timeout <= 0 {
		timeout = r.perTicket
	}
	wctx, cancel := context.WithCancel(context.Background())
	*s = slot{id: s.id, ticket: t.ID, title: t.Title, started: now, timeout: timeout, attempts: attempts, cancel: cancel}
	r.summary.Assignments = append(r.summary.Assignments, Assignment{Ticket: t.ID, Worker: s.id, Via: via, Base: base, Attempt: attempts})
	r.logf("[%s] %s iniciado (%s, base %s, tentativa %d)", s.id, t.ID, via, short(base), attempts)
	tk := *t // copy: the goroutine does not share the board's slice
	wid := s.id
	go func() { r.done <- r.execute(wctx, s, wid, &tk, base) }()
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// errNotMine signals the ticket is no longer running on this worker (e.g.
// canceled via `tb ticket cancel`); the result is dropped without a transition.
var errNotMine = errors.New("ticket não está mais running neste worker")

// updateMine applies fn only if the ticket is still running and assigned to worker.
func (r *Router) updateMine(id, worker string, fn func(*ticket.Ticket)) error {
	_, err := r.store.Update(id, func(x *ticket.Ticket) error {
		if x.Status != ticket.StatusRunning || x.Worker != worker {
			return errNotMine
		}
		fn(x)
		return nil
	})
	return err
}

// completion snapshots the slot at the moment the worker finished.
type completion struct {
	reason, killReason string
	attempts           int
	timeout            time.Duration
	worker             string
}

func (r *Router) complete(ev *workerDone) {
	s := ev.slot
	c := completion{reason: s.reason, killReason: s.killReason, attempts: s.attempts, timeout: s.timeout, worker: s.id}
	now := r.now().UTC()
	started := s.started.UTC()
	*s = slot{id: s.id}
	// The mark is ALWAYS consumed (even on drop/requeue), otherwise it would poison
	// the next attempt of the same ticket. Escape alerts are real even if the
	// result is dropped.
	escapedMark := r.takeEscaped(ev.id)
	for i := range ev.alerts {
		r.emit(&ev.alerts[i])
	}

	res := &ticket.Result{
		Branch: ev.branch, BaseCommit: ev.base, HeadCommit: ev.head, Workspace: ev.workspace,
		ExitCode: ev.exit, StartedAt: &started, FinishedAt: &now, Log: ev.log,
	}
	// Ticket canceled/changed while running: no transition.
	if cur, err := r.store.Get(ev.id); err == nil && (c.reason == reasonCanceled || cur.Status != ticket.StatusRunning || cur.Worker != c.worker) {
		r.discard(c.worker, ev.id)
		r.clearSubscriptions(c.worker)
		return
	}

	// Successfully completed work counts even if a late reason
	// (stuck/kill/shutdown) was set after it finished: honor the reason only when
	// the agent was actually canceled or exited != 0.
	if escapedMark && ev.err == nil {
		// escape attributed to this ticket by a tick (or by another worker) after
		// execute's last sweep: fail anyway
		ev.err = fmt.Errorf("%w: processo escapou do grupo do agente durante a execução", errAgentEscaped)
	}
	finishedOK := ev.err == nil && ev.exit == 0 && !ev.canceled && !ev.timedOut
	switch {
	case finishedOK:
		r.completeOK(ev, &c, res)
	case c.reason == reasonShutdown:
		r.completeRequeue(ev, &c)
	default:
		r.completeFailure(ev, &c, res)
	}
}

func (r *Router) discard(worker, id string) {
	r.logf("[%s] %s não está mais running neste worker (cancelado?); resultado descartado", worker, id)
}

func (r *Router) completeOK(ev *workerDone, c *completion, res *ticket.Result) {
	err := r.updateMine(ev.id, c.worker, func(x *ticket.Ticket) {
		x.Status = ticket.StatusDone
		x.Result = res
	})
	if errors.Is(err, errNotMine) {
		r.discard(c.worker, ev.id)
		return
	}
	if err != nil {
		r.logf("erro ao concluir %s: %v", ev.id, err)
		return
	}
	if c.reason != "" {
		r.logf("[%s] %s terminou com sucesso antes do cancelamento (%s); mantido como concluído", c.worker, ev.id, c.reason)
	}
	r.summary.Done = append(r.summary.Done, ev.id)
	r.logf("[%s] %s concluído (%s)", c.worker, ev.id, short(ev.head))
	r.subscribeDependents(ev.id, c.worker)
}

// completeRequeue returns the ticket to pending (router shutdown).
func (r *Router) completeRequeue(ev *workerDone, c *completion) {
	err := r.updateMine(ev.id, c.worker, func(x *ticket.Ticket) {
		x.Status = ticket.StatusPending
		x.Worker = ""
		x.Subscription = nil
		x.Result = nil
		if x.Attempts > 0 {
			x.Attempts-- // a router interruption does not count as an attempt
		}
	})
	if errors.Is(err, errNotMine) {
		r.discard(c.worker, ev.id)
		return
	}
	r.summary.Requeued = append(r.summary.Requeued, ev.id)
	r.logf("[%s] %s devolvido para pending", c.worker, ev.id)
}

func failureError(ev *workerDone, c *completion) string {
	switch {
	case c.reason == reasonStuck:
		return fmt.Sprintf("timeout de %s excedido (worker_stuck)", c.timeout)
	case c.reason == reasonKilled:
		msg := "encerrado por tb watchdog kill"
		if c.killReason != "" {
			msg += ": " + c.killReason
		}
		return msg
	case ev.err != nil:
		return ev.err.Error()
	default:
		return fmt.Sprintf("agente saiu com código %d", ev.exit)
	}
}

// completeFailure either retries (back to pending) or marks failed.
func (r *Router) completeFailure(ev *workerDone, c *completion, res *ticket.Result) {
	res.Error = failureError(ev, c)
	action := r.failureAction(c.attempts)
	if errors.Is(ev.err, errWorkspaceIntegrity) {
		action = ActionEscalate // tampering is not transient: no retry
	}
	if c.reason == "" && !errors.Is(ev.err, errAgentEscaped) {
		sev := SevError
		if action == ActionEscalate {
			sev = SevCritical
		}
		r.alert(AlertWorkerCrash, sev, c.worker, ev.id, fmt.Sprintf("%s em %s falhou: %s", ev.id, c.worker, res.Error), action)
	}
	r.clearSubscriptions(c.worker)
	retry := action == ActionRetry
	err := r.updateMine(ev.id, c.worker, func(x *ticket.Ticket) {
		x.Result = res
		x.Subscription = nil
		if retry {
			x.Status = ticket.StatusPending
			x.Worker = ""
		} else {
			x.Status = ticket.StatusFailed
		}
	})
	if errors.Is(err, errNotMine) {
		r.discard(c.worker, ev.id)
		return
	}
	if retry {
		r.summary.Retried = append(r.summary.Retried, ev.id)
	} else {
		r.summary.Failed = append(r.summary.Failed, ev.id)
	}
}

func (r *Router) shutdown(reason string) {
	if r.stopReason == "" {
		r.stopReason = reason
	}
	for _, s := range r.slots {
		if s.ticket != "" {
			s.reason = reasonShutdown
			s.cancel()
		}
	}
	for r.running() > 0 {
		ev := <-r.done
		r.complete(&ev)
	}
}

// subscribeDependents reserves pending dependents (direct delivery) for the
// worker that just finished doneID, respecting the per-worker cap.
func (r *Router) subscribeDependents(doneID, worker string) {
	if !r.cfg.DirectDelivery.Enabled {
		return
	}
	list, err := r.store.List(ticket.Filter{})
	if err != nil {
		return
	}
	now := r.now()
	held := 0
	for i := range list {
		t := &list[i]
		if t.Status == ticket.StatusPending && t.Subscription.Live(now) && t.Subscription.Worker == worker {
			held++
		}
	}
	ticket.SortTickets(list, r.strategy, list)
	for i := range list {
		t := &list[i]
		if t.Status != ticket.StatusPending || !containsStr(t.Dependencies, doneID) {
			continue
		}
		if held >= r.cfg.DirectDelivery.MaxSubscriptionsPerWorker {
			return
		}
		if t.Subscription.Live(now) && t.Subscription.Worker != worker {
			continue // already reserved by another live worker
		}
		exp := now.Add(r.ttl).UTC()
		if _, err := r.store.Update(t.ID, func(x *ticket.Ticket) error {
			if x.Status != ticket.StatusPending {
				return fmt.Errorf("mudou de status")
			}
			x.Subscription = &ticket.Subscription{Worker: worker, DirectDelivery: true, ExpiresAt: exp}
			return nil
		}); err == nil {
			held++
		}
	}
}

func (r *Router) clearSubscriptions(worker string) {
	list, err := r.store.List(ticket.Filter{Statuses: []ticket.Status{ticket.StatusPending}})
	if err != nil {
		return
	}
	for i := range list {
		t := &list[i]
		if t.Subscription != nil && t.Subscription.Worker == worker {
			_, _ = r.store.Update(t.ID, func(x *ticket.Ticket) error {
				x.Subscription = nil
				return nil
			})
		}
	}
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// writeState only rewrites state.json when something changed.
func (r *Router) writeState(b *ticket.Board, status string) {
	now := r.now()
	st := &State{
		PID: os.Getpid(), StartedAt: r.startedAt.UTC(), Status: status, Strategy: string(r.strategy),
		Paused: b.Control.Paused, Draining: b.Control.Draining, Workers: []WorkerState{},
	}
	if status == StateStopped {
		t := now.UTC()
		st.StoppedAt = &t
	}
	for _, s := range r.slots {
		w := WorkerState{ID: s.id, Status: "idle", Subscriptions: []string{}}
		if s.ticket != "" {
			w.Status, w.Ticket = "working", s.ticket
			t := s.started.UTC()
			w.StartedAt = &t
		}
		for i := range b.Tickets {
			t := &b.Tickets[i]
			if t.Status == ticket.StatusPending && t.Subscription.Live(now) && t.Subscription.Worker == s.id {
				w.Subscriptions = append(w.Subscriptions, t.ID)
			}
		}
		st.Workers = append(st.Workers, w)
	}
	st.Queue = QueueState{
		Ready:   len(b.Candidates(r.strategy)),
		Blocked: len(b.Blocked()),
		Running: r.running(),
	}
	if !b.Control.Paused {
		r.checkQueue(st.Queue.Ready)
	}
	data, err := marshalState(st)
	if err != nil || bytes.Equal(data, r.lastState) {
		return
	}
	if _, err := writeState(r.opts.Root, st); err != nil {
		r.logf("aviso: state.json: %v", err)
		return
	}
	r.lastState = data
}

func marshalState(st *State) ([]byte, error) { return jsonIndent(st) }
