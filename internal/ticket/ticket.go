// Package ticket implementa o armazenamento de tickets (.toolbelt/tickets) e o
// quadro (board) que ordena o que está pronto ou bloqueado (Fase 3).
package ticket

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Status é o estado de um ticket.
type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusDone     Status = "done"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
)

// AllStatuses lista os status válidos em ordem de exibição.
var AllStatuses = []Status{StatusPending, StatusRunning, StatusDone, StatusFailed, StatusCanceled}

// Valid informa se s é um status conhecido.
func (s Status) Valid() bool {
	for _, v := range AllStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// ParseStatus converte texto em Status validando.
func ParseStatus(s string) (Status, error) {
	st := Status(strings.ToLower(strings.TrimSpace(s)))
	if !st.Valid() {
		return "", fmt.Errorf("status inválido %q (use pending, running, done, failed ou canceled)", s)
	}
	return st, nil
}

var transitions = map[Status][]Status{
	StatusPending:  {StatusRunning, StatusCanceled},
	StatusRunning:  {StatusDone, StatusFailed, StatusPending, StatusCanceled},
	StatusFailed:   {StatusPending},
	StatusCanceled: {StatusPending},
	StatusDone:     {},
}

// CanTransition informa se from -> to é permitido.
func CanTransition(from, to Status) bool {
	for _, t := range transitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// Permissões válidas do mandato.
const (
	PermRead     = "read"
	PermWrite    = "write"
	PermRunTests = "run_tests"
)

// DefaultPermissions é aplicado quando o ticket não declara permissões.
func DefaultPermissions() []string { return []string{PermRead, PermWrite, PermRunTests} }

// Mandate delimita o que o agente pode fazer.
type Mandate struct {
	Scope       []string `yaml:"scope,omitempty" json:"scope"`
	Forbidden   []string `yaml:"forbidden,omitempty" json:"forbidden"`
	Permissions []string `yaml:"permissions,omitempty" json:"permissions"`
}

// Acceptance descreve critérios de aceite e timeout (ex.: "10m").
type Acceptance struct {
	Criteria []string `yaml:"criteria,omitempty" json:"criteria"`
	Timeout  string   `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// Subscription reserva um ticket para um worker (entrega direta).
type Subscription struct {
	Worker         string    `yaml:"worker" json:"worker"`
	DirectDelivery bool      `yaml:"direct_delivery" json:"direct_delivery"`
	ExpiresAt      time.Time `yaml:"expires_at" json:"expires_at"`
}

// Live informa se a assinatura ainda vale em now.
func (s *Subscription) Live(now time.Time) bool {
	return s != nil && s.Worker != "" && now.Before(s.ExpiresAt)
}

// Result registra a execução de um ticket.
type Result struct {
	Branch     string     `yaml:"branch,omitempty" json:"branch,omitempty"`
	BaseCommit string     `yaml:"base_commit,omitempty" json:"base_commit,omitempty"`
	HeadCommit string     `yaml:"head_commit,omitempty" json:"head_commit,omitempty"`
	Workspace  string     `yaml:"workspace,omitempty" json:"workspace,omitempty"`
	ExitCode   int        `yaml:"exit_code" json:"exit_code"`
	Error      string     `yaml:"error,omitempty" json:"error,omitempty"`
	StartedAt  *time.Time `yaml:"started_at,omitempty" json:"started_at,omitempty"`
	FinishedAt *time.Time `yaml:"finished_at,omitempty" json:"finished_at,omitempty"`
	Log        string     `yaml:"log,omitempty" json:"log,omitempty"`
}

// Ticket é uma unidade de trabalho para um agente.
type Ticket struct {
	ID           string        `yaml:"id" json:"id"`
	Title        string        `yaml:"title" json:"title"`
	Description  string        `yaml:"description,omitempty" json:"description,omitempty"`
	Status       Status        `yaml:"status" json:"status"`
	Priority     int           `yaml:"priority" json:"priority"`
	CreatedAt    time.Time     `yaml:"created_at" json:"created_at"`
	UpdatedAt    time.Time     `yaml:"updated_at" json:"updated_at"`
	Mandate      Mandate       `yaml:"mandate" json:"mandate"`
	Acceptance   Acceptance    `yaml:"acceptance" json:"acceptance"`
	Dependencies []string      `yaml:"dependencies,omitempty" json:"dependencies"`
	Subscription *Subscription `yaml:"subscription,omitempty" json:"subscription,omitempty"`
	Attempts     int           `yaml:"attempts" json:"attempts"`
	Worker       string        `yaml:"worker,omitempty" json:"worker,omitempty"`
	Result       *Result       `yaml:"result,omitempty" json:"result,omitempty"`
	MergedAt     *time.Time    `yaml:"merged_at,omitempty" json:"merged_at,omitempty"`
	Source       string        `yaml:"source,omitempty" json:"source,omitempty"`
}

// TimeoutDuration devolve o timeout do ticket (0 quando não definido).
func (t *Ticket) TimeoutDuration() time.Duration {
	if t.Acceptance.Timeout == "" {
		return 0
	}
	d, err := time.ParseDuration(t.Acceptance.Timeout)
	if err != nil {
		return 0
	}
	return d
}

var idRe = regexp.MustCompile(`^TKT-(\d{3,})$`)

// NormalizeID aceita "TKT-001", "tkt-1" ou "1" e devolve o ID canônico.
func NormalizeID(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return FormatID(n), nil
	}
	if strings.HasPrefix(s, "TKT-") {
		if n, err := strconv.Atoi(strings.TrimPrefix(s, "TKT-")); err == nil && n > 0 && idRe.MatchString(FormatID(n)) {
			return FormatID(n), nil
		}
	}
	return "", fmt.Errorf("ID de ticket inválido %q (esperado TKT-NNN)", s)
}

// FormatID formata n como TKT-NNN (3+ dígitos).
func FormatID(n int) string { return fmt.Sprintf("TKT-%03d", n) }

func idNumber(id string) int {
	m := idRe.FindStringSubmatch(id)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// Validate checa campos de um ticket (sem consultar dependências).
func (t *Ticket) Validate() error {
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("título do ticket é obrigatório")
	}
	if t.Status != "" && !t.Status.Valid() {
		return fmt.Errorf("status inválido %q", t.Status)
	}
	for _, p := range t.Mandate.Permissions {
		switch p {
		case PermRead, PermWrite, PermRunTests:
		default:
			return fmt.Errorf("permissão inválida %q (use read, write ou run_tests)", p)
		}
	}
	if t.Acceptance.Timeout != "" {
		d, err := time.ParseDuration(t.Acceptance.Timeout)
		if err != nil || d <= 0 {
			return fmt.Errorf("timeout inválido %q (ex.: 10m)", t.Acceptance.Timeout)
		}
	}
	return nil
}
