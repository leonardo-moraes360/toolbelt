package bus

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // driver SQLite pure-Go (CGO_ENABLED=0)

	"github.com/leonardo-moraes360/toolbelt/internal/mem"
)

// SchemaVersion é a versão do schema em PRAGMA user_version.
const SchemaVersion = 1

// ErrInvalidInvite é a resposta genérica para convite inexistente, usado,
// expirado ou de outro canal (não revela qual).
var ErrInvalidInvite = errors.New("convite inválido ou expirado")

// ErrUnauthorized indica sessão ausente ou token que não confere.
var ErrUnauthorized = errors.New("sessão do barramento inválida (faça join com um convite)")

const schema = `
CREATE TABLE channels (
	name       TEXT PRIMARY KEY,
	owner      TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	expires_at INTEGER
);
CREATE TABLE invites (
	code_hash  TEXT PRIMARY KEY,
	channel    TEXT NOT NULL REFERENCES channels(name),
	role       TEXT NOT NULL,
	bound_name TEXT,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL,
	used_at    INTEGER,
	used_by    TEXT
);
CREATE TABLE agents (
	channel    TEXT NOT NULL REFERENCES channels(name),
	name       TEXT NOT NULL,
	role       TEXT NOT NULL,
	card       TEXT NOT NULL,
	token_hash TEXT NOT NULL,
	joined_at  INTEGER NOT NULL,
	cursor     INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (channel, name)
);
CREATE TABLE messages (
	seq             INTEGER PRIMARY KEY AUTOINCREMENT,
	id              TEXT NOT NULL UNIQUE,
	channel         TEXT NOT NULL REFERENCES channels(name),
	sender          TEXT NOT NULL,
	recipient       TEXT NOT NULL,
	thread          TEXT,
	type            TEXT NOT NULL,
	causation_id    TEXT,
	idempotency_key TEXT,
	ttl             TEXT,
	expires_at      INTEGER,
	payload         TEXT NOT NULL,
	created_at      INTEGER NOT NULL
);
CREATE UNIQUE INDEX messages_idem ON messages(channel, sender, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX messages_channel_seq ON messages(channel, seq);
CREATE TABLE claims (
	channel      TEXT NOT NULL REFERENCES channels(name),
	thread       TEXT NOT NULL,
	owner        TEXT NOT NULL,
	claimed_at   INTEGER NOT NULL,
	completed_at INTEGER,
	result_id    TEXT,
	PRIMARY KEY (channel, thread)
);
`

// Store é o barramento persistido em <root>/.toolbelt/bus/bus.db.
type Store struct {
	db   *sql.DB
	root string
	dir  string
	now  func() time.Time
	poll time.Duration
}

// Dir devolve o diretório do barramento para a raiz root.
func Dir(root string) string { return filepath.Join(root, ".toolbelt", "bus") }

// Open abre (criando se preciso) o store do repositório em root.
func Open(root string) (*Store, error) {
	dir := Dir(root)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("criando %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "bus.db")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("abrindo %s: %w", path, err)
	}
	_ = f.Close()
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)") // antes do WAL: a troca de modo também espera o lock
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Set("_txlock", "immediate")
	dsn := "file:" + path + "?" + q.Encode()
	// A troca para WAL num banco novo pode devolver SQLITE_BUSY sem esperar
	// (o SQLite evita deadlock). Reabre com backoff curto até openTimeout.
	deadline := time.Now().Add(openTimeout)
	for attempt := 1; ; attempt++ {
		s, err := openOnce(dsn, root, dir)
		if err == nil || !isBusy(err) || time.Now().After(deadline) {
			return s, err
		}
		time.Sleep(time.Duration(attempt*10) * time.Millisecond)
	}
}

const openTimeout = 5 * time.Second

func openOnce(dsn, root, dir string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, root: root, dir: dir, now: time.Now, poll: 150 * time.Millisecond}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func isBusy(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") || strings.Contains(msg, "database is locked")
}

// Close fecha o store.
func (s *Store) Close() error { return s.db.Close() }

// Root devolve a raiz do repositório.
func (s *Store) Root() string { return s.root }

// SetClock troca o relógio (testes).
func (s *Store) SetClock(now func() time.Time) { s.now = now }

