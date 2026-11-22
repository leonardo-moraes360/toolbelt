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

type routerCLI struct {
	t   *testing.T
	bin string
	dir string
}

func (c *routerCLI) must(args ...string) string {
	c.t.Helper()
	out, err := runTB(c.t, c.bin, c.dir, args...)
	if err != nil {
		c.t.Fatalf("tb %v: %v\n%s", args, err, out)
	}
	return out
}

func (c *routerCLI) run(args ...string) (string, error) {
	c.t.Helper()
	return runTB(c.t, c.bin, c.dir, args...)
}

func TestRouterWatchdogCLI(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	c := &routerCLI{t: t, bin: buildTB(t), dir: routerTestRepo(t)}
	c.must("ticket", "create", "--title", "base")
	c.must("ticket", "create", "--title", "filho", "--dep", "TKT-001")
	c.must("ticket", "create", "--title", "livre")

	c.checkEmptyState()
	c.checkConfig()
	c.checkDryRun()
	c.checkRealRun()
	c.checkCrashAlerts()
}

// With no state: empty status/alerts and kill is refused.
func (c *routerCLI) checkEmptyState() {
	t := c.t
	if out := c.must("router", "status"); !strings.Contains(out, "No router") {
		t.Fatalf("status vazio: %s", out)
	}
	if out := c.must("watchdog", "alerts"); !strings.Contains(out, "No alerts") {
		t.Fatalf("alerts vazio: %s", out)
	}
	if out, err := c.run("watchdog", "kill", "w1", "--reason", "x"); err == nil || !strings.Contains(out, "no router") {
		t.Fatalf("kill sem router deveria falhar: %v %s", err, out)
	}
}

func (c *routerCLI) checkConfig() {
	t := c.t
	var rc map[string]any
	if err := json.Unmarshal([]byte(c.must("router", "config", "--json")), &rc); err != nil || rc["max_workers"] != float64(3) || rc["strategy"] != "dependency" {
		t.Fatalf("config: %v %v", err, rc)
	}
	if out := c.must("router", "config"); !strings.Contains(out, "max_workers: 3") || !strings.Contains(out, "subscription_ttl: 30m") {
		t.Fatalf("config texto: %s", out)
	}
}

// Dry-run executes nothing; without agent_cmd the error is explicit.
func (c *routerCLI) checkDryRun() {
	t := c.t
	out := c.must("router", "run", "--dry-run", "--workers", "2")
	if !strings.Contains(out, "Round 1") || !strings.Contains(out, "Round 2") || !strings.Contains(out, "TKT-002") {
		t.Fatalf("dry-run: %s", out)
	}
	var plan map[string]any
	if err := json.Unmarshal([]byte(c.must("router", "run", "--dry-run", "--json")), &plan); err != nil || len(plan["rounds"].([]any)) != 2 {
		t.Fatalf("dry-run json: %v %v", err, plan)
	}
	if out := c.must("ticket", "show", "TKT-001"); !strings.Contains(out, "pending") {
		t.Fatalf("dry-run não deveria alterar tickets: %s", out)
	}
	if out, err := c.run("router", "run", "--allow-unsandboxed"); err == nil || !strings.Contains(out, "agent_cmd") {
		t.Fatalf("sem agent-cmd deveria falhar: %v %s", err, out)
	}
}

// Real run: ticket completes and router/watchdog status is reported.
func (c *routerCLI) checkRealRun() {
	t := c.t
	var sum map[string]any
	raw := c.must("router", "run", "--workers", "2", "--allow-unsandboxed", "--json",
		"--agent-cmd", `sh -c 'echo feito-$TB_TICKET_ID > saida.txt'`)
	if err := json.Unmarshal([]byte(raw), &sum); err != nil {
		t.Fatalf("summary json: %v\n%s", err, raw)
	}
	if sum["reason"] != "completed" || len(sum["done"].([]any)) != 3 {
		t.Fatalf("summary: %v", sum)
	}
	var tk map[string]any
	if err := json.Unmarshal([]byte(c.must("ticket", "show", "TKT-002", "--json")), &tk); err != nil {
		t.Fatal(err)
	}
	res := tk["result"].(map[string]any)
	if tk["status"] != "done" || res["head_commit"] == "" || res["branch"] != "tb-workspace/TKT-002" {
		t.Fatalf("TKT-002: %v", tk)
	}

	var st map[string]any
	if err := json.Unmarshal([]byte(c.must("router", "status", "--json")), &st); err != nil || st["status"] != "stopped" || st["stale"] != false {
		t.Fatalf("status: %v %v", err, st)
	}
	if out := c.must("watchdog", "status", "--worker", "w1"); !strings.Contains(out, "w1") || strings.Contains(out, "w2") {
		t.Fatalf("watchdog status: %s", out)
	}
	if out, err := c.run("watchdog", "status", "--worker", "w9"); err == nil {
		t.Fatalf("worker inexistente deveria falhar: %s", out)
	}
}

// An agent failure raises a crash alert visible through the CLI.
func (c *routerCLI) checkCrashAlerts() {
	t := c.t
	c.must("ticket", "create", "--title", "quebra")
	out, err := c.run("router", "run", "--workers", "1", "--allow-unsandboxed", "--agent-cmd", "sh -c 'exit 7'")
	if err == nil || !strings.Contains(out, "TKT-004") {
		t.Fatalf("falha deveria dar exit != 0: %v %s", err, out)
	}
	var alertsDoc struct {
		Alerts []map[string]any `json:"alerts"`
	}
	if err := json.Unmarshal([]byte(c.must("watchdog", "alerts", "--kind", "worker_crash", "--json")), &alertsDoc); err != nil || len(alertsDoc.Alerts) != 2 {
		t.Fatalf("alerts: %v %v", err, alertsDoc)
	}
	alerts := alertsDoc.Alerts
	if alerts[0]["action"] != "retry" || alerts[1]["action"] != "escalate" || alerts[1]["ticket"] != "TKT-004" {
		t.Fatalf("alerts: %v", alerts)
	}
	if out := c.must("watchdog", "alerts", "--since", "1h", "--limit", "1"); !strings.Contains(out, "worker_crash") {
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
