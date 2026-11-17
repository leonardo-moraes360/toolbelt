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

// Tipos de alerta do watchdog.
const (
	AlertWorkerSlow    = "worker_slow"
	AlertWorkerStuck   = "worker_stuck"
	AlertWorkerCrash   = "worker_crash"
	AlertWorkerKilled  = "worker_killed"
	AlertQueueGrowing  = "queue_growing"
	AlertGlobalTimeout = "global_timeout"
	// AlertTicketCanceled: o ticket em execução foi cancelado (tb ticket cancel);
	// o router mata o agente e descarta o resultado.
	AlertTicketCanceled = "ticket_canceled"
	// AlertMultiDepBase: ticket com >=2 dependências sem merge foi baseado em HEAD.
	AlertMultiDepBase = "multi_dep_base"
	// AlertAgentEscaped: descendente do agente escapou do grupo de processos
	// (setsid/setpgid); foi morto e o ticket falha sem retry (Linux).
	AlertAgentEscaped = "agent_escaped"
	// AlertOrphanKilled: processo órfão não-agente (ex.: daemon de hook/fsmonitor do
	// usuário durante a criação do worktree, ou escape não atribuível sem agente
	// rodando) foi encerrado; nenhum ticket é falhado.
	AlertOrphanKilled = "orphan_killed"
	// AlertBoardUnreadable: um ticket YAML está inválido; workers seguem rodando.
	AlertBoardUnreadable = "board_unreadable"
)

// Severidades.
const (
	SevInfo     = "info"
	SevWarn     = "warn"
	SevError    = "error"
	SevCritical = "critical"
)

// Ações sugeridas/aplicadas.
const (
	ActionNone     = "none"
	ActionRetry    = "retry"
	ActionEscalate = "escalate"
	ActionStop     = "stop"
)

// Alert é uma linha de alerts.jsonl.
type Alert struct {
	Time     time.Time `json:"time"`
	Kind     string    `json:"kind"`
	Severity string    `json:"severity"`
	Worker   string    `json:"worker,omitempty"`
	Ticket   string    `json:"ticket,omitempty"`
	Message  string    `json:"message"`
	Action   string    `json:"action"`
}

// alertLog grava alertas em alerts.jsonl (append-only, seguro entre goroutines).
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
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // caminho interno .toolbelt/router
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

// AlertFilter filtra ReadAlerts.
type AlertFilter struct {
	Since time.Time // zero = sem limite
	Kind  string    // vazio = todos
	Limit int       // >0 = só os N mais recentes
}

// ReadAlerts lê alerts.jsonl (ausente = lista vazia), em ordem cronológica.
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
			continue // linha corrompida não derruba a leitura
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

// ValidWorkerID informa se id tem o formato w1, w2, ...
func ValidWorkerID(id string) bool { return workerRe.MatchString(id) }

// KillRequest é o conteúdo de control/kill-<worker>.json.
type KillRequest struct {
	Worker      string    `json:"worker"`
	Reason      string    `json:"reason"`
	RequestedAt time.Time `json:"requested_at"`
}

