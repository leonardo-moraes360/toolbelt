package router

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
	"github.com/leonardo-moraes360/toolbelt/internal/busrun"
	"github.com/leonardo-moraes360/toolbelt/internal/mem"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

// Bus slots (docs/bus-router.md, B4b): tickets handed to agents on a bus
// channel. The worker's harness claims and completes; the router validates
// the claim, reads the result from the claims table and commits the
// worktree itself.

const (
	AlertBusUnclaimed      = "bus_unclaimed"
	AlertBusClaimRejected  = "bus_claim_rejected"
	AlertBusLeaseLost      = "bus_lease_lost"
	AlertBusVerdictFail    = "bus_verdict_fail"
	AlertBusChannelExpired = "bus_channel_expired"
	AlertBusWriteFailed    = "bus_write_failed"
)

var (
	errBusUnclaimed = errors.New("no valid bus claim within the claim timeout (unclaimed)")
	errBusLeaseLost = errors.New("the bus agent lost its lease (bus_lease_lost)")
	errBusVerdict   = errors.New("the bus agent reported FAIL (bus_verdict_fail)")
	errBusResult    = errors.New("the bus result did not come from the claim owner")
)

// Bus attempt phases, shown in state.json.
const (
	phaseAssigned   = "assigned"
	phaseClaimed    = "claimed"
	phaseCommitting = "committing"
)

const maxSummaryBytes = 500

// busLink is the router's bus session.
type busLink struct {
	store        *bus.Store
	member       bus.Member
	to           string
	claimTimeout time.Duration
	expired      bool // bus_channel_expired already alerted
}

// busAttempt is shared between the Run goroutine (watchdog, state) and the
// attempt goroutine, behind its mutex.
type busAttempt struct {
	mu        sync.Mutex
	thread    string
	agent     string
	claimedAt time.Time
	phase     string
}

