package busrun

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
)

type fakeAdapter struct {
	mu      sync.Mutex
	packets []Packet
	fail    bool
}

func (f *fakeAdapter) Wake(_ context.Context, p *Packet, _ *State) (Outcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.packets = append(f.packets, *p)
	if f.fail {
		return Outcome{ExitCode: 1, Output: "falhou"}, nil
	}
	return Outcome{Session: "sess-1"}, nil
}

func (f *fakeAdapter) got() []Packet {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Packet(nil), f.packets...)
}

type env struct {
	root          string
	store         *bus.Store
	owner, worker bus.Member
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := bus.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	_, oi, err := s.CreateChannel("ch", "boss", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	o, err := s.Join("ch", oi.Code, &bus.AgentCard{Name: "boss"}, "")
	if err != nil {
		t.Fatal(err)
	}
	wi, _ := s.CreateInvite("ch", bus.RoleWorker, "", 0)
	w, err := s.Join("ch", wi.Code, &bus.AgentCard{Name: "w"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := bus.SaveSession(root, &w); err != nil {
		t.Fatal(err)
	}
	return &env{root: root, store: s, owner: o.Member, worker: w.Member}
}

func (e *env) writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := ConfigPath(e.root, "ch", "w")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func (e *env) send(t *testing.T, typ bus.Type, payload string) bus.Message {
	t.Helper()
	r, err := e.store.Send(e.owner, &bus.SendInput{To: "agent:w", Type: typ, Payload: json.RawMessage(payload)})
	if err != nil {
		t.Fatal(err)
	}
	return r.Message
}

func (e *env) open(t *testing.T, a Adapter) *Harness {
	t.Helper()
	h, err := Open(&Options{Root: e.root, Channel: "ch", Name: "w", Adapter: a, Poll: 10 * time.Millisecond,
		Backoff: time.Millisecond, Attempts: 2, AllowUnsandboxed: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return h
}

func runFor(t *testing.T, h *Harness, d time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	if err := h.Run(ctx, false); err != nil {
		t.Fatal(err)
	}
}

func readAudit(t *testing.T, e *env) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(RunDir(e.root, "ch"), "w.jsonl"))
	return string(b)
}

const fastCfg = "adapter: exec\nexec:\n  argv: [/bin/true]\ncoalesce:\n  debounce: 80ms\n  max_delay: 2s\n"

func TestIdleNeverWakes(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg)
	f := &fakeAdapter{}
	runFor(t, e.open(t, f), 200*time.Millisecond)
	if n := len(f.got()); n != 0 {
		t.Fatalf("acordou %d vezes sem mensagem", n)
	}
}

func TestCoalesceFilterAndUrgent(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg)
	for i := 0; i < 3; i++ {
		e.send(t, bus.TypeQuery, `{"n":1}`)
	}
	e.send(t, bus.TypeAck, "")
	e.send(t, bus.TypeTaskStatus, "")
	f := &fakeAdapter{}
	h := e.open(t, f)
	runFor(t, h, 400*time.Millisecond)
	got := f.got()
	if len(got) != 1 || got[0].Count != 3 || got[0].Reason != "messages" {
		t.Fatalf("agrupamento: %+v", got)
	}
	if h.st.Session != "sess-1" || h.st.NotifiedSeq != got[0].LastSeq {
		t.Fatalf("estado: %+v", h.st)
	}

	// urgent acorda sem esperar o debounce longo
	e2 := newEnv(t)
	e2.writeConfig(t, "adapter: exec\nexec:\n  argv: [/bin/true]\ncoalesce:\n  debounce: 10s\n  max_delay: 20s\n")
	e2.send(t, bus.TypeUrgent, "")
	f2 := &fakeAdapter{}
	runFor(t, e2.open(t, f2), 150*time.Millisecond)
	if len(f2.got()) != 1 {
		t.Fatalf("urgent não acordou: %+v", f2.got())
	}
}

func TestBudgetDefersWithoutLoss(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg+"budget:\n  max_wakes: 1\n  per: 1h\n")
	e.send(t, bus.TypeQuery, "")
	f := &fakeAdapter{}
	h := e.open(t, f)
	runFor(t, h, 250*time.Millisecond)
	e.send(t, bus.TypeQuery, "")
	runFor(t, h, 250*time.Millisecond)
	if n := len(f.got()); n != 1 {
		t.Fatalf("orçamento: %d despertares", n)
	}
	if !strings.Contains(readAudit(t, e), "wake_budget_exceeded") {
		t.Fatal("estouro de orçamento não auditado")
	}
	msgs, _ := e.store.Recv(context.Background(), e.worker, bus.RecvOptions{})
	if len(msgs) != 2 {
		t.Fatalf("mensagens perdidas: %d", len(msgs))
	}
}

func TestRewakeThenUnread(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg+"rewake_after: 100ms\n")
	e.send(t, bus.TypeQuery, "")
	f := &fakeAdapter{}
	runFor(t, e.open(t, f), 600*time.Millisecond)
	got := f.got()
	if len(got) != 2 || got[1].Reason != "rewake" {
		t.Fatalf("rewake: %+v", got)
	}
	if !strings.Contains(readAudit(t, e), "wake_unread") {
		t.Fatal("wake_unread não auditado")
	}
}

func TestReadMessagesDoNotRewake(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg+"rewake_after: 400ms\n")
	e.send(t, bus.TypeQuery, "")
	f := &fakeAdapter{}
	h := e.open(t, f)
	runFor(t, h, 200*time.Millisecond)
	if _, err := e.store.Recv(context.Background(), e.worker, bus.RecvOptions{}); err != nil {
		t.Fatal(err)
	}
	runFor(t, h, 700*time.Millisecond)
	if n := len(f.got()); n != 1 {
		t.Fatalf("acordou de novo após leitura: %d", n)
	}
}

