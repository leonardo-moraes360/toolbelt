package bus

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func openTest(t *testing.T) (*Store, *clock) {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	c := &clock{t: time.Date(2026, 11, 17, 15, 0, 0, 0, time.UTC)}
	s.SetClock(c.now)
	return s, c
}

// setup cria canal, dono (supervisor) e um worker.
func setup(t *testing.T) (s *Store, c *clock, owner, worker Member) {
	t.Helper()
	s, c = openTest(t)
	_, ownerInv, err := s.CreateChannel("feature-x", "claude-main", 2*time.Hour, 0)
	if err != nil {
		t.Fatal(err)
	}
	o, err := s.Join("feature-x", ownerInv.Code, &AgentCard{Name: "claude-main", Vendor: "anthropic"}, "")
	if err != nil {
		t.Fatal(err)
	}
	inv, err := s.CreateInvite("feature-x", RoleWorker, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	w, err := s.Join("feature-x", strings.ToLower(inv.Code), &AgentCard{Name: "grok-1", Vendor: "xai"}, RoleWorker)
	if err != nil {
		t.Fatal(err)
	}
	return s, c, o.Member, w.Member
}

func send(t *testing.T, s *Store, m Member, in *SendInput) SendResult {
	t.Helper()
	r, err := s.Send(m, in)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestOpenPermissionsAndSchema(t *testing.T) {
	root := t.TempDir()
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	for p, want := range map[string]os.FileMode{
		Dir(root):                          0o700,
		filepath.Join(Dir(root), "bus.db"): 0o600,
	} {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != want {
			t.Errorf("%s: modo %o, quer %o", p, st.Mode().Perm(), want)
		}
	}
	s, err = Open(root) // reabre sem recriar schema
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	if _, err := Open(root); err == nil || !strings.Contains(err.Error(), "desconhecido") {
		t.Fatalf("schema desconhecido deveria falhar fechado, got %v", err)
	}
}

func TestInviteSingleUseExpiryAndBinding(t *testing.T) {
	s, c := openTest(t)
	_, ownerInv, err := s.CreateChannel("ch", "owner", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ownerInv.Role != RoleSupervisor || ownerInv.Name != "owner" {
		t.Fatalf("convite do dono: %+v", ownerInv)
	}
	if _, err := s.Join("ch", ownerInv.Code, &AgentCard{Name: "intruso"}, ""); err == nil {
		t.Fatal("convite do dono não pode ser usado por outro nome")
	}
	if _, err := s.Join("ch", ownerInv.Code, &AgentCard{Name: "owner"}, RoleWorker); err == nil {
		t.Fatal("papel diferente do convite deveria falhar")
	}
	if _, err := s.Join("ch", ownerInv.Code, &AgentCard{Name: "owner"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Join("ch", ownerInv.Code, &AgentCard{Name: "owner2"}, ""); !errors.Is(err, ErrInvalidInvite) {
		t.Fatalf("reuso: %v", err)
	}
	if _, err := s.Join("ch", "ZZZZ-ZZZZ", &AgentCard{Name: "x"}, ""); !errors.Is(err, ErrInvalidInvite) {
		t.Fatalf("código inexistente: %v", err)
	}
	inv, _ := s.CreateInvite("ch", RoleWorker, "", time.Minute)
	c.add(2 * time.Minute)
	if _, err := s.Join("ch", inv.Code, &AgentCard{Name: "late"}, ""); !errors.Is(err, ErrInvalidInvite) {
		t.Fatalf("convite expirado: %v", err)
	}
	if _, _, err := s.CreateChannel("other", "o", 0, 0); err != nil {
		t.Fatal(err)
	}
	inv2, _ := s.CreateInvite("ch", RoleWorker, "", 0)
	if _, err := s.Join("other", inv2.Code, &AgentCard{Name: "x"}, ""); !errors.Is(err, ErrInvalidInvite) {
		t.Fatalf("convite de outro canal: %v", err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM invites WHERE code_hash = ?`, inv2.Code).Scan(&n); err != nil || n != 0 {
		t.Fatalf("código em claro no store (n=%d, err=%v)", n, err)
	}
}

func TestSendRecvRoutingCursorAndPeek(t *testing.T) {
	s, _, owner, worker := setup(t)
	ctx := context.Background()
	a := send(t, s, owner, &SendInput{To: "role:worker", Type: TypeTaskAssign, Thread: "TKT-001", Payload: json.RawMessage(`{"ticket":"TKT-001"}`)})
	send(t, s, owner, &SendInput{To: "role:reviewer", Type: TypeQuery})
	b := send(t, s, owner, &SendInput{To: "agent:grok-1", Type: TypeQuery, Thread: "TKT-001"})
	c := send(t, s, owner, &SendInput{To: "topic:board", Type: TypeTaskStatus})

	got, err := s.Recv(ctx, worker, RecvOptions{Peek: true})
	if err != nil {
		t.Fatal(err)
	}
	if ids(got) != ids([]Message{a.Message, b.Message, c.Message}) {
		t.Fatalf("roteamento: %s", ids(got))
	}
	got, _ = s.Recv(ctx, worker, RecvOptions{Limit: 2})
	if len(got) != 2 {
		t.Fatalf("limit: %d", len(got))
	}
	got, _ = s.Recv(ctx, worker, RecvOptions{})
	if len(got) != 1 || got[0].ID != c.Message.ID {
		t.Fatalf("cursor não avançou: %s", ids(got))
	}
	if got, _ = s.Recv(ctx, worker, RecvOptions{}); len(got) != 0 {
		t.Fatalf("fila deveria estar vazia: %s", ids(got))
	}
	got, _ = s.Recv(ctx, worker, RecvOptions{Since: b.Message.Seq})
	if len(got) != 2 {
		t.Fatalf("since: %s", ids(got))
	}
	// o dono não recebe as próprias mensagens, só as dos outros
	if got, _ = s.Recv(ctx, owner, RecvOptions{}); len(got) != 0 {
		t.Fatalf("dono recebeu as próprias: %s", ids(got))
	}
	d := send(t, s, worker, &SendInput{To: "topic:board", Type: TypeTaskStatus})
	if got, _ = s.Recv(ctx, owner, RecvOptions{}); len(got) != 1 || got[0].ID != d.Message.ID {
		t.Fatalf("dono deveria receber topic do worker: %s", ids(got))
	}
	if got[0].From != "agent:grok-1" || got[0].V != Version {
		t.Fatalf("campos do broker: %+v", got[0])
	}
}

func ids(ms []Message) string {
	var parts []string
	for i := range ms {
		parts = append(parts, ms[i].ID)
	}
	return strings.Join(parts, ",")
}

func TestSendRules(t *testing.T) {
	s, c, owner, worker := setup(t)
	cases := []struct {
		m    Member
		in   *SendInput
		want string
	}{
		{worker, &SendInput{To: "role:worker", Type: TypeTaskAssign}, "router ou supervisor"},
		{owner, &SendInput{To: "topic:board", Type: TypeTaskResult}, "complete"},
		{owner, &SendInput{To: "agent:ninguem", Type: TypeQuery}, "não está no canal"},
		{owner, &SendInput{To: "grok-1", Type: TypeQuery}, "destino inválido"},
		{owner, &SendInput{To: "role:chefe", Type: TypeQuery}, "papel inválido"},
		{owner, &SendInput{To: "topic:x", Type: "bogus"}, "tipo inválido"},
		{owner, &SendInput{To: "topic:x", Type: TypeQuery, Payload: json.RawMessage(`[1]`)}, "objeto JSON"},
		{owner, &SendInput{To: "topic:x", Type: TypeQuery, Payload: json.RawMessage(`{"a":"` + strings.Repeat("x", MaxPayloadBytes) + `"}`)}, "excede"},
		{owner, &SendInput{To: "topic:x", Type: TypeQuery, TTL: "nunca"}, "ttl inválido"},
		{owner, &SendInput{To: "topic:x", Type: TypeQuery, Thread: "tem espaço"}, "thread inválido"},
	}
	for _, tc := range cases {
		if _, err := s.Send(tc.m, tc.in); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%+v: erro %v, quer %q", tc.in, err, tc.want)
		}
	}
	c.add(3 * time.Hour)
	if _, err := s.Send(owner, &SendInput{To: "topic:x", Type: TypeQuery}); err == nil || !strings.Contains(err.Error(), "expirou") {
		t.Fatalf("canal expirado: %v", err)
	}
}

func TestIdempotencyAndTTL(t *testing.T) {
	s, c, owner, worker := setup(t)
	first := send(t, s, owner, &SendInput{To: "topic:board", Type: TypeTaskStatus, IdempotencyKey: "TKT-001/status/1"})
	dup := send(t, s, owner, &SendInput{To: "topic:board", Type: TypeTaskStatus, IdempotencyKey: "TKT-001/status/1"})
	if !dup.Duplicate || dup.Message.ID != first.Message.ID || first.Duplicate {
		t.Fatalf("dedupe: first=%+v dup=%+v", first, dup)
	}
	// mesma chave de outro remetente não é duplicata
	if r := send(t, s, worker, &SendInput{To: "topic:board", Type: TypeTaskStatus, IdempotencyKey: "TKT-001/status/1"}); r.Duplicate {
		t.Fatal("chave é por remetente")
	}
	send(t, s, owner, &SendInput{To: "agent:grok-1", Type: TypeQuery, TTL: "1m"})
	c.add(2 * time.Minute)
	got, _ := s.Recv(context.Background(), worker, RecvOptions{})
	if len(got) != 1 || got[0].ID != first.Message.ID {
		t.Fatalf("mensagem expirada entregue: %s", ids(got))
	}
}

func TestPayloadRedaction(t *testing.T) {
	s, _, owner, _ := setup(t)
	secret := "ghp_" + strings.Repeat("a", 30)
	r := send(t, s, owner, &SendInput{To: "topic:x", Type: TypeQuery,
		Payload: json.RawMessage(`{"note":"token ` + secret + `","list":["Bearer abc.def"]}`)})
	if strings.Contains(string(r.Message.Payload), secret) || strings.Contains(string(r.Message.Payload), "abc.def") {
		t.Fatalf("segredo não redigido: %s", r.Message.Payload)
	}
	log, _ := s.Log("feature-x", "", 0)
	for _, m := range log {
		if strings.Contains(string(m.Payload), secret) {
			t.Fatal("segredo no log")
		}
	}
}

// wantErr falha se err for nil ou não contiver sub ("" aceita qualquer erro).
func wantErr(t *testing.T, err error, sub, what string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), sub) {
		t.Fatalf("%s: erro %v, quer %q", what, err, sub)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestClaimComplete(t *testing.T) {
	s, _, owner, worker := setup(t)
	inv, _ := s.CreateInvite("feature-x", RoleWorker, "", 0)
	other, err := s.Join("feature-x", inv.Code, &AgentCard{Name: "codex-1"}, "")
	must(t, err)
	_, err = s.Complete(worker, "TKT-001", nil)
	wantErr(t, err, "claim antes", "complete sem claim")
	_, err = s.ClaimThread(worker, "TKT-001")
	must(t, err)
	_, err = s.ClaimThread(worker, "TKT-001")
	must(t, err) // reclaim do dono é idempotente
	_, err = s.ClaimThread(other.Member, "TKT-001")
	wantErr(t, err, "agent:grok-1", "segundo dono")
	_, err = s.Complete(other.Member, "TKT-001", nil)
	wantErr(t, err, "só o dono", "complete por não dono")
	res, err := s.Complete(worker, "TKT-001", json.RawMessage(`{"verdict":"PASS","commit":"a1b2c3d"}`))
	must(t, err)
	if res.Message.Type != TypeTaskResult || res.Message.To != "topic:board" || res.Claim.ResultID != res.Message.ID {
		t.Fatalf("resultado: %+v", res)
	}
	_, err = s.Complete(worker, "TKT-001", nil)
	wantErr(t, err, "já foi concluída", "complete duplo")
	_, err = s.ClaimThread(other.Member, "TKT-001")
	wantErr(t, err, "concluída", "claim após conclusão")
	got, _ := s.Recv(context.Background(), owner, RecvOptions{})
	if len(got) != 1 || got[0].Type != TypeTaskResult {
		t.Fatalf("dono deveria receber task.result: %+v", got)
	}
}

func TestRecvLongPoll(t *testing.T) {
	s, _, owner, worker := setup(t)
	s.poll = 10 * time.Millisecond
	done := make(chan []Message, 1)
	go func() {
		got, err := s.Recv(context.Background(), worker, RecvOptions{Wait: 5 * time.Second})
		if err != nil {
			t.Error(err)
		}
		done <- got
	}()
	time.Sleep(50 * time.Millisecond)
	send(t, s, owner, &SendInput{To: "agent:grok-1", Type: TypeQuery})
	select {
	case got := <-done:
		if len(got) != 1 {
			t.Fatalf("long-poll: %d", len(got))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("long-poll não acordou")
	}
	start := time.Now()
	got, _ := s.Recv(context.Background(), worker, RecvOptions{Wait: 60 * time.Millisecond})
	if len(got) != 0 || time.Since(start) < 50*time.Millisecond {
		t.Fatalf("espera vazia: %d msgs em %s", len(got), time.Since(start))
	}
}

func TestConcurrentProcessesShareStore(t *testing.T) {
	root := t.TempDir()
	s1, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s1.Close()
	_, inv, err := s1.CreateChannel("ch", "a", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	sa, err := s1.Join("ch", inv.Code, &AgentCard{Name: "a"}, "")
	if err != nil {
		t.Fatal(err)
	}
	inv2, _ := s1.CreateInvite("ch", RoleWorker, "", 0)
	s2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	sb, err := s2.Join("ch", inv2.Code, &AgentCard{Name: "b"}, "")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = s1.Send(sa.Member, &SendInput{To: "agent:b", Type: TypeQuery}) }()
		go func() { defer wg.Done(); _, _ = s2.Send(sb.Member, &SendInput{To: "agent:a", Type: TypeQuery}) }()
	}
	wg.Wait()
	got, err := s2.Recv(context.Background(), sb.Member, RecvOptions{Limit: 500})
	if err != nil || len(got) != 20 {
		t.Fatalf("b recebeu %d (err %v)", len(got), err)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Seq <= got[i-1].Seq {
			t.Fatal("ordem por seq quebrada")
		}
	}
}

func TestSessionFileAndAuth(t *testing.T) {
	s, _ := openTest(t)
	_, inv, _ := s.CreateChannel("ch", "a", 0, 0)
	sess, err := s.Join("ch", inv.Code, &AgentCard{Name: "a"}, "")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(sess)
	if strings.Contains(string(b), sess.Token) {
		t.Fatal("token serializado no JSON da sessão")
	}
	if err := SaveSession(s.Root(), &sess); err != nil {
		t.Fatal(err)
	}
	p, _ := sessionPath(s.Root(), "ch", "a")
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Fatalf("modo do token: %o", st.Mode().Perm())
	}
	m, err := s.LoadMember("ch", "a")
	if err != nil || m.Role != RoleSupervisor {
		t.Fatalf("LoadMember: %+v %v", m, err)
	}
	if _, err := s.Auth("ch", "a", "errado"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("token errado: %v", err)
	}
	if _, err := s.LoadMember("ch", "b"); err == nil {
		t.Fatal("sem sessão deveria falhar")
	}
	if _, err := s.LoadMember("ch", "../x"); err == nil {
		t.Fatal("nome com path traversal")
	}
}

func TestULIDAndInviteCodeFormat(t *testing.T) {
	ts := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
	a, _ := NewULID(ts)
	b, _ := NewULID(ts.Add(time.Millisecond))
	if len(a) != 26 || a[:10] >= b[:10] || a[0] > '7' {
		t.Fatalf("ulid: %s %s", a, b)
	}
	code, _ := newInviteCode()
	if len(code) != 9 || code[4] != '-' || normalizeInvite(strings.ToLower(code)) != strings.ReplaceAll(code, "-", "") {
		t.Fatalf("código: %s", code)
	}
}

func TestOwnerNameReservedForUnboundInvites(t *testing.T) {
	s, _ := openTest(t)
	_, ownerInv, err := s.CreateChannel("c", "boss", 0, 0)
	must(t, err)
	inv, _ := s.CreateInvite("c", RoleWorker, "", 0)
	_, err = s.Join("c", inv.Code, &AgentCard{Name: "boss"}, "")
	wantErr(t, err, "reservado", "worker tomando o nome do dono")
	reserved, _ := s.CreateInvite("c", RoleReviewer, "rev", 0)
	_, err = s.Join("c", inv.Code, &AgentCard{Name: "rev"}, "")
	wantErr(t, err, "reservado", "nome com convite vinculado aberto")
	_, err = s.Join("c", ownerInv.Code, &AgentCard{Name: "boss"}, "")
	must(t, err)
	_, err = s.Join("c", reserved.Code, &AgentCard{Name: "rev"}, "")
	must(t, err)
}

func TestBoundInviteRejoinRotatesToken(t *testing.T) {
	s, _ := openTest(t)
	_, ownerInv, _ := s.CreateChannel("c", "boss", 0, 0)
	old, err := s.Join("c", ownerInv.Code, &AgentCard{Name: "boss"}, "")
	must(t, err)
	_, err = s.Send(old.Member, &SendInput{To: "topic:x", Type: TypeQuery})
	must(t, err)
	re, _ := s.CreateInvite("c", RoleSupervisor, "boss", 0)
	fresh, err := s.Join("c", re.Code, &AgentCard{Name: "boss", Vendor: "anthropic"}, "")
	must(t, err)
	if _, err := s.Auth("c", "boss", old.Token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("token antigo deveria ser invalidado: %v", err)
	}
	if m, err := s.Auth("c", "boss", fresh.Token); err != nil || m.Role != RoleSupervisor {
		t.Fatalf("token novo: %+v %v", m, err)
	}
}

func TestValidInviteWrongNameOrRoleIsGeneric(t *testing.T) {
	s, _ := openTest(t)
	_, ownerInv, _ := s.CreateChannel("c", "boss", 0, 0)
	if _, err := s.Join("c", ownerInv.Code, &AgentCard{Name: "x"}, ""); !errors.Is(err, ErrInvalidInvite) {
		t.Fatalf("nome errado: %v", err)
	}
	if _, err := s.Join("c", ownerInv.Code, &AgentCard{Name: "boss"}, RoleWorker); !errors.Is(err, ErrInvalidInvite) {
		t.Fatalf("papel errado: %v", err)
	}
	if _, err := s.Join("c", ownerInv.Code, &AgentCard{Name: "boss"}, ""); err != nil {
		t.Fatalf("convite não deveria ter sido consumido pelas tentativas: %v", err)
	}
}

func TestPayloadKeepsLargeIntegers(t *testing.T) {
	s, _, owner, _ := setup(t)
	r := send(t, s, owner, &SendInput{To: "topic:x", Type: TypeQuery,
		Payload: json.RawMessage(`{"id":9007199254740993,"big":12345678901234567890,"f":1.5}`)})
	for _, want := range []string{"9007199254740993", "12345678901234567890", "1.5"} {
		if !strings.Contains(string(r.Message.Payload), want) {
			t.Fatalf("número alterado: %s (quer %s)", r.Message.Payload, want)
		}
	}
	if _, err := s.Send(owner, &SendInput{To: "topic:x", Type: TypeQuery, Payload: json.RawMessage(`{"a":1}{"b":2}`)}); err == nil {
		t.Fatal("dois objetos concatenados deveriam falhar")
	}
}

func TestConcurrentOpenOfFreshStore(t *testing.T) {
	for round := 0; round < 5; round++ {
		root := t.TempDir()
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s, err := Open(root)
				if err != nil {
					errs <- err
					return
				}
				_, err = s.Channels()
				_ = s.Close()
				if err != nil {
					errs <- err
				}
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatalf("open concorrente: %v", err)
		}
	}
}

func TestPayloadNotHTMLEscaped(t *testing.T) {
	s, _, owner, _ := setup(t)
	r := send(t, s, owner, &SendInput{To: "topic:x", Type: TypeQuery,
		Payload: json.RawMessage(`{"task":"<uma frase> & mais"}`)})
	if !strings.Contains(string(r.Message.Payload), `<uma frase> & mais`) {
		t.Fatalf("payload escapado: %s", r.Message.Payload)
	}
	b, err := MarshalIndent(WrapAll([]Message{r.Message}))
	if err != nil || !strings.Contains(string(b), `<uma frase> & mais`) {
		t.Fatalf("saída escapada: %s (%v)", b, err)
	}
}
