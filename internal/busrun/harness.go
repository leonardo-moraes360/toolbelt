package busrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
)

var errLocked = errors.New("another tb bus run is already running for this agent")

type Options struct {
	Root             string
	Channel          string
	Name             string
	ConfigPath       string // empty = default
	AllowUnsandboxed bool
	TBBinary         string        // tests: tb binary written to .mcp.json
	Poll             time.Duration // default 250ms
	Backoff          time.Duration // retry base; default 1s
	Attempts         int           // default 5
	Adapter          Adapter       // tests: overrides the config's adapter
}

type Harness struct {
	opts     Options
	root     string
	cfg      *Config
	store    *bus.Store
	member   bus.Member
	adapter  Adapter
	base     string // <private>/run/<channel>/<name>
	agentDir string // <private>/sessions/<channel>/<name> (token + mcp.json)
	st       State
	redactor *redactor
	unlock   func()
	now      func() time.Time

	lastVersion int64
	lastMax     int64
	firstSeen   time.Time
	lastArrival time.Time
	retryAfter  time.Time
	budgetLog   bool
	expiredLog  bool      // channel_expired already audited for this expiry
	expiredAt   time.Time // last check that found the channel expired
}

// With the channel expired and the store quiet, recheck only at this interval
// (extend changes data_version, which triggers an immediate check).
const expiredRecheck = 5 * time.Second

// Open validates config, session, sandbox and lock. Call Close when done.
func Open(opts *Options) (*Harness, error) {
	o := *opts
	if o.Poll == 0 {
		o.Poll = 250 * time.Millisecond
	}
	if o.Backoff == 0 {
		o.Backoff = time.Second
	}
	if o.Attempts == 0 {
		o.Attempts = 5
	}
	if o.ConfigPath == "" {
		p, err := ConfigPath(o.Root, o.Channel, o.Name)
		if err != nil {
			return nil, err
		}
		o.ConfigPath = p
	}
	cfg, err := LoadConfig(o.ConfigPath)
	if err != nil {
		return nil, err
	}
	store, err := bus.Open(o.Root)
	if err != nil {
		return nil, err
	}
	h := &Harness{opts: o, root: o.Root, cfg: cfg, store: store, now: time.Now}
	if err := h.init(); err != nil {
		h.Close()
		return nil, err
	}
	return h, nil
}

func (h *Harness) init() error {
	m, err := h.store.LoadMember(h.opts.Channel, h.opts.Name)
	if err != nil {
		return err
	}
	h.member = m
	if h.redactor, err = newRedactor(h.cfg); err != nil {
		return err
	}
	dir, err := RunDir(h.root, m.Channel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	h.base = filepath.Join(dir, m.Name)
	if h.agentDir, err = bus.AgentDir(h.root, m.Channel, m.Name); err != nil {
		return err
	}
	unlock, err := acquireLock(h.base + ".lock")
	if err != nil {
		return err
	}
	h.unlock = unlock
	if h.st, err = loadState(h.base + ".state"); err != nil {
		return err
	}
	// State from another store (bus.db recreated, agent rejoined): seqs restart.
	if maxSeq, err := h.store.MaxSeq(m.Channel); err != nil {
		return err
	} else if h.st.NotifiedSeq > maxSeq {
		h.st = State{Session: h.st.Session, ConfigSession: h.st.ConfigSession}
	}
	if h.opts.Adapter != nil {
		h.adapter = h.opts.Adapter
	} else if h.adapter, err = newAdapter(h); err != nil {
		return err
	}
	return nil
}

func (h *Harness) Check() error {
	if h.opts.Adapter != nil {
		return nil
	}
	bin := h.cfg.Exec.Argv
	if h.cfg.Adapter == AdapterHarness {
		cmd := h.cfg.Harness.Command
		if cmd == "" {
			cmd = "claude"
		}
		bin = []string{cmd}
	}
	if _, err := lookPath(bin[0]); err != nil {
		return fmt.Errorf("adapter binary: %w", err)
	}
	for _, p := range append(append([]string{}, h.cfg.Sandbox.Read...), h.cfg.Sandbox.Write...) {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("sandbox: %w", err)
		}
	}
	return sandboxCheck(h)
}

func (h *Harness) Close() {
	if h.unlock != nil {
		h.unlock()
		h.unlock = nil
	}
	if h.store != nil {
		_ = h.store.Close()
	}
}

func (h *Harness) Member() bus.Member { return h.member }

func (h *Harness) audit(ev *Event) {
	ev.Time = h.now().UTC()
	ev.Detail = h.redactor.redact(ev.Detail)
	_ = auditLog{path: h.base + ".jsonl"}.write(ev)
}

