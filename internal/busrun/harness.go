package busrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
)

var errLocked = errors.New("outro tb bus run já está rodando para este agente")

// Options controla o harness.
type Options struct {
	Root             string
	Channel          string
	Name             string
	ConfigPath       string // vazio = padrão
	AllowUnsandboxed bool
	TBBinary         string        // testes: binário tb usado no .mcp.json
	Poll             time.Duration // padrão 250ms
	Backoff          time.Duration // base do retry; padrão 1s
	Attempts         int           // padrão 5
	Adapter          Adapter       // testes: substitui o adapter da config
}

// Harness é o processo que espera e acorda um agente.
type Harness struct {
	opts     Options
	root     string
	cfg      *Config
	store    *bus.Store
	member   bus.Member
	adapter  Adapter
	base     string // <privado>/run/<canal>/<nome>
	agentDir string // <privado>/sessions/<canal>/<nome> (token + mcp.json)
	st       State
	unlock   func()
	now      func() time.Time

	lastVersion int64
	lastMax     int64
	firstSeen   time.Time
	lastArrival time.Time
	retryAfter  time.Time
	budgetLog   bool
}

// Open valida config, sessão, sandbox e lock. Chame Close ao terminar.
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
	// Estado de outro store (bus.db recriado, agente reentrou): seqs recomeçam.
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

// Check valida o ambiente do adapter sem acordar ninguém.
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
		return fmt.Errorf("binário do adapter: %w", err)
	}
	for _, p := range append(append([]string{}, h.cfg.Sandbox.Read...), h.cfg.Sandbox.Write...) {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("sandbox: %w", err)
		}
	}
	return sandboxCheck(h)
}

// Close libera store e lock.
func (h *Harness) Close() {
	if h.unlock != nil {
		h.unlock()
		h.unlock = nil
	}
	if h.store != nil {
		_ = h.store.Close()
	}
}

// Member devolve o agente do harness.
func (h *Harness) Member() bus.Member { return h.member }

func (h *Harness) audit(ev *Event) {
	ev.Time = h.now().UTC()
	for _, name := range h.cfg.PassEnv {
		if v := os.Getenv(name); len(v) >= 4 {
			ev.Detail = strings.ReplaceAll(ev.Detail, v, "[REDACTED]")
		}
	}
	_ = auditLog{path: h.base + ".jsonl"}.write(ev)
}

// pending devolve as mensagens que acordam com seq em (from, to].
func (h *Harness) pending(from, to int64) ([]bus.Message, error) {
	return h.store.Pending(h.member, from, to, h.cfg.WakeOn, 500)
}

// session devolve a sessão a retomar: a do estado, salvo se o humano mudou
// harness.session na config desde a última execução.
func (h *Harness) session() string {
	cfgSess := h.cfg.Harness.Session
	if cfgSess != h.st.ConfigSession {
		h.st.ConfigSession = cfgSess
		h.st.Session = cfgSess
	}
	return h.st.Session
}

// Erros que encerram o harness; os demais são transitórios (log + continua).
func fatal(err error) bool {
	return errors.Is(err, bus.ErrUnauthorized) || errors.Is(err, errLocked)
}

// Run executa o laço até ctx terminar. once: no máximo um despertar.
func (h *Harness) Run(ctx context.Context, once bool) error {
	t := time.NewTicker(h.opts.Poll)
	defer t.Stop()
	for {
		if _, err := h.step(ctx, once); err != nil {
			if once || fatal(err) {
				return err
			}
			h.audit(&Event{Event: "error", Detail: err.Error()})
			log.Warn("harness do bus: erro transitório", "error", err)
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

// DryRun devolve o packet que seria entregue agora (sem despertar).
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
	return h.wake(ctx, "messages", msgs, now)
}

// idle: nada mudou no store e não há despertar pendente, rewake ou retry.
func (h *Harness) idle(version int64, now time.Time) bool {
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

// maybeRewake reenvia uma única vez o último packet se o agente não leu;
// tendo reenviado (com sucesso ou não), o próximo prazo vira wake_unread.
func (h *Harness) maybeRewake(ctx context.Context, cursor int64, now time.Time) (bool, error) {
	if cursor >= h.st.NotifiedSeq || !h.rewakeDue(now) || now.Before(h.retryAfter) {
		return false, nil
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

// wake aplica orçamento, chama o adapter com retry e grava o estado.
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
	if err := h.save(); err != nil { // slot do orçamento persiste mesmo se cancelar
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
	if h.st.FailedSeq == p.LastSeq { // task.status só uma vez por last_seq
		return h.save()
	}
	h.st.FailedSeq = p.LastSeq
	payload, _ := json.Marshal(map[string]any{"state": "wake_failed", "agent": h.member.ID(), "last_seq": p.LastSeq})
	_, err := h.store.Send(h.member, &bus.SendInput{To: "topic:board", Type: bus.TypeTaskStatus, Payload: payload})
	return errors.Join(err, h.save())
}
