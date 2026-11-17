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

// Motivos de término do router (Summary.Reason).
const (
	ReasonCompleted     = "completed"
	ReasonOnce          = "once"
	ReasonDrained       = "drained"
	ReasonInterrupted   = "interrupted"
	ReasonGlobalTimeout = "global_timeout"
	ReasonStalled       = "stalled"
)

// maxBoardFailures: ticks consecutivos com o board ilegível antes de desistir.
const maxBoardFailures = 30

// Caminhos de atribuição.
const (
	ViaBoard  = "board"
	ViaDirect = "direct"
)

const (
	reasonStuck    = "stuck"
	reasonKilled   = "killed"
	reasonShutdown = "shutdown"
	// reasonCanceled: ticket deixou de estar running/deste worker (tb ticket cancel).
	reasonCanceled = "cancel\u006ced"
)

// Options configura um Router.
type Options struct {
	Root             string              // raiz do repositório (onde fica .toolbelt)
	Config           config.RouterConfig // zero value = padrões
	Workers          int                 // >0 sobrepõe Config.MaxWorkers
	Strategy         string              // não vazio sobrepõe Config.Strategy
	AgentCmd         []string            // não vazio sobrepõe Config.AgentCmd
	Once             bool                // uma rodada de agendamento e sai
	AllowUnsandboxed bool                // opt-in explícito (sem Landlock)
	EnvOptions       execenv.Options     // allow/pass de variáveis do filho
	Environ          []string            // ambiente base (nil = os.Environ())
	Now              func() time.Time    // relógio injetável (nil = time.Now)
	Tick             time.Duration       // intervalo do watchdog/loop (0 = 1s)
	Out              io.Writer           // avanço humano (nil = descarta)
}

// Assignment registra uma atribuição ticket -> worker.
type Assignment struct {
	Ticket  string `json:"ticket"`
	Worker  string `json:"worker"`
	Via     string `json:"via"` // board | direct
	Base    string `json:"base"`
	Attempt int    `json:"attempt"`
}

// Summary é o resultado de Run.
type Summary struct {
	Reason      string       `json:"reason"`
	Assignments []Assignment `json:"assignments"`
	Done        []string     `json:"done"`
	Failed      []string     `json:"failed"`
	Retried     []string     `json:"retried"`
	Requeued    []string     `json:"requeued"`
	// Unschedulable lista tickets pending que nunca rodarão (dependências
	// failed/canceladas/inexistentes) quando Reason == stalled.
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
	alerts    []pendingAlert // escapes detectados pelo execute; emitidos em complete
}

// Router é o scheduler. Todo estado mutável (slots, alertas, sumário) pertence
// à goroutine de Run; workers só devolvem workerDone por canal.
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
	escapedTkts  map[string]bool   // tickets marcados por um escape (consumido em execute/complete)
	active       map[string]string // ticket -> worker com agente em execução (suspeitos de escape)
	summary      Summary
	identOnce    sync.Once
	ident        []string // -c user.name/user.email do commit (ver identityArgs)
}