// pending: waking messages with seq in (from, to].
func (h *Harness) pending(from, to int64) ([]bus.Message, error) {
	return h.store.Pending(h.member, from, to, h.cfg.WakeOn, h.cfg.topics(), 500)
}

// session prefers the saved one unless the human changed harness.session in
// the config since the last run.
func (h *Harness) session() string {
	cfgSess := h.cfg.Harness.Session
	if cfgSess != h.st.ConfigSession {
		h.st.ConfigSession = cfgSess
		h.st.Session = cfgSess
	}
	return h.st.Session
}

// Errors that stop the harness; all others are transient (log and continue).
func fatal(err error) bool {
	return errors.Is(err, bus.ErrUnauthorized) || errors.Is(err, errLocked)
}

// Run: once allows at most one wake.
func (h *Harness) Run(ctx context.Context, once bool) error {
	t := time.NewTicker(h.opts.Poll)
	defer t.Stop()
	for {
		if _, err := h.step(ctx, once); err != nil {
			if once || fatal(err) {
				return err
			}
			h.audit(&Event{Event: "error", Detail: err.Error()})
			log.Warn("bus harness: transient error", "error", err)
		} else if once {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

// DryRun returns the packet that would be delivered now, without waking.
func (h *Harness) DryRun(ctx context.Context) (*Packet, error) {
	cursor, err := h.store.Cursor(h.member)
	if err != nil {
		return nil, err
	}
	msgs, err := h.pending(max(cursor, h.st.NotifiedSeq), 0)
	if err != nil {
		return nil, err
	}
	p := newPacket(h.member, "messages", msgs)
	p.Attempt = 1
	return p, nil
}

func (h *Harness) step(ctx context.Context, once bool) (bool, error) {
	v, err := h.store.DataVersion()
	if err != nil {
		return false, err
	}
	now := h.now()
	if h.idle(v, now) {
		return false, nil
	}
	h.lastVersion = v
	cursor, err := h.store.Cursor(h.member)
	if err != nil {
		return false, err
	}
	if cursor >= h.st.NotifiedSeq {
		h.st.Read = true
	}
	msgs, err := h.pending(max(cursor, h.st.NotifiedSeq), 0)
	if err != nil {
		return false, err
	}
	if len(msgs) == 0 {
		h.firstSeen = time.Time{}
		return h.maybeRewake(ctx, cursor, now)
	}
	last := msgs[len(msgs)-1].Seq
	if h.firstSeen.IsZero() {
		h.firstSeen = now
	}
	if last != h.lastMax {
		h.lastMax, h.lastArrival = last, now
	}
	if !once && !h.ready(msgs, now) {
		return false, nil
	}
	if open, err := h.channelOpen(now); err != nil || !open {
		if err == nil && once {
			err = fmt.Errorf("channel %q expired: use tb bus channel extend", h.member.Channel)
		}
		return false, err
	}
	return h.wake(ctx, "messages", msgs, now)
}

// An expired channel wakes no one (the agent could not reply). Audit once per
// expiry and keep waiting: `tb bus channel extend` reopens the channel without
// restarting the harness.
func (h *Harness) channelOpen(now time.Time) (bool, error) {
	_, err := h.store.ActiveChannel(h.member.Channel)
	if errors.Is(err, bus.ErrChannelExpired) {
		h.expiredAt = now
		if !h.expiredLog {
			h.expiredLog = true
			h.audit(&Event{Event: "channel_expired", Detail: err.Error()})
			log.Warn("bus harness: channel expired, wakes suspended until tb bus channel extend", "channel", h.member.Channel)
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	h.expiredAt = time.Time{}
	if h.expiredLog {
		h.expiredLog = false
		h.audit(&Event{Event: "channel_reopened"})
		log.Info("bus harness: channel reopened", "channel", h.member.Channel)
	}
	return true, nil
}

// idle: the store is unchanged and no wake, rewake or retry is pending.
func (h *Harness) idle(version int64, now time.Time) bool {
	if version == h.lastVersion && !h.expiredAt.IsZero() && now.Sub(h.expiredAt) < expiredRecheck {
		return true
	}
	retryDue := !h.retryAfter.IsZero() && !now.Before(h.retryAfter)
	return version == h.lastVersion && h.lastVersion != 0 && h.firstSeen.IsZero() && !h.rewakeDue(now) && !retryDue
}

func (h *Harness) ready(msgs []bus.Message, now time.Time) bool {
	if now.Before(h.retryAfter) {
		return false
	}
	for i := range msgs {
		if msgs[i].Type == bus.TypeUrgent {
			return true
		}
	}
	return now.Sub(h.lastArrival) >= h.cfg.Coalesce.Debounce || now.Sub(h.firstSeen) >= h.cfg.Coalesce.MaxDelay
}

func (h *Harness) rewakeDue(now time.Time) bool {
	return h.st.NotifiedSeq > 0 && !h.st.Read && !h.st.Unread && now.Sub(h.st.NotifiedAt) >= h.cfg.RewakeAfter
}

// maybeRewake resends the last packet once if the agent has not read it; after
// that resend (successful or not), the next deadline becomes wake_unread.
func (h *Harness) maybeRewake(ctx context.Context, cursor int64, now time.Time) (bool, error) {
	if cursor >= h.st.NotifiedSeq || !h.rewakeDue(now) || now.Before(h.retryAfter) {
		return false, nil
	}
	if open, err := h.channelOpen(now); err != nil || !open {
		return false, err
	}
	if h.st.Rewaked {
		h.st.Unread = true
		h.audit(&Event{Event: "wake_unread", LastSeq: h.st.NotifiedSeq})
		return false, h.save()
	}
	msgs, err := h.pending(cursor, h.st.NotifiedSeq)
	if err != nil || len(msgs) == 0 {
		return false, err
	}
	h.st.Rewaked = true
	h.st.NotifiedAt = now
	woke, err := h.wake(ctx, "rewake", msgs, now)
	return woke, errors.Join(err, h.save())
}

func (h *Harness) save() error { return saveState(h.base+".state", &h.st) }

func (h *Harness) wake(ctx context.Context, reason string, msgs []bus.Message, now time.Time) (bool, error) {
	if !h.st.allowWake(now, h.cfg.Budget) {
		_ = h.save()
		if !h.budgetLog {
			h.audit(&Event{Event: "wake_budget_exceeded", FirstSeq: msgs[0].Seq, LastSeq: msgs[len(msgs)-1].Seq})
			h.budgetLog = true
		}
		return false, nil
	}
	h.budgetLog = false
	if err := h.save(); err != nil { // the budget slot persists even if canceled
		return false, err
	}
	p := newPacket(h.member, reason, msgs)
	var out Outcome
	var err error
	for attempt := 1; attempt <= h.opts.Attempts; attempt++ {
		p.Attempt = attempt
		h.audit(&Event{Event: "wake_start", FirstSeq: p.FirstSeq, LastSeq: p.LastSeq, Attempt: attempt})
		start := h.now()
		out, err = h.adapter.Wake(ctx, p, &h.st)
		ok := err == nil && out.ExitCode == 0 && !out.TimedOut
		code := out.ExitCode
		ev := &Event{FirstSeq: p.FirstSeq, LastSeq: p.LastSeq, Attempt: attempt, ExitCode: &code,
			DurationMS: h.now().Sub(start).Milliseconds(), Detail: out.Output}
		if err != nil {
			ev.Detail = err.Error()
		}
		if ok {
			ev.Event = "wake_ok"
			h.audit(ev)
			if out.Session != "" {
				h.st.Session = out.Session
			}
			if reason == "messages" {
				h.st.NotifiedSeq, h.st.NotifiedAt = p.LastSeq, h.now()
				h.st.Rewaked, h.st.Unread, h.st.Read = false, false, false
			}
			h.firstSeen, h.retryAfter = time.Time{}, time.Time{}
			return true, h.save()
		}
		ev.Event = "wake_attempt_failed"
		h.audit(ev)
		if ctx.Err() != nil {
			return false, nil
		}
		if attempt < h.opts.Attempts {
			select {
			case <-ctx.Done():
				return false, nil
			case <-time.After(h.opts.Backoff << (attempt - 1)):
			}
		}
	}
	return false, h.fail(p)
}

func (h *Harness) fail(p *Packet) error {
	h.audit(&Event{Event: "wake_failed", FirstSeq: p.FirstSeq, LastSeq: p.LastSeq, Attempt: p.Attempt})
	h.retryAfter = h.now().Add(h.cfg.RewakeAfter)
	h.firstSeen = time.Time{}
	if h.st.FailedSeq == p.LastSeq { // task.status only once per last_seq
		return h.save()
	}
	h.st.FailedSeq = p.LastSeq
	payload, _ := json.Marshal(map[string]any{"state": "wake_failed", "agent": h.member.ID(), "last_seq": p.LastSeq})
	_, err := h.store.Send(h.member, &bus.SendInput{To: "topic:board", Type: bus.TypeTaskStatus, Payload: payload})
	return errors.Join(err, h.save())
}
