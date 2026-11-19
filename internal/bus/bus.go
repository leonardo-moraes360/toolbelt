// Package bus is the local multi-vendor agent bus (ADR-013, stage B1): SQLite
// store, invite-gated channels, bus/0.1 envelope, cursor-based send/recv and
// thread claim/complete.
package bus

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Version is the protocol version stamped on every message.
const Version = "bus/0.1"

// Payloads carry references, not blobs.
const MaxPayloadBytes = 64 << 10

// MaxWait caps the recv long-poll below the tool-call timeout.
const MaxWait = 25 * time.Second

const DefaultInviteTTL = 10 * time.Minute

// Role is fixed by the invite.
type Role string

const (
	RoleRouter     Role = "router"
	RoleSupervisor Role = "supervisor"
	RoleWorker     Role = "worker"
	RoleReviewer   Role = "reviewer"
)

var Roles = []Role{RoleRouter, RoleSupervisor, RoleWorker, RoleReviewer}

func ParseRole(s string) (Role, error) {
	r := Role(strings.ToLower(strings.TrimSpace(s)))
	for _, v := range Roles {
		if v == r {
			return r, nil
		}
	}
	return "", fmt.Errorf("papel inválido %q (use router, supervisor, worker ou reviewer)", s)
}

type Type string

const (
	TypeTaskAssign Type = "task.assign"
	TypeTaskStatus Type = "task.status"
	TypeTaskResult Type = "task.result"
	TypeQuery      Type = "query"
	TypeReply      Type = "reply"
	TypeAck        Type = "ack"
	TypeUrgent     Type = "urgent"
)

var types = []Type{TypeTaskAssign, TypeTaskStatus, TypeTaskResult, TypeQuery, TypeReply, TypeAck, TypeUrgent}

func ParseType(s string) (Type, error) {
	t := Type(strings.TrimSpace(s))
	for _, v := range types {
		if v == t {
			return t, nil
		}
	}
	return "", fmt.Errorf("tipo inválido %q (use task.assign, task.status, query, reply, ack ou urgent)", s)
}

// Wake is the wake mode declared in the Agent Card.
type Wake string

const (
	WakeExec    Wake = "exec"
	WakeHarness Wake = "harness"
	WakeNotify  Wake = "notify"
	WakeManual  Wake = "manual"
)

// AgentCard is declared at join. Informational only: it grants no permission.
type AgentCard struct {
	Name           string   `json:"name"`
	Vendor         string   `json:"vendor,omitempty"`
	Model          string   `json:"model,omitempty"`
	Tier           string   `json:"tier,omitempty"`
	ContextWindow  int      `json:"context_window,omitempty"`
	Capabilities   []string `json:"capabilities,omitempty"`
	Tools          []string `json:"tools,omitempty"`
	Sandbox        string   `json:"sandbox,omitempty"`
	MaxConcurrency int      `json:"max_concurrency,omitempty"`
	Wake           Wake     `json:"wake,omitempty"`
	Session        string   `json:"session,omitempty"`
}

var (
	nameRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)
	threadRe = regexp.MustCompile(`^[A-Za-z0-9._:/-]{1,128}$`)
)

func ValidName(kind, s string) error {
	if !nameRe.MatchString(s) {
		return fmt.Errorf("%s inválido %q (use [a-z0-9][a-z0-9._-], até 63 caracteres)", kind, s)
	}
	return nil
}

func validRef(kind, s string) error {
	if s == "" {
		return nil
	}
	if !threadRe.MatchString(s) {
		return fmt.Errorf("%s inválido %q (use [A-Za-z0-9._:/-], até 128 caracteres)", kind, s)
	}
	return nil
}

// Validate also normalizes the card.
func (c *AgentCard) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	if err := ValidName("nome de agente", c.Name); err != nil {
		return err
	}
	switch c.Wake {
	case "":
		c.Wake = WakeManual
	case WakeExec, WakeHarness, WakeNotify, WakeManual:
	default:
		return fmt.Errorf("wake inválido %q (use exec, harness, notify ou manual)", c.Wake)
	}
	if c.ContextWindow < 0 || c.MaxConcurrency < 0 {
		return errors.New("context_window e max_concurrency não podem ser negativos")
	}
	return nil
}