func (a *busAttempt) snapshot() (thread, agent, phase string, claimedAt time.Time) {
	if a == nil {
		return "", "", "", time.Time{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.thread, a.agent, a.phase, a.claimedAt
}

func (a *busAttempt) set(fn func(*busAttempt)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	fn(a)
}

func isBusSlot(id string) bool { return strings.HasPrefix(id, "b") }

// openBus loads the router's session and checks the channel (preflight and
// --check): role router, channel active, a single router-role member.
func (r *Router) openBus() error {
	bc := r.cfg.Bus
	if !bc.Enabled() {
		return nil
	}
	if r.bus != nil {
		return nil
	}
	s, err := bus.Open(r.opts.Root)
	if err != nil {
		return err
	}
	m, err := s.LoadMember(bc.Channel, bc.As)
	if err != nil {
		_ = s.Close()
		return fmt.Errorf("router.bus: session %s/%s: %w (join it with tb bus join --role router)", bc.Channel, bc.As, err)
	}
	if err := checkBusChannel(s, &m); err != nil {
		_ = s.Close()
		return err
	}
	ct, _ := time.ParseDuration(bc.ClaimTimeout)
	r.bus = &busLink{store: s, member: m, to: bc.To, claimTimeout: ct}
	return nil
}

func checkBusChannel(s *bus.Store, m *bus.Member) error {
	if m.Role != bus.RoleRouter {
		return fmt.Errorf("router.bus: %s has role %s, not router", m.ID(), m.Role)
	}
	if _, err := s.ActiveChannel(m.Channel); err != nil {
		return fmt.Errorf("router.bus: %w", err)
	}
	agents, err := s.Agents(m.Channel)
	if err != nil {
		return err
	}
	routers := 0
	for i := range agents {
		if agents[i].Role == bus.RoleRouter {
			routers++
		}
	}
	if routers > 1 {
		return fmt.Errorf("router.bus: channel %s has %d router-role members; two routers would fight over the same tickets", m.Channel, routers)
	}
	return nil
}

// CheckBus is tb router run --check: the bus session and channel, without
// running anything.
func (r *Router) CheckBus() error {
	if !r.cfg.Bus.Enabled() {
		return errors.New("router.bus.channel is not configured (set it in the global config or -c)")
	}
	defer r.closeBus()
	if err := r.openBus(); err != nil {
		return err
	}
	if to := r.cfg.Bus.To; strings.HasPrefix(to, "agent:") {
		agents, err := r.bus.store.Agents(r.bus.member.Channel)
		if err != nil {
			return err
		}
		for i := range agents {
			if bus.AgentID(agents[i].Name) == to {
				return nil
			}
		}
		return fmt.Errorf("router.bus.to: %s is not a member of %s", to, r.bus.member.Channel)
	}
	return nil
}

func (r *Router) closeBus() {
	if r.bus != nil {
		_ = r.bus.store.Close()
		r.bus = nil
	}
}

// busOpen reports whether bus slots may take work now; an expired channel
// is alerted once.
func (r *Router) busOpen() bool {
	if r.bus == nil {
		return false
	}
	_, err := r.bus.store.ActiveChannel(r.bus.member.Channel)
	if errors.Is(err, bus.ErrChannelExpired) {
		if !r.bus.expired {
			r.bus.expired = true
			r.alert(AlertBusChannelExpired, SevWarn, "", "",
				fmt.Sprintf("bus channel %s expired; bus slots idle until tb bus channel extend", r.bus.member.Channel), ActionNone)
		}
		return false
	}
	if err == nil {
		r.bus.expired = false
	}
	return err == nil
}

// newThread is <ID>/<rand>.<attempt>: unique across attempts, runs and a
// recreated board.
func newThread(id string, attempt int) string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s/%s.%d", id, strings.ToLower(base32.StdEncoding.EncodeToString(b[:])[:6]), attempt)
}

// revokeStale closes the bus thread a previous run left open; false when
// it could not, so the ticket must not be assigned again yet.
func (r *Router) revokeStale(t *ticket.Ticket) bool {
	if t.BusThread == "" || r.bus == nil {
		return true
	}
	if _, err := r.bus.store.Revoke(r.bus.member, t.BusThread, "restart"); err != nil {
		r.logf("%s: stale bus thread %s not revoked yet: %v", t.ID, t.BusThread, err)
		return false
	}
	_, _ = r.store.Update(t.ID, func(x *ticket.Ticket) error {
		if x.BusThread == t.BusThread {
			x.BusThread = ""
		}
		return nil
	})
	return true
}

// busAttemptRun is the state of one attempt goroutine.
type busAttemptRun struct {
	r        *Router
	ev       *workerDone
	t        *ticket.Ticket
	a        *busAttempt
	worker   string
	ws       string
	attempt  int
	timeout  time.Duration
	assignAt time.Time
	deadline time.Time
	owner    string
	epoch    int64
}

// executeBus runs in the attempt goroutine and returns through workerDone,
// like execute.
func (r *Router) executeBus(ctx context.Context, s *slot, worker string, t *ticket.Ticket, base string, a *busAttempt, attempt int, timeout time.Duration) workerDone {
	ev := workerDone{slot: s, id: t.ID, base: base, branch: "tb-workspace/" + t.ID, exit: -1}
	ev.log = logRel(t.ID)
	ws, err := r.createWorkspace(t, base)
	if err != nil {
		ev.err = err
		return ev
	}
	ev.workspace = ws.Path
	thread, _, _, _ := a.snapshot()
	ev.busThread = thread
	run := &busAttemptRun{r: r, ev: &ev, t: t, a: a, worker: worker, ws: ws.Path, attempt: attempt, timeout: timeout}
	if !run.assign(thread) {
		return ev
	}
	run.loop(ctx)
	return ev
}

func (b *busAttemptRun) alert(kind, sev, msg string) {
	b.ev.alerts = append(b.ev.alerts, pendingAlert{kind: kind, severity: sev, worker: b.worker, ticket: b.t.ID, msg: msg, action: ActionNone})
}

// assign sends the router-task for thread; false ends the attempt.
func (b *busAttemptRun) assign(thread string) bool {
	r := b.r
	task := busrun.RouterTask{
		V: busrun.RouterTaskVersion, Ticket: b.t.ID, Thread: thread, Attempt: b.attempt, Title: b.t.Title,
		Workspace: b.ws, TicketFile: r.ticketFile(b.t.ID), Branch: b.ev.branch, Base: b.ev.base, Timeout: b.timeout.String(),
	}
	payload, err := json.Marshal(task)
	if err != nil {
		b.ev.err = err
		return false
	}
	res, err := r.bus.store.Send(r.bus.member, &bus.SendInput{
		To: r.bus.to, Type: bus.TypeTaskAssign, Thread: thread, IdempotencyKey: thread + "/assign",
		TTL: r.bus.claimTimeout.String(), Payload: payload,
	})
	if err != nil {
		b.writeFailed("assign", err)
		return false
	}
	b.assignAt = res.Message.CreatedAt
	if b.deadline.IsZero() {
		b.deadline = r.now().Add(r.bus.claimTimeout)
	}
	b.a.set(func(x *busAttempt) { x.thread, x.phase = thread, phaseAssigned })
	b.ev.busThread = thread
	return true
}

// writeFailed: an expired channel requeues (thread kept for a later
// revoke); any other write error fails the attempt.
func (b *busAttemptRun) writeFailed(op string, err error) {
	b.alert(AlertBusWriteFailed, SevWarn, fmt.Sprintf("bus %s for %s failed: %v", op, b.t.ID, err))
	if errors.Is(err, bus.ErrChannelExpired) {
		b.ev.requeue, b.ev.keepThread = true, true
		b.ev.err = nil
		return
	}
	b.ev.err = fmt.Errorf("bus %s failed: %w", op, err)
}

func (b *busAttemptRun) loop(ctx context.Context) {
	tick := time.NewTicker(b.r.tick)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			b.stop("canceled")
			return
		case <-tick.C:
		}
		if done := b.poll(ctx); done {
			return
		}
	}
}