func (s *Store) migrate() error {
	// Lê a versão dentro da transação IMMEDIATE: dois processos abrindo um
	// store novo ao mesmo tempo serializam aqui e só um cria o schema.
	return s.tx(func(tx *sql.Tx) error {
		var v int
		if err := tx.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
			return fmt.Errorf("lendo versão do schema: %w", err)
		}
		switch v {
		case SchemaVersion:
			return nil
		case 0:
			if _, err := tx.Exec(schema); err != nil {
				return fmt.Errorf("criando schema: %w", err)
			}
			_, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, SchemaVersion))
			return err
		default:
			return fmt.Errorf("schema do barramento v%d desconhecido (este tb suporta v%d)", v, SchemaVersion)
		}
	})
}

func (s *Store) tx(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) nowNS() int64 { return s.now().UTC().UnixNano() }

func fromNS(ns int64) time.Time { return time.Unix(0, ns).UTC() }

// Channel é um canal do barramento.
type Channel struct {
	Name      string     `json:"name"`
	Owner     string     `json:"owner"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Invite é um convite recém-emitido. Code só existe aqui: o store guarda o hash.
type Invite struct {
	Code      string    `json:"code"`
	Channel   string    `json:"channel"`
	Role      Role      `json:"role"`
	Name      string    `json:"name,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateChannel cria o canal e um convite de uso único para o dono (supervisor).
func (s *Store) CreateChannel(name, owner string, ttl, inviteTTL time.Duration) (Channel, Invite, error) {
	if err := ValidName("canal", name); err != nil {
		return Channel{}, Invite{}, err
	}
	if err := ValidName("dono", owner); err != nil {
		return Channel{}, Invite{}, err
	}
	if ttl < 0 || inviteTTL < 0 {
		return Channel{}, Invite{}, errors.New("ttl não pode ser negativo")
	}
	now := s.nowNS()
	ch := Channel{Name: name, Owner: owner, CreatedAt: fromNS(now)}
	var exp sql.NullInt64
	if ttl > 0 {
		exp = sql.NullInt64{Int64: now + int64(ttl), Valid: true}
		t := fromNS(exp.Int64)
		ch.ExpiresAt = &t
	}
	var inv Invite
	err := s.tx(func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM channels WHERE name = ?`, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("canal %q já existe", name)
		}
		if _, err := tx.Exec(`INSERT INTO channels(name, owner, created_at, expires_at) VALUES (?, ?, ?, ?)`,
			name, owner, now, exp); err != nil {
			return err
		}
		var err error
		inv, err = insertInvite(tx, name, RoleSupervisor, owner, now, inviteTTL)
		return err
	})
	return ch, inv, err
}

// CreateInvite emite um convite de uso único com papel fixado. name, se não
// vazio, vincula o convite a esse agente; se ele já estiver no canal, o join
// com este convite substitui a sessão perdida (recuperação feita pelo humano).
func (s *Store) CreateInvite(channel string, role Role, name string, ttl time.Duration) (Invite, error) {
	if _, err := ParseRole(string(role)); err != nil {
		return Invite{}, err
	}
	if name != "" {
		if err := ValidName("nome de agente", name); err != nil {
			return Invite{}, err
		}
	}
	if ttl < 0 {
		return Invite{}, errors.New("ttl não pode ser negativo")
	}
	now := s.nowNS()
	var inv Invite
	err := s.tx(func(tx *sql.Tx) error {
		if _, err := activeChannel(tx, channel, now); err != nil {
			return err
		}
		var err error
		inv, err = insertInvite(tx, channel, role, name, now, ttl)
		return err
	})
	return inv, err
}

func insertInvite(tx *sql.Tx, channel string, role Role, bound string, now int64, ttl time.Duration) (Invite, error) {
	if ttl == 0 {
		ttl = DefaultInviteTTL
	}
	code, err := newInviteCode()
	if err != nil {
		return Invite{}, err
	}
	exp := now + int64(ttl)
	var b sql.NullString
	if bound != "" {
		b = sql.NullString{String: bound, Valid: true}
	}
	if _, err := tx.Exec(`INSERT INTO invites(code_hash, channel, role, bound_name, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		hashSecret(normalizeInvite(code)), channel, string(role), b, now, exp); err != nil {
		return Invite{}, err
	}
	return Invite{Code: code, Channel: channel, Role: role, Name: bound, ExpiresAt: fromNS(exp)}, nil
}

func activeChannel(tx *sql.Tx, name string, now int64) (Channel, error) {
	var ch Channel
	var created int64
	var exp sql.NullInt64
	err := tx.QueryRow(`SELECT name, owner, created_at, expires_at FROM channels WHERE name = ?`, name).
		Scan(&ch.Name, &ch.Owner, &created, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return Channel{}, fmt.Errorf("canal %q não existe", name)
	}
	if err != nil {
		return Channel{}, err
	}
	ch.CreatedAt = fromNS(created)
	if exp.Valid {
		t := fromNS(exp.Int64)
		ch.ExpiresAt = &t
		if now >= exp.Int64 {
			return ch, fmt.Errorf("canal %q expirou", name)
		}
	}
	return ch, nil
}

// Channels lista os canais.
func (s *Store) Channels() ([]Channel, error) {
	rows, err := s.db.Query(`SELECT name, owner, created_at, expires_at FROM channels ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Channel{}
	for rows.Next() {
		var ch Channel
		var created int64
		var exp sql.NullInt64
		if err := rows.Scan(&ch.Name, &ch.Owner, &created, &exp); err != nil {
			return nil, err
		}
		ch.CreatedAt = fromNS(created)
		if exp.Valid {
			t := fromNS(exp.Int64)
			ch.ExpiresAt = &t
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

// Member é um agente autenticado num canal.
type Member struct {
	Channel string `json:"channel"`
	Name    string `json:"name"`
	Role    Role   `json:"role"`
}

// ID devolve agent:<name>.
func (m Member) ID() string { return AgentID(m.Name) }

// Session é o resultado do join. Token nunca é serializado.
type Session struct {
	Member
	AgentID string    `json:"agent_id"`
	Card    AgentCard `json:"card"`
	Version string    `json:"v"`
	Token   string    `json:"-"`
}

// Join consome o convite e registra o agente com o papel do convite.
// wantRole, se não vazio, precisa coincidir com o papel do convite.
func (s *Store) Join(channel, code string, card *AgentCard, wantRole Role) (Session, error) {
	if err := card.Validate(); err != nil {
		return Session{}, err
	}
	if wantRole != "" {
		if _, err := ParseRole(string(wantRole)); err != nil {
			return Session{}, err
		}
	}
	token, err := newToken()
	if err != nil {
		return Session{}, err
	}
	cardJSON, err := json.Marshal(card)
	if err != nil {
		return Session{}, err
	}
	now := s.nowNS()
	var sess Session
	err = s.tx(func(tx *sql.Tx) error {
		if _, err := activeChannel(tx, channel, now); err != nil {
			return err
		}
		role, rejoin, err := consumeInvite(tx, channel, code, card.Name, wantRole, now)
		if err != nil {
			return err
		}
		if rejoin {
			_, err = tx.Exec(`UPDATE agents SET role = ?, card = ?, token_hash = ? WHERE channel = ? AND name = ?`,
				string(role), string(cardJSON), hashSecret(token), channel, card.Name)
		} else {
			_, err = tx.Exec(`INSERT INTO agents(channel, name, role, card, token_hash, joined_at) VALUES (?, ?, ?, ?, ?, ?)`,
				channel, card.Name, string(role), string(cardJSON), hashSecret(token), now)
		}
		if err != nil {
			return err
		}
		sess = Session{
			Member:  Member{Channel: channel, Name: card.Name, Role: role},
			AgentID: AgentID(card.Name),
			Card:    *card,
			Version: Version,
			Token:   token,
		}
		return nil
	})
	return sess, err
}

// consumeInvite valida o convite, marca como usado e retorna o papel fixado.
// rejoin indica convite vinculado a um agente que já está no canal.
// Convite válido com nome ou papel errado devolve o mesmo erro genérico.
func consumeInvite(tx *sql.Tx, channel, code, name string, wantRole Role, now int64) (role Role, rejoin bool, err error) {
	hash := hashSecret(normalizeInvite(code))
	var r string
	var bound sql.NullString
	var exp int64
	var used sql.NullInt64
	err = tx.QueryRow(`SELECT role, bound_name, expires_at, used_at FROM invites WHERE code_hash = ? AND channel = ?`,
		hash, channel).Scan(&r, &bound, &exp, &used)
	switch {
	case errors.Is(err, sql.ErrNoRows), err == nil && (used.Valid || now >= exp):
		return "", false, ErrInvalidInvite
	case err != nil:
		return "", false, err
	case bound.Valid && bound.String != name, wantRole != "" && Role(r) != wantRole:
		return "", false, ErrInvalidInvite
	}
	if !bound.Valid {
		if err := checkNameFree(tx, channel, name, now); err != nil {
			return "", false, err
		}
	}
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM agents WHERE channel = ? AND name = ?`, channel, name).Scan(&n); err != nil {
		return "", false, err
	}
	if n > 0 && !bound.Valid {
		return "", false, fmt.Errorf("agente %q já está no canal %q", name, channel)
	}
	if _, err := tx.Exec(`UPDATE invites SET used_at = ?, used_by = ? WHERE code_hash = ?`, now, name, hash); err != nil {
		return "", false, err
	}
	return Role(r), n > 0, nil
}

// checkNameFree recusa, para convite não vinculado, o nome do dono do canal e
// nomes reservados por convite vinculado ainda aberto.
func checkNameFree(tx *sql.Tx, channel, name string, now int64) error {
	var n int
	err := tx.QueryRow(`SELECT
		(SELECT COUNT(*) FROM channels WHERE name = ? AND owner = ?) +
		(SELECT COUNT(*) FROM invites WHERE channel = ? AND bound_name = ? AND used_at IS NULL AND expires_at > ?)`,
		channel, name, channel, name, now).Scan(&n)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("nome %q reservado neste canal", name)
	}
	return nil
}

// Auth valida o token de sessão e devolve o membro.
func (s *Store) Auth(channel, name, token string) (Member, error) {
	if token == "" {
		return Member{}, ErrUnauthorized
	}
	var role, hash string
	err := s.db.QueryRow(`SELECT role, token_hash FROM agents WHERE channel = ? AND name = ?`, channel, name).Scan(&role, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return Member{}, ErrUnauthorized
	}
	if err != nil {
		return Member{}, err
	}
	if hashSecret(token) != hash {
		return Member{}, ErrUnauthorized
	}
	return Member{Channel: channel, Name: name, Role: Role(role)}, nil
}

// Agents lista os agentes de um canal.
func (s *Store) Agents(channel string) ([]AgentInfo, error) {
	rows, err := s.db.Query(`SELECT name, role, card, joined_at FROM agents WHERE channel = ? ORDER BY joined_at`, channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AgentInfo{}
	for rows.Next() {
		var a AgentInfo
		var card string
		var joined int64
		if err := rows.Scan(&a.Name, &a.Role, &card, &joined); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(card), &a.Card); err != nil {
			return nil, err
		}
		a.AgentID = AgentID(a.Name)
		a.JoinedAt = fromNS(joined)
		out = append(out, a)
	}
	return out, rows.Err()
}

// AgentInfo descreve um agente registrado.
type AgentInfo struct {
	AgentID  string    `json:"agent_id"`
	Name     string    `json:"name"`
	Role     Role      `json:"role"`
	Card     AgentCard `json:"card"`
	JoinedAt time.Time `json:"joined_at"`
}

// SendInput é o que o remetente controla no envelope.
type SendInput struct {
	To             string
	Thread         string
	Type           Type
	CausationID    string
	IdempotencyKey string
	TTL            string
	Payload        json.RawMessage
}

// SendResult é o resultado do envio.
type SendResult struct {
	Message   Message `json:"message"`
	Duplicate bool    `json:"duplicate"`
}

// Send publica uma mensagem do membro m.
func (s *Store) Send(m Member, in *SendInput) (SendResult, error) {
	v, err := validateSend(m, in)
	if err != nil {
		return SendResult{}, err
	}
	var res SendResult
	err = s.tx(func(tx *sql.Tx) error {
		now := s.nowNS()
		if _, err := activeChannel(tx, m.Channel, now); err != nil {
			return err
		}
		if prev, ok, err := findIdempotent(tx, m, in.IdempotencyKey); err != nil || ok {
			res = SendResult{Message: prev, Duplicate: ok}
			return err
		}
		if v.kind == "agent" {
			if err := requireAgent(tx, m.Channel, v.target); err != nil {
				return err
			}
		}
		msg, err := insertMessage(tx, m.Channel, m.ID(), strings.TrimSpace(in.To), in.Thread, in.Type,
			in.CausationID, in.IdempotencyKey, in.TTL, v.ttl, v.payload, now)
		res = SendResult{Message: msg}
		return err
	})
	return res, err
}

type validSend struct {
	kind, target string
	ttl          time.Duration
	payload      []byte
}

func validateSend(m Member, in *SendInput) (validSend, error) {
	var v validSend
	if _, err := ParseType(string(in.Type)); err != nil {
		return v, err
	}
	if in.Type == TypeTaskResult {
		return v, errors.New("task.result só é publicado por complete (dono do claim)")
	}
	if in.Type == TypeTaskAssign && m.Role != RoleRouter && m.Role != RoleSupervisor {
		return v, fmt.Errorf("task.assign exige papel router ou supervisor (papel atual: %s)", m.Role)
	}
	var err error
	if v.kind, v.target, err = ParseTarget(in.To); err != nil {
		return v, err
	}
	for _, r := range [][2]string{{"thread", in.Thread}, {"idempotency_key", in.IdempotencyKey}, {"causation_id", in.CausationID}} {
		if err := validRef(r[0], r[1]); err != nil {
			return v, err
		}
	}
	if in.TTL != "" {
		v.ttl, err = time.ParseDuration(in.TTL)
		if err != nil || v.ttl <= 0 {
			return v, fmt.Errorf("ttl inválido %q (ex.: 30m)", in.TTL)
		}
	}
	v.payload, err = sanitizePayload(in.Payload)
	return v, err
}

func findIdempotent(tx *sql.Tx, m Member, key string) (Message, bool, error) {
	if key == "" {
		return Message{}, false, nil
	}
	prev, err := scanMessage(tx.QueryRow(selectMessage+` WHERE channel = ? AND sender = ? AND idempotency_key = ?`,
		m.Channel, m.ID(), key))
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, false, nil
	}
	if err != nil {
		return Message{}, false, err
	}
	return prev, true, nil
}

