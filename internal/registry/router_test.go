package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRouterToolsRegistered(t *testing.T) {
	names := map[string]bool{}
	for _, tool := range Default().List() {
		names[tool.Name] = true
	}
	for _, n := range []string{"tb_router_status", "tb_watchdog_alerts"} {
		if !names[n] {
			t.Errorf("tool %s não registrada", n)
		}
	}
}

func TestRouterToolsReadPersistedState(t *testing.T) {
	dir := t.TempDir()
	res, text := callTool(t, "tb_router_status", map[string]any{"cwd": dir})
	if res.IsError || text == "" {
		t.Fatalf("status sem router: %v %s", res.IsError, text)
	}
	var none map[string]any
	if err := json.Unmarshal([]byte(text), &none); err != nil || none["status"] != "none" {
		t.Fatalf("esperava status none: %s", text)
	}

	writeRouterFixture(t, dir)
	checkRouterTools(t, dir)
}

// writeRouterFixture grava state.json e alerts.jsonl sintéticos em dir.
func writeRouterFixture(t *testing.T, dir string) {
	t.Helper()
	rdir := filepath.Join(dir, ".toolbelt", "router")
	if err := os.MkdirAll(rdir, 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"pid":2147480000,"started_at":"2026-01-01T00:00:00Z","status":"running","strategy":"fifo","paused":false,"draining":false,"workers":[{"id":"w1","status":"idle","subscriptions":[]}],"queue":{"ready":0,"blocked":0,"running":0}}`
	if err := os.WriteFile(filepath.Join(rdir, "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	alerts := `{"time":"` + now.Add(-time.Hour).Format(time.RFC3339) + `","kind":"worker_slow","severity":"warn","message":"a","action":"none"}
{"time":"` + now.Format(time.RFC3339) + `","kind":"worker_crash","severity":"error","worker":"w1","ticket":"TKT-001","message":"b","action":"retry"}
`
	if err := os.WriteFile(filepath.Join(rdir, "alerts.jsonl"), []byte(alerts), 0o644); err != nil {
		t.Fatal(err)
	}
}

// checkRouterTools confere o que as tools leem do estado persistido.
func checkRouterTools(t *testing.T, dir string) {
	t.Helper()
	_, text := callTool(t, "tb_router_status", map[string]any{"cwd": dir})
	var st map[string]any
	if err := json.Unmarshal([]byte(text), &st); err != nil || st["stale"] != true || st["strategy"] != "fifo" {
		t.Fatalf("status: %s", text)
	}
	_, text = callTool(t, "tb_watchdog_alerts", map[string]any{"cwd": dir})
	var all []map[string]any
	if err := json.Unmarshal([]byte(text), &all); err != nil || len(all) != 2 {
		t.Fatalf("alerts: %s", text)
	}
	_, text = callTool(t, "tb_watchdog_alerts", map[string]any{"cwd": dir, "since": "30m", "kind": "worker_crash"})
	var some []map[string]any
	if err := json.Unmarshal([]byte(text), &some); err != nil || len(some) != 1 || some[0]["action"] != "retry" {
		t.Fatalf("alerts filtrados: %s", text)
	}
	res, _ := callTool(t, "tb_watchdog_alerts", map[string]any{"cwd": dir, "since": "xyz"})
	if !res.IsError {
		t.Fatal("since inválido deveria ser erro")
	}
}