// poll handles one tick; true ends the attempt.
func (b *busAttemptRun) poll(ctx context.Context) bool {
	r := b.r
	if _, err := r.bus.store.ActiveChannel(r.bus.member.Channel); errors.Is(err, bus.ErrChannelExpired) {
		// Nobody can claim or complete now: honor a stored result, else requeue.
		if c, cerr := r.bus.store.Claim(r.bus.member.Channel, b.ev.busThread); cerr == nil && c.RevokedAt == nil && b.completedByUs(&c) {
			b.result(ctx, &c)
			return true
		}
		b.writeFailed("attempt", err)
		return true
	}
	c, err := r.bus.store.Claim(r.bus.member.Channel, b.ev.busThread)
	switch {
	case errors.Is(err, bus.ErrNoClaim):
		return b.waitClaim()
	case err != nil:
		if b.owner == "" {
			return b.waitClaim() // a failing store must not hold the slot past the claim deadline
		}
		return false // transient: try again next tick
	}
	if b.owner == "" {
		return b.checkClaim(&c)
	}
	if c.CompletedAt != nil && c.Owner == b.owner && c.Epoch == b.epoch && c.RevokedAt == nil {
		b.result(ctx, &c)
		return true
	}
	if lost := b.lost(&c); lost != "" {
		b.alert(AlertBusLeaseLost, SevWarn, fmt.Sprintf("%s: %s (%s)", b.t.ID, lost, b.ev.busThread))
		if b.revokeOrResult(ctx, "lease_lost") {
			return true
		}
		b.ev.err = errBusLeaseLost
		return true
	}
	return false
}

func (b *busAttemptRun) waitClaim() bool {
	if b.r.now().Before(b.deadline) {
		return false
	}
	if b.revokeOrResult(context.Background(), "unclaimed") {
		return true
	}
	if b.ev.requeue {
		return true
	}
	b.alert(AlertBusUnclaimed, SevWarn, fmt.Sprintf("%s: no valid claim within %s", b.t.ID, b.r.bus.claimTimeout))
	b.ev.err = errBusUnclaimed
	return true
}

// checkClaim validates a first claim; an invalid one is revoked and the
// task reassigned on a new thread within the same attempt.
func (b *busAttemptRun) checkClaim(c *bus.Claim) bool {
	if why := b.r.invalidClaim(c, b.assignAt); why != "" {
		b.alert(AlertBusClaimRejected, SevWarn, fmt.Sprintf("%s: claim by %s rejected (%s)", b.t.ID, c.Owner, why))
		if _, err := b.r.bus.store.Revoke(b.r.bus.member, c.Thread, "claim_rejected"); err != nil {
			b.writeFailed("revoke", err)
			return true
		}
		if b.r.now().After(b.deadline) {
			b.alert(AlertBusUnclaimed, SevWarn, fmt.Sprintf("%s: no valid claim within %s", b.t.ID, b.r.bus.claimTimeout))
			b.ev.err = errBusUnclaimed
			return true
		}
		next := newThread(b.t.ID, b.attempt)
		if err := b.r.setBusThread(b.t.ID, b.worker, next); err != nil {
			b.ev.err = err
			return true
		}
		return !b.assign(next)
	}
	b.owner, b.epoch = c.Owner, c.Epoch
	b.a.set(func(x *busAttempt) { x.agent, x.claimedAt, x.phase = c.Owner, b.r.now(), phaseClaimed })
	return false
}