func TestExecFailureRetriesAndKeepsMessage(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg)
	msg := e.send(t, bus.TypeQuery, "")
	f := &fakeAdapter{fail: true}
	h := e.open(t, f)
	runFor(t, h, 300*time.Millisecond)
	if n := len(f.got()); n != 2 {
		t.Fatalf("tentativas: %d", n)
	}
	audit := readAudit(t, e)
	if !strings.Contains(audit, "wake_failed") {
		t.Fatalf("wake_failed ausente: %s", audit)
	}
	log, _ := e.store.Log("ch", "", 0)
	last := log[len(log)-1]
	if last.Type != bus.TypeTaskStatus || last.From != "agent:w" || !strings.Contains(string(last.Payload), "wake_failed") {
		t.Fatalf("task.status de falha: %+v", last)
	}
	msgs, _ := e.store.Recv(context.Background(), e.worker, bus.RecvOptions{})
	if len(msgs) != 1 || msgs[0].ID != msg.ID {
		t.Fatalf("mensagem perdida: %+v", msgs)
	}
}

// TestExecAdapterPacketHasNoSecrets roda o adapter exec real: o script grava
// stdin e ambiente; nada de payload, token de sessão ou valor de pass_env vaza
// para o packet nem para a auditoria.
func TestExecAdapterPacketHasNoSecrets(t *testing.T) {
	e := newEnv(t)
	bin := t.TempDir()
	script := filepath.Join(bin, "wake.sh")
	out := filepath.Join(e.root, "wake.out")
	body := "#!/bin/sh\ncat > " + out + "\necho \"KEY=$TB_WAKE_TEST_KEY CH=$TB_BUS_CHANNEL\" >> " + out + "\necho \"saida com $TB_WAKE_TEST_KEY\"\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TB_WAKE_TEST_KEY", "ghp_"+strings.Repeat("s", 30))
	t.Setenv("OTHER_SECRET_TOKEN", "nao-deve-passar")
	e.writeConfig(t, "adapter: exec\nexec:\n  argv: ["+script+"]\npass_env: [TB_WAKE_TEST_KEY]\ncoalesce:\n  debounce: 50ms\n  max_delay: 1s\n")
	e.send(t, bus.TypeQuery, `{"segredo":"conteudo-privado"}`)
	h, err := Open(&Options{Root: e.root, Channel: "ch", Name: "w", Poll: 10 * time.Millisecond, AllowUnsandboxed: true})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	runFor(t, h, 1500*time.Millisecond)
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("script não rodou: %v\n%s", err, readAudit(t, e))
	}
	got := string(b)
	if !strings.Contains(got, `"v":"bus-wake/0.1"`) || !strings.Contains(got, "CH=ch") || !strings.Contains(got, "KEY=ghp_") {
		t.Fatalf("packet/env: %s", got)
	}
	if strings.Contains(got, "conteudo-privado") || strings.Contains(got, "nao-deve-passar") {
		t.Fatalf("vazamento no filho: %s", got)
	}
	audit := readAudit(t, e)
	if !strings.Contains(audit, "wake_ok") || strings.Contains(audit, "ghp_s") || strings.Contains(audit, "conteudo-privado") {
		t.Fatalf("auditoria: %s", audit)
	}
}

func TestConfigValidationAndLock(t *testing.T) {
	e := newEnv(t)
	cases := map[string]string{
		"adapter: exec\nexec:\n  argv: [true]\n":                                               "absoluto",
		"adapter: harness\nharness:\n  vendor: claude\n  permission_mode: bypassPermissions\n": "bypassPermissions",
		"adapter: harness\nharness:\n  vendor: codex\n":                                        "não suportado",
		"adapter: smoke\n": "inválido",
		"adapter: exec\nexec:\n  argv: [/bin/true]\nbogus: 1\n": "bogus",
	}
	for body, want := range cases {
		p := e.writeConfig(t, body)
		if _, err := LoadConfig(p); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: erro %v, quer %q", body, err, want)
		}
	}
	p := e.writeConfig(t, fastCfg)
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(p); err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Fatalf("permissão aberta: %v", err)
	}
	_ = os.Chmod(p, 0o600)
	e.open(t, &fakeAdapter{})
	if _, err := Open(&Options{Root: e.root, Channel: "ch", Name: "w", Adapter: &fakeAdapter{}}); err == nil || !strings.Contains(err.Error(), "já está rodando") {
		t.Fatalf("segundo harness: %v", err)
	}
}

func TestDryRunAndOnce(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, "adapter: exec\nexec:\n  argv: [/bin/true]\ncoalesce:\n  debounce: 10s\n  max_delay: 20s\n")
	e.send(t, bus.TypeTaskAssign, `{"x":"privado"}`)
	f := &fakeAdapter{}
	h := e.open(t, f)
	p, err := h.DryRun(context.Background())
	if err != nil || p.Count != 1 || p.Messages[0].Type != bus.TypeTaskAssign {
		t.Fatalf("dry-run: %+v %v", p, err)
	}
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), "privado") {
		t.Fatal("packet com payload")
	}
	if len(f.got()) != 0 {
		t.Fatal("dry-run acordou")
	}
	if err := h.Run(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if len(f.got()) != 1 {
		t.Fatalf("--once deveria acordar ignorando o debounce: %d", len(f.got()))
	}
}

func TestLastSessionID(t *testing.T) {
	out := `{"type":"system","subtype":"init","session_id":"a"}
nao json
{"type":"result","session_id":"b","result":"ok"}`
	if got := lastSessionID(out); got != "b" {
		t.Fatalf("session: %q", got)
	}
}