func requireAgent(tx *sql.Tx, channel, name string) error {
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM agents WHERE channel = ? AND name = ?`, channel, name).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("agente %q não está no canal %q", name, channel)
	}
	return nil
}

func insertMessage(tx *sql.Tx, channel, from, to, thread string, typ Type, causation, idem, ttlText string, ttl time.Duration, payload []byte, now int64) (Message, error) {
	id, err := NewULID(fromNS(now))
	if err != nil {
		return Message{}, err
	}
	var exp sql.NullInt64
	if ttl > 0 {
		exp = sql.NullInt64{Int64: now + int64(ttl), Valid: true}
	}
	r, err := tx.Exec(`INSERT INTO messages(id, channel, sender, recipient, thread, type, causation_id, idempotency_key, ttl, expires_at, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, channel, from, to, nullStr(thread), string(typ), nullStr(causation), nullStr(idem), nullStr(ttlText), exp, string(payload), now)
	if err != nil {
		return Message{}, err
	}
	seq, err := r.LastInsertId()
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID: id, Seq: seq, V: Version, Channel: channel, From: from, To: to, Thread: thread, Type: typ,
		CausationID: causation, IdempotencyKey: idem, TTL: ttlText, Payload: payload, CreatedAt: fromNS(now),
	}, nil
}