// invalidClaim returns why a claim does not count, or "".
func (r *Router) invalidClaim(c *bus.Claim, assignAt time.Time) string {
	if c.ClaimedAt.Before(assignAt) {
		return "claimed before the assign"
	}
	if c.Confinement != bus.ConfinementMarker {
		return "no confinement marker (the harness must run router_tasks)"
	}
	if strings.HasPrefix(r.bus.to, "agent:") && c.Owner != r.bus.to {
		return "not the addressee"
	}
	agents, err := r.bus.store.Agents(r.bus.member.Channel)
	if err != nil {
		return "agents unreadable"
	}
	for i := range agents {
		if bus.AgentID(agents[i].Name) == c.Owner {
			if agents[i].Role != bus.RoleWorker {
				return "role " + string(agents[i].Role) + ", not worker"
			}
			return ""
		}
	}
	return "unknown agent"
}

// lost returns why the recorded claim no longer holds, or "".
func (b *busAttemptRun) lost(c *bus.Claim) string {
	switch {
	case c.RevokedAt != nil:
		return "thread revoked"
	case c.Owner != b.owner || c.Epoch != b.epoch:
		return "taken over by " + c.Owner
	case c.ReleasedAt != nil:
		return "claim released"
	}
	st, err := b.r.bus.store.Status(b.r.bus.member.Channel)
	if err != nil {
		return ""
	}
	for i := range st.Agents {
		if bus.AgentID(st.Agents[i].Name) == b.owner && !st.Agents[i].Alive {
			return "owner not alive"
		}
	}
	return ""
}

// stop revokes on cancel (timeout, kill, ticket change, shutdown). A result
// stored first wins, as for a local agent that exited 0 before its cancel.
func (b *busAttemptRun) stop(reason string) {
	if b.revokeOrResult(context.Background(), reason) {
		return
	}
	if !b.ev.requeue && b.ev.err == nil {
		b.ev.canceled = true
	}
}

// revokeOrResult revokes the attempt thread; true when a result had already
// been stored (and was processed).
func (b *busAttemptRun) revokeOrResult(ctx context.Context, reason string) bool {
	r := b.r
	res, err := r.bus.store.Revoke(r.bus.member, b.ev.busThread, reason)
	if err != nil {
		// Expired channel or busy store: honor a result stored by the
		// recorded owner at its epoch, else give up on the thread (kept for a
		// later revoke).
		if c, cerr := r.bus.store.Claim(r.bus.member.Channel, b.ev.busThread); cerr == nil && c.RevokedAt == nil && b.completedByUs(&c) {
			b.result(ctx, &c)
			return true
		}
		b.ev.keepThread = true
		b.writeFailed("revoke", err)
		return false
	}
	if res.State == bus.RevokeCompleted && b.completedByUs(&res.Claim) {
		b.result(ctx, &res.Claim)
		return true
	}
	return false
}

// completedByUs: the claim is completed by the validated owner at its
// epoch. A claim completed before the router recorded it is validated now.
func (b *busAttemptRun) completedByUs(c *bus.Claim) bool {
	if c.CompletedAt == nil {
		return false
	}
	if b.owner == "" {
		if b.r.invalidClaim(c, b.assignAt) != "" {
			return false
		}
		b.owner, b.epoch = c.Owner, c.Epoch
	}
	return c.Owner == b.owner && c.Epoch == b.epoch
}

