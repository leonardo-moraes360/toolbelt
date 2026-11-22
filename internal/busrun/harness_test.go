package busrun

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
	p, err := ConfigPath(e.root, "ch", "w")
	if err != nil {
		t.Fatal(err)
	}
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
	dir, _ := RunDir(e.root, "ch")
	b, _ := os.ReadFile(filepath.Join(dir, "w.jsonl"))
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

	// urgent wakes without waiting for the long debounce.
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

// Runs the real exec adapter: the script records stdin and env; no payload,
// session token or pass_env value may leak into the packet or the audit.
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
		"adapter: exec\nexec:\n  argv: [true]\n":                                               "absolute",
		"adapter: harness\nharness:\n  vendor: claude\n  permission_mode: bypassPermissions\n": "bypassPermissions",
		"adapter: harness\nharness:\n  vendor: codex\n":                                        "not supported",
		"adapter: smoke\n": "invalid",
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
	if _, err := Open(&Options{Root: e.root, Channel: "ch", Name: "w", Adapter: &fakeAdapter{}}); !errors.Is(err, bus.ErrSessionInUse) || !strings.Contains(err.Error(), "pid") {
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

// Finding 1: 500+ non-waking messages must not hide a task.assign.
func TestManyNonWakingMessagesDoNotHideWake(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg)
	for i := 0; i < 501; i++ {
		e.send(t, bus.TypeAck, "")
	}
	e.send(t, bus.TypeTaskAssign, "")
	f := &fakeAdapter{}
	runFor(t, e.open(t, f), 300*time.Millisecond)
	if got := f.got(); len(got) != 1 || got[0].Messages[0].Type != bus.TypeTaskAssign {
		t.Fatalf("task.assign escondido: %+v", got)
	}
}

type flakyAdapter struct {
	fakeAdapter
	okFirst int
}

func (f *flakyAdapter) Wake(ctx context.Context, p *Packet, st *State) (Outcome, error) {
	f.mu.Lock()
	n := len(f.packets)
	f.mu.Unlock()
	if n < f.okFirst {
		return f.fakeAdapter.Wake(ctx, p, st)
	}
	f.mu.Lock()
	f.packets = append(f.packets, *p)
	f.mu.Unlock()
	return Outcome{ExitCode: 1}, nil
}

// Finding 2: a failing rewake must not loop forever or spam status.
func TestFailedRewakeIsBounded(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg+"rewake_after: 80ms\n")
	e.send(t, bus.TypeQuery, "")
	f := &flakyAdapter{okFirst: 1}
	runFor(t, e.open(t, f), 1200*time.Millisecond)
	if n := len(f.got()); n != 3 { // 1 ok + 2 attempts of the single rewake
		t.Fatalf("despertares: %d", n)
	}
	log, _ := e.store.Log("ch", "", 0)
	statuses := 0
	for i := range log {
		if log[i].Type == bus.TypeTaskStatus {
			statuses++
		}
	}
	if statuses != 1 {
		t.Fatalf("task.status wake_failed: %d", statuses)
	}
	if !strings.Contains(readAudit(t, e), "wake_unread") {
		t.Fatal("deveria terminar em wake_unread")
	}
}

// Finding 6: once the agent has read, the harness goes idle again.
func TestIdleAfterReadPastRewake(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg+"rewake_after: 300ms\n")
	e.send(t, bus.TypeQuery, "")
	f := &fakeAdapter{}
	h := e.open(t, f)
	runFor(t, h, 150*time.Millisecond)
	if _, err := e.store.Recv(context.Background(), e.worker, bus.RecvOptions{}); err != nil {
		t.Fatal(err)
	}
	runFor(t, h, 500*time.Millisecond)
	v, _ := h.store.DataVersion()
	if !h.idle(v, time.Now()) || len(f.got()) != 1 {
		t.Fatalf("idle=%v despertares=%d estado=%+v", h.idle(v, time.Now()), len(f.got()), h.st)
	}
}