func AgentID(name string) string { return "agent:" + name }

func ParseTarget(to string) (kind, name string, err error) {
	kind, name, ok := strings.Cut(strings.TrimSpace(to), ":")
	if !ok {
		return "", "", fmt.Errorf("destino inválido %q (use agent:<nome>, role:<papel> ou topic:<nome>)", to)
	}
	switch kind {
	case "agent", "topic":
		if err := ValidName("destino", name); err != nil {
			return "", "", err
		}
	case "role":
		if _, err := ParseRole(name); err != nil {
			return "", "", err
		}
	default:
		return "", "", fmt.Errorf("destino inválido %q (use agent:<nome>, role:<papel> ou topic:<nome>)", to)
	}
	return kind, name, nil
}

// Message is the params of the bus/message envelope.
type Message struct {
	ID             string          `json:"id"`
	Seq            int64           `json:"seq"`
	V              string          `json:"v"`
	Channel        string          `json:"channel"`
	From           string          `json:"from"`
	To             string          `json:"to"`
	Thread         string          `json:"thread,omitempty"`
	Type           Type            `json:"type"`
	CausationID    string          `json:"causation_id,omitempty"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	TTL            string          `json:"ttl,omitempty"`
	Payload        json.RawMessage `json:"payload"`
	CreatedAt      time.Time       `json:"created_at"`
}

// Envelope is JSON-RPC 2.0.
type Envelope struct {
	JSONRPC string  `json:"jsonrpc"`
	Method  string  `json:"method"`
	Params  Message `json:"params"`
}

func Wrap(m *Message) Envelope { return Envelope{JSONRPC: "2.0", Method: "bus/message", Params: *m} }

func WrapAll(ms []Message) []Envelope {
	out := make([]Envelope, len(ms))
	for i := range ms {
		out[i] = Wrap(&ms[i])
	}
	return out
}

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewULID: 48-bit ms timestamp + 80 random bits.
func NewULID(t time.Time) (string, error) {
	var b [16]byte
	ms := uint64(t.UnixMilli())
	for i := 5; i >= 0; i-- {
		b[i] = byte(ms)
		ms >>= 8
	}
	if _, err := rand.Read(b[6:]); err != nil {
		return "", err
	}
	// 128 bits -> 26 5-bit chars (2 padding bits at the top).
	var out [26]byte
	var acc uint64
	var bits uint
	idx := 25
	for i := 15; i >= 0; i-- {
		acc |= uint64(b[i]) << bits
		bits += 8
		for bits >= 5 && idx >= 0 {
			out[idx] = crockford[acc&31]
			acc >>= 5
			bits -= 5
			idx--
		}
	}
	if idx >= 0 {
		out[idx] = crockford[acc&31]
	}
	return string(out[:]), nil
}

// newInviteCode: 8 Crockford chars in two groups (K7Q2-9XMD).
func newInviteCode() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	var sb strings.Builder
	for i, v := range b {
		if i == 4 {
			sb.WriteByte('-')
		}
		sb.WriteByte(crockford[v&31])
	}
	return sb.String(), nil
}

// normalizeInvite applies Crockford rules (case, O→0, I/L→1, no hyphen).
func normalizeInvite(code string) string {
	r := strings.NewReplacer("-", "", " ", "", "O", "0", "I", "1", "L", "1")
	return r.Replace(strings.ToUpper(strings.TrimSpace(code)))
}

func newToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func hashSecret(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// marshal does not escape <, > and & (payloads are agent-to-agent text, not
// HTML) and drops the Encoder's trailing newline.
func marshal(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// MarshalIndent is json.MarshalIndent without HTML escaping (bus CLI/MCP output).
func MarshalIndent(v any) ([]byte, error) { return marshal(v, "  ") }