// result processes a completed claim: the message must come from the
// owner on this thread; PASS commits after the HEAD/branch/parent checks.
func (b *busAttemptRun) result(_ context.Context, c *bus.Claim) {
	r := b.r
	b.ev.busAgent, b.ev.busEpoch = c.Owner, c.Epoch
	msg, err := r.bus.store.Message(r.bus.member.Channel, c.ResultID)
	if err != nil || msg.From != c.Owner || msg.Thread != b.ev.busThread || msg.Type != bus.TypeTaskResult {
		b.ev.err = errBusResult
		return
	}
	var v busrun.Verdict
	_ = json.Unmarshal(msg.Payload, &v)
	if v.Summary != "" {
		r.appendLog(b.ev.log, b.worker, b.t.ID, "bus summary: "+cleanSummary(v.Summary)+"\n")
	}
	if v.Verdict != "PASS" {
		b.alert(AlertBusVerdictFail, SevWarn, fmt.Sprintf("%s: %s reported %q", b.t.ID, c.Owner, cleanText(v.Verdict, 16)))
		b.ev.err = errBusVerdict
		return
	}
	b.a.set(func(x *busAttempt) { x.phase = phaseCommitting })
	head, err := r.commitBusWorkspace(b.ws, b.t, b.ev.base)
	if err != nil {
		b.ev.err = err
		return
	}
	b.ev.head, b.ev.exit = head, 0
}

// commitBusWorkspace refuses an agent that committed, switched branch or
// detached HEAD, then commits like a local worker and checks the result has
// the base as its only parent.
func (r *Router) commitBusWorkspace(dir string, t *ticket.Ticket, base string) (string, error) {
	g, err := verifyWorkspace(r.opts.Root, dir)
	if err != nil {
		return "", err
	}
	branch := "refs/heads/tb-workspace/" + t.ID
	if err := checkBusHead(g, t.ID, branch, base, "moved off the base (the agent committed)"); err != nil {
		return "", err
	}
	head, err := r.commitWorkspace(dir, t)
	if err != nil {
		return head, err
	}
	// Re-checked after the commit: HEAD must still be the ticket branch and
	// both must point at the commit tb merge will use.
	if err := checkBusHead(g, t.ID, branch, head, "changed during the commit"); err != nil {
		return "", err
	}
	if head == base {
		return head, nil
	}
	parents, err := g.run("rev-list", "--parents", "-n", "1", "HEAD")
	if err != nil {
		return "", err
	}
	if f := strings.Fields(parents); len(f) != 2 || f[1] != base {
		return "", integrityErr("bus commit of %s does not have the base as its only parent", t.ID)
	}
	return head, nil
}

// checkBusHead: HEAD is the ticket branch, and HEAD and the branch are want.
func checkBusHead(g *trustedGit, id, branch, want, what string) error {
	if ref, err := g.run("symbolic-ref", "HEAD"); err != nil || ref != branch {
		return integrityErr("bus worktree of %s: HEAD is not %s", id, branch)
	}
	for _, rev := range []string{"HEAD", branch} {
		if sha, err := g.run("rev-parse", rev); err != nil || sha != want {
			return integrityErr("bus worktree of %s: %s %s", id, rev, what)
		}
	}
	return nil
}

// setBusThread records a new thread for a running bus attempt.
func (r *Router) setBusThread(id, worker, thread string) error {
	return r.updateMine(id, worker, func(x *ticket.Ticket) { x.BusThread = thread })
}

// cleanText strips control characters (ANSI escapes included) and cuts to
// n bytes on a rune boundary.
func cleanText(s string, n int) string {
	var b strings.Builder
	for _, c := range s {
		if unicode.IsControl(c) || unicode.Is(unicode.Cf, c) {
			continue
		}
		if b.Len()+len(string(c)) > n {
			break
		}
		b.WriteRune(c)
	}
	return b.String()
}

// cleanSummary redacts before cutting: a cut must not split a secret.
func cleanSummary(s string) string { return cleanText(mem.RedactText(s), maxSummaryBytes) }

func busState(a *busAttempt) (thread, agent, phase string, claimedAt *time.Time) {
	th, ag, ph, at := a.snapshot()
	if !at.IsZero() {
		u := at.UTC()
		claimedAt = &u
	}
	return th, ag, ph, claimedAt
}

func logRel(id string) string { return ".toolbelt/router/logs/" + id + ".log" }

func (r *Router) ticketFile(id string) string {
	p, err := filepath.Abs(r.store.Path(id))
	if err != nil {
		return r.store.Path(id)
	}
	return p
}