func nullStr(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

const selectMessage = `SELECT seq, id, channel, sender, recipient, thread, type, causation_id, idempotency_key, ttl, payload, created_at FROM messages`

type rowScanner interface{ Scan(dest ...any) error }

func scanMessage(r rowScanner) (Message, error) {
	var m Message
	var thread, causation, idem, ttl sql.NullString
	var payload string
	var created int64
	if err := r.Scan(&m.Seq, &m.ID, &m.Channel, &m.From, &m.To, &thread, &m.Type, &causation, &idem, &ttl, &payload, &created); err != nil {
		return Message{}, err
	}
	m.V = Version
	m.Thread, m.CausationID, m.IdempotencyKey, m.TTL = thread.String, causation.String, idem.String, ttl.String
	m.Payload = json.RawMessage(payload)
	m.CreatedAt = fromNS(created)
	return m, nil
}

// sanitizePayload garante objeto JSON ≤ MaxPayloadBytes e redige segredos.
func sanitizePayload(raw json.RawMessage) ([]byte, error) {
	if strings.TrimSpace(string(raw)) == "" {
		return []byte("{}"), nil
	}
	if len(raw) > MaxPayloadBytes {
		return nil, fmt.Errorf("payload excede %d bytes: envie referências (path, SHA, ticket), não conteúdo", MaxPayloadBytes)
	}
	var obj map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber() // preserva inteiros grandes (IDs, números de PR)
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("payload precisa ser um objeto JSON: %w", err)
	}
	if dec.More() {
		return nil, errors.New("payload precisa ser um único objeto JSON")
	}
	if obj == nil {
		obj = map[string]any{}
	}
	out, err := marshal(redactValue(obj), "")
	if err != nil {
		return nil, err
	}
	if len(out) > MaxPayloadBytes {
		return nil, fmt.Errorf("payload excede %d bytes: envie referências (path, SHA, ticket), não conteúdo", MaxPayloadBytes)
	}
	return out, nil
}