// Finding 8: state from another store (seq above the current max) is discarded.
func TestStaleStateIsReset(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg)
	dir, _ := RunDir(e.root, "ch")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := saveState(filepath.Join(dir, "w.state"), &State{NotifiedSeq: 999, Session: "s"}); err != nil {
		t.Fatal(err)
	}
	e.send(t, bus.TypeQuery, "")
	f := &fakeAdapter{}
	runFor(t, e.open(t, f), 300*time.Millisecond)
	if len(f.got()) != 1 {
		t.Fatalf("mensagem nova ignorada por estado velho: %d", len(f.got()))
	}
}

func TestPrivateDirOutsideRepo(t *testing.T) {
	e := newEnv(t)
	priv, err := bus.PrivateDir(e.root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(priv, e.root) {
		t.Fatalf("diretório privado dentro do repo: %s", priv)
	}
	if _, err := os.Stat(filepath.Join(bus.Dir(e.root), "sessions")); !os.IsNotExist(err) {
		t.Fatal("sessões ainda no repositório")
	}
}

func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	return p
}

// Findings 3, 4 and 5: the claude preset found on PATH (relative command) runs
// sandboxed; the child cannot read another agent's token or the wake config;
// the audit does not keep the stream (which carries payloads).
func TestClaudePresetFromPathIsolatedAndQuiet(t *testing.T) {
	e := newEnv(t)
	bin := t.TempDir()
	ownerTok, _ := bus.AgentDir(e.root, "ch", "boss")
	ownerSess, _ := e.store.Join("ch", mustInvite(t, e), &bus.AgentCard{Name: "spy-target"}, "")
	_ = bus.SaveSession(e.root, &ownerSess)
	otherTok, _ := bus.AgentDir(e.root, "ch", "spy-target")
	cfgPath, _ := ConfigPath(e.root, "ch", "w")
	out := filepath.Join(e.root, "probe.out")
	writeScript(t, bin, "claude", `
cat "`+otherTok+`/token" > /dev/null 2>&1 && echo "OTHER_TOKEN_READ" >> `+out+`
cat "`+cfgPath+`" > /dev/null 2>&1 && echo "WAKE_CONFIG_READ" >> `+out+`
echo ran >> `+out+`
echo '{"type":"user","message":{"content":[{"type":"tool_result","content":"conteudo-privado"}]}}'
echo '{"type":"result","subtype":"success","is_error":false,"num_turns":2,"session_id":"s-1","result":"vi conteudo-privado"}'
`)
	_ = ownerTok
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	e.writeConfig(t, "adapter: harness\nharness:\n  vendor: claude\ncoalesce:\n  debounce: 50ms\n  max_delay: 1s\n")
	e.send(t, bus.TypeQuery, `{"x":"conteudo-privado"}`)
	h, err := Open(&Options{Root: e.root, Channel: "ch", Name: "w", Poll: 10 * time.Millisecond, AllowUnsandboxed: true})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	runFor(t, h, 1500*time.Millisecond)
	probe, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("claude do PATH não rodou: %v\n%s", err, readAudit(t, e))
	}
	if !landlockOn() {
		t.Log("Landlock indisponível: isolamento de leitura não verificado")
	} else if strings.Contains(string(probe), "_READ") {
		t.Fatalf("filho leu arquivo privado: %s", probe)
	}
	audit := readAudit(t, e)
	if !strings.Contains(audit, "wake_ok") || strings.Contains(audit, "conteudo-privado") {
		t.Fatalf("auditoria: %s", audit)
	}
	if h.st.Session != "s-1" {
		t.Fatalf("session: %q", h.st.Session)
	}
}

