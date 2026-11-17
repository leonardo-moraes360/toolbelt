package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func routerTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("symbolic-ref", "HEAD", "refs/heads/main")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "t")
	git("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".toolbelt/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "init")
	return dir
}

func TestRouterWatchdogCLI(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	bin := buildTB(t)
	dir := routerTestRepo(t)
	must := func(args ...string) string {
		t.Helper()
		out, err := runTB(t, bin, dir, args...)
		if err != nil {
			t.Fatalf("tb %v: %v\n%s", args, err, out)
		}
		return out
	}
	must("ticket", "create", "--title", "base")
	must("ticket", "create", "--title", "filho", "--dep", "TKT-001")
	must("ticket", "create", "--title", "livre")

	// sem estado: status/alerts vazios, kill recusado
	if out := must("router", "status"); !strings.Contains(out, "Nenhum router") {
		t.Fatalf("status vazio: %s", out)
	}
	if out := must("watchdog", "alerts"); !strings.Contains(out, "Nenhum alerta") {
		t.Fatalf("alerts vazio: %s", out)
	}
	if out, err := runTB(t, bin, dir, "watchdog", "kill", "w1", "--reason", "x"); err == nil || !strings.Contains(out, "nenhum router") {
		t.Fatalf("kill sem router deveria falhar: %v %s", err, out)
	}

	// config efetiva
	var rc map[string]any
	if err := json.Unmarshal([]byte(must("router", "config", "--json")), &rc); err != nil || rc["max_workers"] != float64(3) || rc["strategy"] != "dependency" {
		t.Fatalf("config: %v %v", err, rc)
	}
	if out := must("router", "config"); !strings.Contains(out, "max_workers: 3") || !strings.Contains(out, "subscription_ttl: 30m") {
		t.Fatalf("config texto: %s", out)
	}

	// dry-run não executa nada
	out := must("router", "run", "--dry-run", "--workers", "2")
	if !strings.Contains(out, "Rodada 1") || !strings.Contains(out, "Rodada 2") || !strings.Contains(out, "TKT-002") {
		t.Fatalf("dry-run: %s", out)
	}
	var plan map[string]any
	if err := json.Unmarshal([]byte(must("router", "run", "--dry-run", "--json")), &plan); err != nil || len(plan["rounds"].([]any)) != 2 {
		t.Fatalf("dry-run json: %v %v", err, plan)
	}
	if out := must("ticket", "show", "TKT-001"); !strings.Contains(out, "pending") {
		t.Fatalf("dry-run não deveria alterar tickets: %s", out)
	}

	// sem agent_cmd: erro claro
	if out, err := runTB(t, bin, dir, "router", "run", "--allow-unsandboxed"); err == nil || !strings.Contains(out, "agent_cmd") {
		t.Fatalf("sem agent-cmd deveria falhar: %v %s", err, out)
	}

	// execução real
	var sum map[string]any
	raw := must("router", "run", "--workers", "2", "--allow-unsandboxed", "--json",
		"--agent-cmd", `sh -c 'echo feito-$TB_TICKET_ID > saida.txt'`)
	if err := json.Unmarshal([]byte(raw), &sum); err != nil {
		t.Fatalf("summary json: %v\n%s", err, raw)
	}
	if sum["reason"] != "completed" || len(sum["done"].([]any)) != 3 {
		t.Fatalf("summary: %v", sum)
	}
	var tk map[string]any
	if err := json.Unmarshal([]byte(must("ticket", "show", "TKT-002", "--json")), &tk); err != nil {
		t.Fatal(err)
	}
	res := tk["result"].(map[string]any)
	if tk["status"] != "done" || res["head_commit"] == "" || res["branch"] != "tb-workspace/TKT-002" {
		t.Fatalf("TKT-002: %v", tk)
	}

	var st map[string]any
	if err := json.Unmarshal([]byte(must("router", "status", "--json")), &st); err != nil || st["status"] != "stopped" || st["stale"] != false {
		t.Fatalf("status: %v %v", err, st)
	}
	if out := must("watchdog", "status", "--worker", "w1"); !strings.Contains(out, "w1") || strings.Contains(out, "w2") {
		t.Fatalf("watchdog status: %s", out)
	}
	if out, err := runTB(t, bin, dir, "watchdog", "status", "--worker", "w9"); err == nil {
		t.Fatalf("worker inexistente deveria falhar: %s", out)
	}

	// falha: alerta de crash visível pelo CLI
	must("ticket", "create", "--title", "quebra")
	out, err := runTB(t, bin, dir, "router", "run", "--workers", "1", "--allow-unsandboxed", "--agent-cmd", "sh -c 'exit 7'")
	if err == nil || !strings.Contains(out, "TKT-004") {
		t.Fatalf("falha deveria dar exit != 0: %v %s", err, out)
	}
	var alerts []map[string]any
	if err := json.Unmarshal([]byte(must("watchdog", "alerts", "--kind", "worker_crash", "--json")), &alerts); err != nil || len(alerts) != 2 {
		t.Fatalf("alerts: %v %v", err, alerts)
	}
	if alerts[0]["action"] != "retry" || alerts[1]["action"] != "escalate" || alerts[1]["ticket"] != "TKT-004" {
		t.Fatalf("alerts: %v", alerts)
	}
	if out := must("watchdog", "alerts", "--since", "1h", "--limit", "1"); !strings.Contains(out, "worker_crash") {
		t.Fatalf("alerts texto: %s", out)
	}
}

func TestRouterRunStalledExitsNonZero(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	bin := buildTB(t)
	dir := routerTestRepo(t)
	must := func(args ...string) string {
		t.Helper()
		out, err := runTB(t, bin, dir, args...)
		if err != nil {
			t.Fatalf("tb %v: %v\n%s", args, err, out)
		}
		return out
	}
	must("ticket", "create", "--title", "base")
	must("ticket", "create", "--title", "filho", "--dep", "TKT-001")
	must("ticket", "cancel", "TKT-001")
	out, err := runTB(t, bin, dir, "router", "run", "--allow-unsandboxed", "--agent-cmd", "sh -c 'true'")
	if err == nil || !strings.Contains(out, "TKT-002") || !strings.Contains(out, "stalled") {
		t.Fatalf("stalled deveria sair != 0 listando TKT-002: %v\n%s", err, out)
	}
}