func redactValue(v any) any {
	switch x := v.(type) {
	case string:
		return mem.RedactText(x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[mem.RedactText(k)] = redactValue(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = redactValue(val)
		}
		return out
	default:
		return v
	}
}

// RecvOptions controla a leitura.
type RecvOptions struct {
	Wait  time.Duration // long-poll; limitado a MaxWait
	Limit int           // padrão 50
	Peek  bool          // não avança o cursor
	Since int64         // >0: relê a partir deste seq (inclusive)
}

// Recv devolve mensagens para o membro após o cursor (ou desde Since).
func (s *Store) Recv(ctx context.Context, m Member, opt RecvOptions) ([]Message, error) {
	if opt.Limit <= 0 || opt.Limit > 500 {
		opt.Limit = 50
	}
	if opt.Wait > MaxWait {
		opt.Wait = MaxWait
	}
	if opt.Since < 0 {
		return nil, errors.New("since não pode ser negativo")
	}
	deadline := time.Now().Add(opt.Wait)
	for {
		msgs, err := s.recvOnce(m, opt)
		if err != nil || len(msgs) > 0 || opt.Wait <= 0 || !time.Now().Before(deadline) {
			return msgs, err
		}
		t := time.NewTimer(s.poll)
		select {
		case <-ctx.Done():
			t.Stop()
			return []Message{}, nil
		case <-t.C:
		}
	}
}

func (s *Store) recvOnce(m Member, opt RecvOptions) ([]Message, error) {
	out := []Message{}
	err := s.tx(func(tx *sql.Tx) error {
		var cursor int64
		if err := tx.QueryRow(`SELECT cursor FROM agents WHERE channel = ? AND name = ?`, m.Channel, m.Name).Scan(&cursor); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUnauthorized
			}
			return err
		}
		after := cursor
		if opt.Since > 0 {
			after = opt.Since - 1
		}
		rows, err := tx.Query(selectMessage+` WHERE channel = ? AND seq > ? AND sender <> ?
			AND (recipient = ? OR recipient = ? OR recipient LIKE 'topic:%')
			AND (expires_at IS NULL OR expires_at > ?)
			ORDER BY seq LIMIT ?`,
			m.Channel, after, m.ID(), m.ID(), "role:"+string(m.Role), s.nowNS(), opt.Limit)
		if err != nil {
			return err
		}
		for rows.Next() {
			msg, err := scanMessage(rows)
			if err != nil {
				rows.Close()
				return err
			}
			out = append(out, msg)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if !opt.Peek && len(out) > 0 {
			last := out[len(out)-1].Seq
			if last > cursor {
				if _, err := tx.Exec(`UPDATE agents SET cursor = ? WHERE channel = ? AND name = ?`, last, m.Channel, m.Name); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return out, err
}

// Claim é o dono de uma thread.
type Claim struct {
	Channel     string     `json:"channel"`
	Thread      string     `json:"thread"`
	Owner       string     `json:"owner"`
	ClaimedAt   time.Time  `json:"claimed_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	ResultID    string     `json:"result_id,omitempty"`
}

func scanClaim(r rowScanner) (Claim, error) {
	var c Claim
	var claimed int64
	var completed sql.NullInt64
	var result sql.NullString
	if err := r.Scan(&c.Channel, &c.Thread, &c.Owner, &claimed, &completed, &result); err != nil {
		return Claim{}, err
	}
	c.ClaimedAt = fromNS(claimed)
	if completed.Valid {
		t := fromNS(completed.Int64)
		c.CompletedAt = &t
	}
	c.ResultID = result.String
	return c, nil
}

const selectClaim = `SELECT channel, thread, owner, claimed_at, completed_at, result_id FROM claims WHERE channel = ? AND thread = ?`

// ClaimThread assume a thread (dono único). Repetir o próprio claim é idempotente.
func (s *Store) ClaimThread(m Member, thread string) (Claim, error) {
	if thread == "" {
		return Claim{}, errors.New("thread é obrigatória")
	}
	if err := validRef("thread", thread); err != nil {
		return Claim{}, err
	}
	var c Claim
	err := s.tx(func(tx *sql.Tx) error {
		now := s.nowNS()
		if _, err := activeChannel(tx, m.Channel, now); err != nil {
			return err
		}
		cur, err := scanClaim(tx.QueryRow(selectClaim, m.Channel, thread))
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if _, err := tx.Exec(`INSERT INTO claims(channel, thread, owner, claimed_at) VALUES (?, ?, ?, ?)`,
				m.Channel, thread, m.ID(), now); err != nil {
				return err
			}
			c = Claim{Channel: m.Channel, Thread: thread, Owner: m.ID(), ClaimedAt: fromNS(now)}
			return nil
		case err != nil:
			return err
		case cur.CompletedAt != nil:
			return fmt.Errorf("thread %q já foi concluída por %s", thread, cur.Owner)
		case cur.Owner != m.ID():
			return fmt.Errorf("thread %q já reivindicada por %s", thread, cur.Owner)
		default:
			c = cur
			return nil
		}
	})
	return c, err
}

// CompleteResult é o resultado de complete.
type CompleteResult struct {
	Claim   Claim   `json:"claim"`
	Message Message `json:"message"`
}

// Complete encerra a thread do dono e publica task.result em topic:board.
func (s *Store) Complete(m Member, thread string, result json.RawMessage) (CompleteResult, error) {
	if thread == "" {
		return CompleteResult{}, errors.New("thread é obrigatória")
	}
	payload, err := sanitizePayload(result)
	if err != nil {
		return CompleteResult{}, err
	}
	var res CompleteResult
	err = s.tx(func(tx *sql.Tx) error {
		now := s.nowNS()
		if _, err := activeChannel(tx, m.Channel, now); err != nil {
			return err
		}
		cur, err := scanClaim(tx.QueryRow(selectClaim, m.Channel, thread))
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("thread %q não foi reivindicada (use claim antes)", thread)
		}
		if err != nil {
			return err
		}
		if cur.Owner != m.ID() {
			return fmt.Errorf("só o dono (%s) conclui a thread %q", cur.Owner, thread)
		}
		if cur.CompletedAt != nil {
			return fmt.Errorf("thread %q já foi concluída", thread)
		}
		msg, err := insertMessage(tx, m.Channel, m.ID(), "topic:board", thread, TypeTaskResult, "",
			"", "", 0, payload, now)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE claims SET completed_at = ?, result_id = ? WHERE channel = ? AND thread = ?`,
			now, msg.ID, m.Channel, thread); err != nil {
			return err
		}
		t := fromNS(now)
		cur.CompletedAt = &t
		cur.ResultID = msg.ID
		res = CompleteResult{Claim: cur, Message: msg}
		return nil
	})
	return res, err
}

// Log devolve as últimas limit mensagens do canal (trilha de auditoria),
// em ordem crescente de seq. thread filtra, se não vazio.
func (s *Store) Log(channel, thread string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 100
	}
	q := selectMessage + ` WHERE channel = ?`
	args := []any{channel}
	if thread != "" {
		q += ` AND thread = ?`
		args = append(args, thread)
	}
	q += ` ORDER BY seq DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// Cursor devolve o cursor de leitura do membro.
func (s *Store) Cursor(m Member) (int64, error) {
	var c int64
	err := s.db.QueryRow(`SELECT cursor FROM agents WHERE channel = ? AND name = ?`, m.Channel, m.Name).Scan(&c)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrUnauthorized
	}
	return c, err
}

// DataVersion devolve PRAGMA data_version: muda quando outra conexão grava.
func (s *Store) DataVersion() (int64, error) {
	var v int64
	err := s.db.QueryRow(`PRAGMA data_version`).Scan(&v)
	return v, err
}