func mustInvite(t *testing.T, e *env) string {
	t.Helper()
	inv, err := e.store.CreateInvite("ch", bus.RoleWorker, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	return inv.Code
}

func TestWakeTopicsFalseIgnoresTopics(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg+"wake_topics: false\n")
	if _, err := e.store.Send(e.owner, &bus.SendInput{To: "topic:board", Type: bus.TypeQuery}); err != nil {
		t.Fatal(err)
	}
	f := &fakeAdapter{}
	h := e.open(t, f)
	runFor(t, h, 250*time.Millisecond)
	if len(f.got()) != 0 {
		t.Fatalf("topic acordou: %+v", f.got())
	}
	e.send(t, bus.TypeQuery, "")
	runFor(t, h, 250*time.Millisecond)
	if got := f.got(); len(got) != 1 || got[0].Count != 1 {
		t.Fatalf("mensagem direta: %+v", got)
	}
}

func TestExpiredChannelSuspendsWakesUntilExtend(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg)
	e.send(t, bus.TypeQuery, "")
	if _, err := e.store.ExtendChannel("ch", time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	f := &fakeAdapter{}
	h := e.open(t, f)
	runFor(t, h, 300*time.Millisecond)
	if len(f.got()) != 0 {
		t.Fatalf("acordou com canal expirado: %+v", f.got())
	}
	if a := readAudit(t, e); strings.Count(a, `"channel_expired"`) != 1 {
		t.Fatalf("channel_expired deve ser auditado uma vez: %s", a)
	}
	if _, err := e.store.ExtendChannel("ch", 0); err != nil {
		t.Fatal(err)
	}
	runFor(t, h, 300*time.Millisecond)
	if got := f.got(); len(got) != 1 || got[0].Count != 1 {
		t.Fatalf("após extend: %+v", got)
	}
	if a := readAudit(t, e); !strings.Contains(a, `"channel_reopened"`) {
		t.Fatalf("channel_reopened ausente: %s", a)
	}
}

func TestOnceOnExpiredChannelFails(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, fastCfg)
	e.send(t, bus.TypeQuery, "")
	if _, err := e.store.ExtendChannel("ch", time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	f := &fakeAdapter{}
	err := e.open(t, f).Run(context.Background(), true)
	if err == nil || !strings.Contains(err.Error(), "channel extend") {
		t.Fatalf("--once com canal expirado: %v", err)
	}
	if len(f.got()) != 0 {
		t.Fatalf("acordou: %+v", f.got())
	}
}

func TestToolbeltServerFailure(t *testing.T) {
	init := func(status string) string {
		return `{"type":"system","subtype":"init","mcp_servers":[{"name":"toolbelt","status":"` + status + `"}]}` + "\n" +
			`{"type":"result","subtype":"success"}`
	}
	if got := toolbeltServerFailure(init("connected")); got != "" {
		t.Fatalf("connected: %q", got)
	}
	if got := toolbeltServerFailure(init("failed")); !strings.Contains(got, "session_in_use") {
		t.Fatalf("failed: %q", got)
	}
	if got := toolbeltServerFailure("not json\n{}"); got != "" {
		t.Fatalf("no init event: %q", got)
	}
}

// A fake claude that exits 0 with the toolbelt server failed (a client took
// the identity after the probe) is a failed attempt, not wake_ok.
func TestClaudeWakeFailsWhenToolbeltServerDoesNotConnect(t *testing.T) {
	e := newEnv(t)
	script := filepath.Join(t.TempDir(), "claude")
	body := "#!/bin/sh\ncat <<'EOF'\n" +
		`{"type":"system","subtype":"init","mcp_servers":[{"name":"toolbelt","status":"failed"}]}` + "\n" +
		`{"type":"result","subtype":"success","is_error":false,"result":"ok","session_id":"s1"}` + "\nEOF\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	e.writeConfig(t, "adapter: harness\nharness:\n  vendor: claude\n  command: "+script+"\n")
	h := e.open(t, nil)
	out, err := claudeAdapter{h}.Wake(context.Background(), &Packet{Channel: "ch", Agent: "w"}, &State{})
	if err != nil {
		t.Fatal(err)
	}
	if out.ExitCode == 0 || !strings.Contains(out.Output, "session_in_use") {
		t.Fatalf("outcome = %+v", out)
	}
}
