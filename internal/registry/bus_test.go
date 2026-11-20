package registry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
)

func callBus(t *testing.T, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := Default().Call(context.Background(), name, args)
	if err != nil {
		t.Fatal(err)
	}
	return res.Content[0].Text, res.IsError
}

func mustBus(t *testing.T, name string, args map[string]any) string {
	t.Helper()
	out, isErr := callBus(t, name, args)
	if isErr {
		t.Fatalf("%s: %s", name, out)
	}
	return out
}

func failBus(t *testing.T, name string, args map[string]any, sub string) {
	t.Helper()
	if out, isErr := callBus(t, name, args); !isErr || !strings.Contains(out, sub) {
		t.Fatalf("%s deveria falhar com %q: %s", name, sub, out)
	}
}

func busEnvelopes(t *testing.T, out string) []bus.Envelope {
	t.Helper()
	var envs []bus.Envelope
	if err := json.Unmarshal([]byte(out), &envs); err != nil {
		t.Fatalf("envelopes: %v\n%s", err, out)
	}
	return envs
}

// busChannel creates channel fx (as a human would via the CLI) and returns the invites.
func busChannel(t *testing.T) (root, ownerCode, workerCode string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := bus.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, ownerInv, err := s.CreateChannel("fx", "claude-main", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	workerInv, err := s.CreateInvite("fx", bus.RoleWorker, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	return root, ownerInv.Code, workerInv.Code
}

// TestBusMCPFlow: two MCP agents (simulated by swapping the connection's
// in-memory sessions) talk and close a thread.
func TestBusMCPFlow(t *testing.T) {
	ResetBusSessions()
	t.Cleanup(ResetBusSessions)
	root, ownerCode, workerCode := busChannel(t)

	failBus(t, "tb_bus_send", map[string]any{"to": "topic:x", "type": "query"}, "tb_bus_join")
	out := mustBus(t, "tb_bus_join", map[string]any{"channel": "fx", "invite": ownerCode, "name": "claude-main", "vendor": "anthropic", "cwd": root})
	if strings.Contains(out, "token") {
		t.Fatalf("resposta do join expõe token: %s", out)
	}
	out = mustBus(t, "tb_bus_send", map[string]any{
		"to": "role:worker", "type": "task.assign", "thread": "TKT-001",
		"payload": map[string]any{"ticket": "TKT-001"}, "idempotency_key": "TKT-001/assign/1",
	})
	if !strings.Contains(out, `"from": "agent:claude-main"`) {
		t.Fatalf("send: %s", out)
	}
	failBus(t, "tb_bus_join", map[string]any{"channel": "fx", "invite": workerCode, "name": "x", "cwd": root}, "already in channel")
	ownerSessions := busState.sessions

	ResetBusSessions() // "another connection"
	mustBus(t, "tb_bus_join", map[string]any{"channel": "fx", "invite": workerCode, "name": "grok-1", "role": "worker", "cwd": root})
	envs := busEnvelopes(t, mustBus(t, "tb_bus_recv", map[string]any{"wait": "1s"}))
	if len(envs) != 1 || envs[0].Params.Type != bus.TypeTaskAssign {
		t.Fatalf("recv: %+v", envs)
	}
	failBus(t, "tb_bus_send", map[string]any{"to": "role:worker", "type": "task.assign"}, "router or supervisor")
	mustBus(t, "tb_bus_claim", map[string]any{"thread": "TKT-001"})
	mustBus(t, "tb_bus_complete", map[string]any{"thread": "TKT-001", "epoch": 1, "result": map[string]any{"verdict": "PASS"}})

	busState.mu.Lock()
	busState.sessions = ownerSessions
	busState.mu.Unlock()
	envs = busEnvelopes(t, mustBus(t, "tb_bus_recv", map[string]any{"channel": "fx"}))
	if len(envs) != 1 || envs[0].Params.Type != bus.TypeTaskResult {
		t.Fatalf("dono deveria receber task.result: %+v", envs)
	}
}

func TestBusMCPNoChannelCreateTool(t *testing.T) {
	for _, tool := range Default().List() {
		if strings.HasPrefix(tool.Name, "tb_bus_") && (strings.Contains(tool.Name, "channel") || strings.Contains(tool.Name, "invite")) {
			t.Fatalf("criar canal/convite não pode ser exposto via MCP: %s", tool.Name)
		}
	}
	for _, name := range []string{"tb_bus_join", "tb_bus_send", "tb_bus_recv", "tb_bus_claim", "tb_bus_complete", "tb_bus_heartbeat", "tb_bus_status"} {
		if _, ok := Default().Get(name); !ok {
			t.Errorf("ferramenta ausente: %s", name)
		}
	}
}

func busLastSeen(t *testing.T, root, name string) time.Time {
	t.Helper()
	s, err := bus.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	st, err := s.Status("fx")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range st.Agents {
		if a.Name == name {
			return a.LastSeen
		}
	}
	t.Fatalf("agent %s missing", name)
	return time.Time{}
}

// AC10: the MCP server heartbeats bound sessions with no tool calls, and
// stops when the connection context ends.
func TestBusMCPBackgroundHeartbeat(t *testing.T) {
	ResetBusSessions()
	t.Cleanup(ResetBusSessions)
	root, ownerCode, _ := busChannel(t)
	mustBus(t, "tb_bus_join", map[string]any{"channel": "fx", "invite": ownerCode, "name": "claude-main", "cwd": root})
	before := busLastSeen(t, root, "claude-main")

	time.Sleep(20 * time.Millisecond)
	due := map[string]time.Time{}
	heartbeatDue(time.Now(), due)
	after := busLastSeen(t, root, "claude-main")
	if !after.After(before) {
		t.Fatalf("heartbeat did not renew last_seen: %v vs %v", after, before)
	}
	if next := due["fx"]; time.Until(next) < 30*time.Second {
		t.Fatalf("next heartbeat should follow lease/3, got %v", next)
	}
	time.Sleep(20 * time.Millisecond)
	heartbeatDue(time.Now(), due) // not due yet: no write
	if !busLastSeen(t, root, "claude-main").Equal(after) {
		t.Fatal("heartbeat ran before its interval")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { RunBusHeartbeats(ctx, time.Millisecond); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunBusHeartbeats did not stop on cancel")
	}
}

// AC12/AC13: heartbeat and status tools exist, status leaks no secrets, and
// no MCP tool runs the watchdog.
func TestBusMCPHeartbeatStatus(t *testing.T) {
	ResetBusSessions()
	t.Cleanup(ResetBusSessions)
	root, ownerCode, workerCode := busChannel(t)
	mustBus(t, "tb_bus_join", map[string]any{"channel": "fx", "invite": ownerCode, "name": "claude-main", "cwd": root})
	mustBus(t, "tb_bus_claim", map[string]any{"thread": "T1"})
	out := mustBus(t, "tb_bus_heartbeat", map[string]any{})
	if !strings.Contains(out, `"T1"`) || !strings.Contains(out, "heartbeat_interval") {
		t.Fatalf("heartbeat: %s", out)
	}
	out = mustBus(t, "tb_bus_status", map[string]any{})
	for _, secret := range []string{ownerCode, workerCode, `"session"`, "token", "payload"} {
		if strings.Contains(out, secret) {
			t.Fatalf("status leaks %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, `"T1"`) {
		t.Fatalf("status: %s", out)
	}
	for _, tool := range Default().List() {
		if strings.HasPrefix(tool.Name, "tb_bus_") && strings.Contains(tool.Name, "watchdog") {
			t.Fatalf("watchdog must not be exposed over MCP: %s", tool.Name)
		}
	}
}