// New valida as opções e prepara o router (sem tocar tickets).
func New(opts *Options) (*Router, error) {
	o := *opts
	if o.Root == "" {
		return nil, errors.New("raiz do repositório obrigatória")
	}
	cfg := o.Config
	if cfg.MaxWorkers == 0 {
		cfg = config.DefaultRouterConfig()
	}
	if o.Workers > 0 {
		cfg.MaxWorkers = o.Workers
	}
	if o.Strategy != "" {
		cfg.Strategy = o.Strategy
	}
	if len(o.AgentCmd) > 0 {
		cfg.AgentCmd = o.AgentCmd
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	strategy, err := ticket.ParseStrategy(cfg.Strategy)
	if err != nil {
		return nil, err
	}
	r := &Router{
		opts: o, cfg: cfg, workers: cfg.MaxWorkers, strategy: strategy,
		argv: cfg.AgentCmd, now: o.Now, tick: o.Tick, maxBoardFail: maxBoardFailures,
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
	r.store = ticket.Open(o.Root)
	r.store.SetClock(r.now)
	r.alerts = &alertLog{path: AlertsPath(o.Root)}
	for i := 1; i <= r.workers; i++ {
		r.slots = append(r.slots, &slot{id: fmt.Sprintf("w%d", i)})
	}
	return r, nil
}

// Config devolve a configuração efetiva.
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

func (r *Router) prepareRun() (release func(), err error) {
	if len(r.argv) == 0 {
		return nil, errors.New("agent_cmd não configurado (use router.agent_cmd em .toolbelt/config.yaml ou --agent-cmd)")
	}
	if _, err := sandbox.New(sandbox.Config{WorkDir: r.opts.Root, AllowUnsandboxed: r.opts.AllowUnsandboxed}); err != nil {
		return nil, err
	}
	if _, err := gitOut(r.opts.Root, "rev-parse", "--verify", "HEAD"); err != nil {
		return nil, fmt.Errorf("repositório git sem commit (HEAD): %w", err)
	}
	r.identityArgs()
	if err := os.MkdirAll(Dir(r.opts.Root), 0o755); err != nil {
		return nil, err
	}
	release, err = acquireLock(LockPath(r.opts.Root))
	if err != nil {
		return nil, err
	}
	wsm, err := newWorkspaceManager(r.opts.Root)
	if err != nil {
		release()
		return nil, err
	}
	r.wsm = wsm
	for _, d := range []string{LogsDir(r.opts.Root), ControlDir(r.opts.Root)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			release()
			return nil, err
		}
	}
	return release, nil
}

func (r *Router) tickBoard() (stop bool, err error) {
	b, err := ticket.LoadBoard(r.store)
	if err != nil {
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
	r.boardFails = 0
	r.schedule(b)
	if reason, fin := r.finished(b); fin {
		r.stopReason = reason
		return true, nil
	}
	r.writeState(b, StateRunning)
	return false, nil
}

// Run executa o loop até não haver trabalho, drain, --once, timeout global ou
// cancelamento do ctx (que requeue os tickets em execução).
func (r *Router) Run(ctx context.Context) (*Summary, error) {
	release, err := r.prepareRun()
	if err != nil {
		return nil, err
	}
	defer release()

	r.done = make(chan workerDone, r.workers)
	r.startedAt = r.now()
	clearControl(r.opts.Root)
	releaseSubreaper, haveSubreaper := sandbox.AcquireSubreaper()
	defer func() {
		if haveSubreaper {
			r.sweepTick()
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
		stop, err := r.tickBoard()
		if err != nil {
			return r.finish(), err
		}
		if stop {
			break
		}
		select {
		case ev := <-r.done:
			r.complete(&ev)
		case <-ticker.C:
			if r.watchdogTick() {
				r.shutdown(ReasonGlobalTimeout)
				return r.finish(), nil
			}
		case <-ctx.Done():
			r.shutdown(ReasonInterrupted)
			return r.finish(), nil
		}
	}
	return r.finish(), nil
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

// recover devolve a pending tickets running órfãos de um router anterior e
// limpa assinaturas antigas (entrega direta vale só dentro de uma execução).
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

func (r *Router) stalledPending(b *ticket.Board) (string, bool) {
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

// finished decide se o loop termina (após schedule).
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
		return r.stalledPending(b)
	}
	if b.Control.Paused {
		return "", false
	}
	return ReasonStalled, true
}

// subLive: assinatura válida = viva e para um worker que existe neste router.
func (r *Router) subLive(t *ticket.Ticket, now time.Time) (string, bool) {
	if !r.cfg.DirectDelivery.Enabled || !t.Subscription.Live(now) || r.slotByID(t.Subscription.Worker) == nil {
		return "", false
	}
	return t.Subscription.Worker, true
}

func (r *Router) pickForSlot(s *slot, cands []ticket.Ticket, taken map[string]bool, now time.Time) (pick *ticket.Ticket, via string) {
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
			continue
		}
		return t, ViaBoard
	}
	return nil, ViaBoard
}

func (r *Router) scheduleBase(s *slot, pick *ticket.Ticket, byID map[string]ticket.Ticket, head string) string {
	base := head
	if len(pick.Dependencies) == 1 {
		if dep, ok := byID[pick.Dependencies[0]]; ok && dep.Result != nil && dep.Result.HeadCommit != "" {
			base = dep.Result.HeadCommit
		}
		return base
	}
	if len(pick.Dependencies) < 2 {
		return base
	}
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
	return base
}

// schedule atribui tickets a slots ociosos.
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
	taken := map[string]bool{}
	byID := map[string]ticket.Ticket{}
	for i := range b.Tickets {
		t := &b.Tickets[i]
		byID[t.ID] = *t
	}
	head := ""
	for _, s := range r.slots {
		if s.ticket != "" {
			taken[s.ticket] = true
		}
	}
	for _, s := range r.slots {
		if s.ticket != "" {
			continue
		}
		pick, via := r.pickForSlot(s, cands, taken, now)
		if pick == nil {
			continue
		}
		taken[pick.ID] = true
		if head == "" {
			head, _ = gitOut(r.opts.Root, "rev-parse", "HEAD")
		}
		base := r.scheduleBase(s, pick, byID, head)
		r.start(s, pick, via, base)
	}
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
	tk := *t
	wid := s.id
	go func() { r.done <- r.execute(wctx, s, wid, &tk, base) }()
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// errNotMine sinaliza que o ticket não está mais running neste worker (ex.:
// cancelado por `tb ticket cancel`); o resultado é descartado sem transição.
var errNotMine = errors.New("ticket não está mais running neste worker")

// updateMine aplica fn só se o ticket ainda está running e atribuído a worker.
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

// complete aplica o resultado de um worker ao ticket.
func (r *Router) completeDone() {
	ev := <-r.done
	r.complete(&ev)
}

func (r *Router) complete(ev *workerDone) {
	s := ev.slot
	reason, killReason, attempts, timeout, worker := s.reason, s.killReason, s.attempts, s.timeout, s.id
	now := r.now().UTC()
	started := s.started.UTC()
	*s = slot{id: s.id}
	escapedMark := r.takeEscaped(ev.id)
	for i := range ev.alerts {
		r.emit(&ev.alerts[i])
	}

	res := &ticket.Result{
		Branch: ev.branch, BaseCommit: ev.base, HeadCommit: ev.head, Workspace: ev.workspace,
		ExitCode: ev.exit, StartedAt: &started, FinishedAt: &now, Log: ev.log,
	}
	discard := func() {
		r.logf("[%s] %s não está mais running neste worker (cancelado?); resultado descartado", worker, ev.id)
	}
	if cur, err := r.store.Get(ev.id); err == nil && (reason == reasonCanceled || cur.Status != ticket.StatusRunning || cur.Worker != worker) {
		discard()
		r.clearSubscriptions(worker)
		return
	}

	if escapedMark && ev.err == nil {
		ev.err = fmt.Errorf("%w: processo escapou do grupo do agente durante a execução", errAgentEscaped)
	}
	finishedOK := ev.err == nil && ev.exit == 0 && !ev.canceled && !ev.timedOut
	switch {
	case finishedOK:
		r.completeSuccess(ev, worker, reason, res, discard)
		return
	case reason == reasonShutdown:
		r.completeShutdown(ev, worker, discard)
		return
	}
	r.completeFailure(ev, worker, reason, killReason, attempts, timeout, res, discard)
}

func (r *Router) completeSuccess(ev *workerDone, worker, reason string, res *ticket.Result, discard func()) {
	err := r.updateMine(ev.id, worker, func(x *ticket.Ticket) {
		x.Status = ticket.StatusDone
		x.Result = res
	})
	if errors.Is(err, errNotMine) {
		discard()
		return
	}
	if err != nil {
		r.logf("erro ao concluir %s: %v", ev.id, err)
		return
	}
	if reason != "" {
		r.logf("[%s] %s terminou com sucesso antes do cancelamento (%s); mantido como concluído", worker, ev.id, reason)
	}
	r.summary.Done = append(r.summary.Done, ev.id)
	r.logf("[%s] %s concluído (%s)", worker, ev.id, short(ev.head))
	r.subscribeDependents(ev.id, worker)
}

func (r *Router) completeShutdown(ev *workerDone, worker string, discard func()) {
	err := r.updateMine(ev.id, worker, func(x *ticket.Ticket) {
		x.Status = ticket.StatusPending
		x.Worker = ""
		x.Subscription = nil
		x.Result = nil
		if x.Attempts > 0 {
			x.Attempts--
		}
	})
	if errors.Is(err, errNotMine) {
		discard()
		return
	}
	r.summary.Requeued = append(r.summary.Requeued, ev.id)
	r.logf("[%s] %s devolvido para pending", worker, ev.id)
}

func (r *Router) failureError(ev *workerDone, reason, killReason string, timeout time.Duration) string {
	switch {
	case reason == reasonStuck:
		return fmt.Sprintf("timeout de %s excedido (worker_stuck)", timeout)
	case reason == reasonKilled:
		msg := "encerrado por tb watchdog kill"
		if killReason != "" {
			msg += ": " + killReason
		}
		return msg
	case ev.err != nil:
		return ev.err.Error()
	default:
		return fmt.Sprintf("agente saiu com código %d", ev.exit)
	}
}

func (r *Router) completeFailure(ev *workerDone, worker, reason, killReason string, attempts int, timeout time.Duration, res *ticket.Result, discard func()) {
	res.Error = r.failureError(ev, reason, killReason, timeout)
	action := r.failureAction(attempts)
	if errors.Is(ev.err, errWorkspaceIntegrity) {
		action = ActionEscalate
	}
	if reason == "" && !errors.Is(ev.err, errAgentEscaped) {
		sev := SevError
		if action == ActionEscalate {
			sev = SevCritical
		}
		r.alert(AlertWorkerCrash, sev, worker, ev.id, fmt.Sprintf("%s em %s falhou: %s", ev.id, worker, res.Error), action)
	}
	r.clearSubscriptions(worker)
	retry := action == ActionRetry
	err := r.updateMine(ev.id, worker, func(x *ticket.Ticket) {
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
		discard()
		return
	}
	if retry {
		r.summary.Retried = append(r.summary.Retried, ev.id)
	} else {
		r.summary.Failed = append(r.summary.Failed, ev.id)
	}
}

// shutdown cancela workers, espera-os e devolve os tickets a pending.
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
		r.completeDone()
	}
}

// subscribeDependents reserva, para o worker que acabou de concluir doneID, os
// dependentes pending (entrega direta), respeitando o teto por worker.
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
			continue
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

// clearSubscriptions devolve ao board os tickets reservados para worker.
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

// writeState persiste state.json (só reescreve quando algo mudou).
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