// RequestKill grava um pedido de kill que o router consome no próximo tick.
func RequestKill(root, worker, reason string, now time.Time) (string, error) {
	if !ValidWorkerID(worker) {
		return "", fmt.Errorf("worker inválido %q (esperado w1, w2, ...)", worker)
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

// alert registra um alerta (e o repassa ao progresso).
func (r *Router) alert(kind, severity, worker, tkt, msg, action string) {
	a := Alert{Time: r.now().UTC(), Kind: kind, Severity: severity, Worker: worker, Ticket: tkt, Message: msg, Action: action}
	if err := r.alerts.append(&a); err != nil {
		r.logf("aviso: não consegui gravar alerta: %v", err)
	}
	r.logf("[alerta %s/%s] %s", severity, kind, msg)
}

func (r *Router) failureAction(attempts int) string {
	if attempts < r.cfg.Retry.MaxAttempts {
		return ActionRetry
	}
	return ActionEscalate
}

// watchdogTick roda a cada Options.Tick: controle, slow/stuck, fila e timeout
// global. Devolve true quando o loop deve parar (timeout global).
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
				fmt.Sprintf("%s foi %s fora do router; encerrando o agente em %s e descartando o resultado", s.ticket, cur.Status, s.id),
				ActionNone)
			s.cancel()
			continue
		}
		elapsed := now.Sub(s.started)
		if !s.slowAlerted && elapsed >= time.Duration(r.cfg.SlowThreshold*float64(s.timeout)) && elapsed < s.timeout {
			s.slowAlerted = true
			r.alert(AlertWorkerSlow, SevWarn, s.id, s.ticket,
				fmt.Sprintf("%s em %s há %s (limite %s)", s.ticket, s.id, elapsed.Round(time.Millisecond), s.timeout),
				ActionNone)
		}
		if elapsed >= s.timeout {
			s.reason = reasonStuck
			r.alert(AlertWorkerStuck, SevCritical, s.id, s.ticket,
				fmt.Sprintf("%s em %s excedeu o timeout de %s; cancelando", s.ticket, s.id, s.timeout),
				r.failureAction(s.attempts))
			s.cancel()
		}
	}
	if now.Sub(r.startedAt) >= r.global {
		r.alert(AlertGlobalTimeout, SevCritical, "", "",
			fmt.Sprintf("timeout global de %s atingido; encerrando o router", r.global), ActionStop)
		r.stopReason = ReasonGlobalTimeout
		return true
	}
	return false
}

// checkQueue emite queue_growing quando ready > 2*workers (uma vez por transição).
func (r *Router) checkQueue(ready int) {
	growing := ready > 2*r.workers
	if growing && !r.queueGrowing {
		r.alert(AlertQueueGrowing, SevWarn, "", "",
			fmt.Sprintf("fila crescendo: %d tickets prontos para %d workers", ready, r.workers), ActionNone)
	}
	r.queueGrowing = growing
}

// processControl consome control/kill-<worker>.json.
func (r *Router) processControl() {
	for _, name := range r.killRequestNames() {
		r.handleKillRequest(name)
	}
}

// killRequestNames lista, ordenados, os arquivos kill-*.json do diretório de controle.
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

// handleKillRequest lê (e apaga) um pedido de kill e, se válido, cancela o worker.
func (r *Router) handleKillRequest(name string) {
	path := filepath.Join(ControlDir(r.opts.Root), name)
	data, err := os.ReadFile(path) //nolint:gosec // dentro de .toolbelt/router/control
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
	// Pedido anterior ao início desta execução é obsoleto (sem horário
	// legível, vale o mtime do arquivo).
	stamp := req.RequestedAt
	if (!parsed || stamp.IsZero()) && fi != nil {
		stamp = fi.ModTime()
	}
	if !stamp.IsZero() && stamp.Before(r.startedAt) {
		r.logf("kill obsoleto ignorado: %s", name)
		return
	}
	s := r.slotByID(req.Worker)
	if s == nil || s.ticket == "" || s.reason != "" {
		r.logf("kill ignorado: %s não está executando nada", req.Worker)
		return
	}
	s.reason = reasonKilled
	s.killReason = req.Reason
	msg := fmt.Sprintf("%s em %s encerrado por tb watchdog kill", s.ticket, s.id)
	if req.Reason != "" {
		msg += ": " + req.Reason
	}
	r.alert(AlertWorkerKilled, SevWarn, s.id, s.ticket, msg, r.failureAction(s.attempts))
	s.cancel()
}

// ParseSince converte "10m"/"2h" (relativo a now) ou RFC3339 em horário; vazio = zero.
func ParseSince(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		if d < 0 {
			return time.Time{}, fmt.Errorf("--since negativo: %q", s)
		}
		return now.Add(-d), nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("--since inválido %q (use 10m, 2h ou RFC3339)", s)
	}
	return t, nil
}

// clearControl apaga pedidos de kill pendentes (obsoletos no início de Run).
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
