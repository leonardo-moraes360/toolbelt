//go:build unix

package busrun

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
)

type taskEnv struct {
	*env
	router bus.Member
	script string
	ran    string
}

// newTaskEnv: a router member, a ticket TKT-001 with its workspace, and an
// exec child script with the given body (run in the workspace).
func newTaskEnv(t *testing.T, body string) *taskEnv {
	t.Helper()
	e := newEnv(t)
	inv, err := e.store.CreateInvite("ch", bus.RoleRouter, "router", 0)
	if err != nil {
		t.Fatal(err)
	}
	r, err := e.store.Join("ch", inv.Code, &bus.AgentCard{Name: "router"}, bus.RoleRouter)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{".toolbelt/workspaces/TKT-001", ".toolbelt/tickets"} {
		if err := os.MkdirAll(filepath.Join(e.root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(e.root, ".toolbelt", "tickets", "TKT-001.yaml"), []byte("id: TKT-001\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	// The counter lives in the workspace: the only place a confined child writes.
	te := &taskEnv{env: e, router: r.Member, script: filepath.Join(bin, "task.sh"),
		ran: filepath.Join(e.root, ".toolbelt", "workspaces", "TKT-001", "ran")}
	full := "#!/bin/sh\necho run >> " + te.ran + "\n" + body
	if err := os.WriteFile(te.script, []byte(full), 0o755); err != nil {
		t.Fatal(err)
	}
	e.writeConfig(t, "adapter: exec\nexec:\n  argv: ["+te.script+"]\nrouter_tasks:\n  from: router\ncoalesce:\n  debounce: 20ms\n  max_delay: 1s\n")
	return te
}

func (te *taskEnv) task(thread string) RouterTask {
	return RouterTask{V: RouterTaskVersion, Ticket: "TKT-001", Thread: thread, Attempt: 1, Title: "t",
		Workspace:  filepath.Join(te.root, ".toolbelt", "workspaces", "TKT-001"),
		TicketFile: filepath.Join(te.root, ".toolbelt", "tickets", "TKT-001.yaml"), Timeout: "1m"}
}

func (te *taskEnv) assign(t *testing.T, from bus.Member, task *RouterTask) {
	t.Helper()
	b, _ := json.Marshal(task)
	if _, err := te.store.Send(from, &bus.SendInput{To: "role:worker", Type: bus.TypeTaskAssign, Thread: task.Thread, Payload: b}); err != nil {
		t.Fatal(err)
	}
}

func (te *taskEnv) openH(t *testing.T, sandboxed bool) *Harness {
	t.Helper()
	h, err := Open(&Options{Root: te.root, Channel: "ch", Name: "w", Poll: 10 * time.Millisecond,
		TaskPoll: 50 * time.Millisecond, AllowUnsandboxed: !sandboxed, unconfinedTasks: !sandboxed})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return h
}

// runUntil runs the harness until thread is completed or revoked, or 10s.
func (te *taskEnv) runUntil(t *testing.T, h *Harness, thread string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.Run(ctx, false) }()
	for ctx.Err() == nil {
		if c, err := te.store.Claim("ch", thread); err == nil && (c.CompletedAt != nil || c.RevokedAt != nil) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond) // let the child exit and the audit line land
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func (te *taskEnv) runs() int {
	b, _ := os.ReadFile(te.ran)
	return strings.Count(string(b), "run")
}

func (te *taskEnv) verdict(t *testing.T, thread string) (bus.Claim, Verdict) {
	t.Helper()
	c, err := te.store.Claim("ch", thread)
	if err != nil {
		t.Fatal(err)
	}
	var v Verdict
	if c.ResultID != "" {
		m, err := te.store.Message("ch", c.ResultID)
		if err != nil {
			t.Fatal(err)
		}
		_ = json.Unmarshal(m.Payload, &v)
	}
	return c, v
}

// AC6: the harness claims confined, the child edits the workspace and
// writes PASS, and the harness completes with it.
func TestRouterTaskHappyPath(t *testing.T) {
	te := newTaskEnv(t, "echo hi > out.txt\nprintf '{\"verdict\":\"PASS\",\"summary\":\"ok\"}' > \"$TB_TASK_VERDICT\"\n")
	task := te.task("TKT-001/abc.1")
	te.assign(t, te.router, &task)
	h := te.openH(t, landlockOn())
	te.runUntil(t, h, task.Thread)
	c, v := te.verdict(t, task.Thread)
	if c.Owner != "agent:w" || c.Confinement != bus.ConfinementMarker || c.CompletedAt == nil || v.Verdict != "PASS" || v.Summary != "ok" {
		t.Fatalf("claim %+v verdict %+v\n%s", c, v, readAudit(t, te.env))
	}
	if _, err := os.Stat(filepath.Join(task.Workspace, "out.txt")); err != nil {
		t.Fatalf("workspace not edited: %v", err)
	}
	if a := readAudit(t, te.env); !strings.Contains(a, "router_task_ok") || strings.Contains(a, `"summary"`) {
		t.Fatalf("audit:\n%s", a)
	}
	// AC13: no rewake and no second run, even after a restart.
	h.Close()
	runFor(t, te.openH(t, landlockOn()), 300*time.Millisecond)
	if n := te.runs(); n != 1 {
		t.Fatalf("task ran %d times", n)
	}
}

// AC7: a worker that loses the claim never runs the child.
func TestRouterTaskLost(t *testing.T) {
	te := newTaskEnv(t, "printf '{\"verdict\":\"PASS\"}' > \"$TB_TASK_VERDICT\"\n")
	task := te.task("TKT-001/abc.1")
	te.assign(t, te.router, &task)
	inv, _ := te.store.CreateInvite("ch", bus.RoleWorker, "", 0)
	other, err := te.store.Join("ch", inv.Code, &bus.AgentCard{Name: "other"}, bus.RoleWorker)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := te.store.ClaimThread(other.Member, task.Thread); err != nil {
		t.Fatal(err)
	}
	runFor(t, te.openH(t, false), 500*time.Millisecond)
	if te.runs() != 0 || !strings.Contains(readAudit(t, te.env), "router_task_lost") {
		t.Fatalf("runs=%d\n%s", te.runs(), readAudit(t, te.env))
	}
}

// AC10: a revoke kills the child and no complete follows.
func TestRouterTaskRevoked(t *testing.T) {
	te := newTaskEnv(t, "sleep 5\nprintf '{\"verdict\":\"PASS\"}' > \"$TB_TASK_VERDICT\"\n")
	task := te.task("TKT-001/abc.1")
	te.assign(t, te.router, &task)
	h := te.openH(t, false)
	go func() {
		for deadline := time.Now().Add(5 * time.Second); te.runs() == 0 && time.Now().Before(deadline); {
			time.Sleep(10 * time.Millisecond)
		}
		_, _ = te.store.Revoke(te.router, task.Thread, "timeout")
	}()
	start := time.Now()
	te.runUntil(t, h, task.Thread)
	time.Sleep(300 * time.Millisecond) // the watcher polls every 50ms
	c, _ := te.verdict(t, task.Thread)
	if c.CompletedAt != nil || c.RevokedAt == nil || !strings.Contains(readAudit(t, te.env), "router_task_revoked") {
		t.Fatalf("claim %+v\n%s", c, readAudit(t, te.env))
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("the child was not killed promptly")
	}
}

// AC11: bad verdicts complete as FAIL with fixed harness text.
func TestRouterTaskBadVerdicts(t *testing.T) {
	cases := map[string]string{
		"":                                   "no verdict file",
		"echo nope > \"$TB_TASK_VERDICT\"\n": "invalid verdict file",
		"printf '{\"verdict\":\"pass\"}' > \"$TB_TASK_VERDICT\"\n":         "invalid verdict file",
		"head -c 5000 /dev/zero > \"$TB_TASK_VERDICT\"\n":                  "no verdict file",
		"ln -s /etc/hostname \"$TB_TASK_VERDICT\"\n":                       "no verdict file",
		"printf '{\"verdict\":\"PASS\"}' > \"$TB_TASK_VERDICT\"\nexit 3\n": "the child exited 3",
	}
	for body, want := range cases {
		te := newTaskEnv(t, body)
		task := te.task("TKT-001/bad.1")
		te.assign(t, te.router, &task)
		te.runUntil(t, te.openH(t, false), task.Thread)
		if _, v := te.verdict(t, task.Thread); v.Verdict != "FAIL" || v.Summary != want {
			t.Errorf("%q: %+v", body, v)
		}
	}
}

// AC12: anything that is not a valid router task is an ordinary message.
func TestRouterTaskValidation(t *testing.T) {
	te := newTaskEnv(t, "")
	h := te.openH(t, false)
	roles, err := h.roles()
	if err != nil {
		t.Fatal(err)
	}
	ok := te.task("TKT-001/v.1")
	msg := func(from string, task *RouterTask, thread string) *bus.Message {
		b, _ := json.Marshal(task)
		return &bus.Message{ID: "m", Type: bus.TypeTaskAssign, From: from, Thread: thread, Payload: b}
	}
	if _, valid := h.validTask(msg("agent:router", &ok, ok.Thread), roles); !valid {
		t.Fatal("a valid task was refused")
	}
	inv, _ := te.store.CreateInvite("ch", bus.RoleRouter, "router2", 0)
	if _, err := te.store.Join("ch", inv.Code, &bus.AgentCard{Name: "router2"}, bus.RoleRouter); err != nil {
		t.Fatal(err)
	}
	if roles, err = h.roles(); err != nil {
		t.Fatal(err)
	}
	bad := ok
	bad.Ticket, bad.Workspace = "../x", filepath.Join(te.root, ".toolbelt", "workspaces", "..", "x")
	link := ok
	link.Ticket = "TKT-002"
	link.Workspace = filepath.Join(te.root, ".toolbelt", "workspaces", "TKT-002")
	link.TicketFile = filepath.Join(te.root, ".toolbelt", "tickets", "TKT-002.yaml")
	_ = os.Symlink(t.TempDir(), link.Workspace)
	_ = os.WriteFile(link.TicketFile, []byte("x"), 0o644)
	for name, m := range map[string]*bus.Message{
		"supervisor":      msg("agent:boss", &ok, ok.Thread),
		"worker":          msg("agent:w", &ok, ok.Thread),
		"other router":    msg("agent:router2", &ok, ok.Thread),
		"ticket ../x":     msg("agent:router", &bad, bad.Thread),
		"symlinked space": msg("agent:router", &link, link.Thread),
		"thread mismatch": msg("agent:router", &ok, "TKT-001/other.1"),
	} {
		if _, valid := h.validTask(m, roles); valid {
			t.Errorf("%s accepted", name)
		}
	}
}

// AC9: sandbox entries that would void the confinement are refused.
func TestRouterTaskSandboxEntries(t *testing.T) {
	te := newTaskEnv(t, "")
	h := te.openH(t, false)
	home := filepath.Dir(te.root)
	for _, w := range []string{te.root, home, filepath.Join(te.root, ".toolbelt")} {
		h.cfg.Sandbox.Write = []string{w}
		if err := h.taskSandboxOK(); err == nil {
			t.Errorf("sandbox.write %s accepted", w)
		}
	}
	h.cfg.Sandbox.Write = []string{t.TempDir()}
	if err := h.taskSandboxOK(); err != nil {
		t.Fatalf("unrelated path refused: %v", err)
	}
}

// AC8: under Landlock the child writes the workspace and reads the ticket
// copy, but cannot write the repository, .git or bus.db.
func TestRouterTaskGrants(t *testing.T) {
	if !landlockOn() {
		t.Skip("needs Landlock")
	}
	probe := "probe() { if ( eval \"$1\" ) 2>/dev/null; then echo \"$2 ok\" >> rep; else echo \"$2 denied\" >> rep; fi; }\n" +
		"probe \": > ../../../x\" root\nprobe \"echo x >> ../../../.git/config\" gitconfig\n" +
		"probe \"echo x >> ../../bus/bus.db\" busdb\nprobe \"cat ../../bus/bus.db\" busread\n" +
		"probe \"echo x >> ../../tickets/TKT-001.yaml\" liveticket\nprobe \"cat $TB_TASK_TICKET_FILE\" ticket\n" +
		"printf '{\"verdict\":\"PASS\"}' > \"$TB_TASK_VERDICT\"\n"
	te := newTaskEnv(t, probe)
	if err := os.MkdirAll(filepath.Join(te.root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(te.root, ".git", "config"), nil, 0o644)
	task := te.task("TKT-001/g.1")
	te.assign(t, te.router, &task)
	te.runUntil(t, te.openH(t, true), task.Thread)
	rep, err := os.ReadFile(filepath.Join(task.Workspace, "rep"))
	if err != nil {
		t.Fatalf("no report: %v\n%s", err, readAudit(t, te.env))
	}
	for _, want := range []string{"root denied", "gitconfig denied", "busdb denied", "busread denied", "liveticket denied", "ticket ok"} {
		if !strings.Contains(string(rep), want) {
			t.Fatalf("report:\n%s", rep)
		}
	}
}

// AC14: the claude preset gets a fresh session and no bus tools.
func TestRouterTaskClaudeArgv(t *testing.T) {
	te := newTaskEnv(t, "")
	te.writeConfig(t, "adapter: harness\nharness:\n  vendor: claude\n  command: /bin/true\n  session: s1\nrouter_tasks:\n  from: router\n")
	h := te.openH(t, false)
	task := te.task("TKT-001/c.1")
	argv := strings.Join(h.taskClaudeArgv(&task, "/m.json", "/t.yaml", "/v.json"), " ")
	if strings.Contains(argv, "--resume") || strings.Contains(argv, "tb_bus") || !strings.Contains(argv, "--strict-mcp-config") ||
		!strings.Contains(argv, "--allowedTools Read,Edit,Write,Glob,Grep") {
		t.Fatalf("argv: %s", argv)
	}
	if _, err := LoadConfig(te.writeConfig(t, "adapter: exec\nexec:\n  argv: [/bin/true]\nrouter_tasks:\n  from: router\n  allowed_tools: [mcp__toolbelt__tb_bus_claim]\n")); err == nil {
		t.Fatal("bus tools accepted for router tasks")
	}
	if _, err := LoadConfig(te.writeConfig(t, "adapter: notify\nnotify:\n  backend: terminal\nrouter_tasks:\n  from: router\n")); err == nil {
		t.Fatal("router_tasks accepted with notify")
	}
}

// A router_tasks harness is a dedicated task worker: ordinary messages never
// wake it, and a flood of invalid assigns cannot starve a valid one.
func TestRouterTaskDedicated(t *testing.T) {
	te := newTaskEnv(t, "printf '{\"verdict\":\"PASS\"}' > \"$TB_TASK_VERDICT\"\n")
	te.send(t, bus.TypeQuery, `{}`)
	for i := range 60 {
		bad := te.task("TKT-001/junk." + strconv.Itoa(i))
		bad.V = "other/0.1"
		te.assign(t, te.router, &bad)
	}
	task := te.task("TKT-001/ok.1")
	te.assign(t, te.router, &task)
	te.runUntil(t, te.openH(t, false), task.Thread)
	if _, v := te.verdict(t, task.Thread); v.Verdict != "PASS" || te.runs() != 1 {
		t.Fatalf("verdict %+v runs %d\n%s", v, te.runs(), readAudit(t, te.env))
	}
}

// Paths are compared canonically: a symlinked root cannot slip a grant on
// the repository past the check.
func TestRouterTaskSymlinkedRoot(t *testing.T) {
	te := newTaskEnv(t, "")
	h := te.openH(t, false)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(te.root, link); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{link, filepath.Join(link, ".git"), filepath.Join(link, ".toolbelt", "bus")} {
		h.cfg.Sandbox.Write = []string{w}
		if err := h.taskSandboxOK(); err == nil {
			t.Errorf("sandbox.write %s accepted", w)
		}
	}
	h.cfg.Sandbox.Write = nil
	h.cfg.Sandbox.Read = []string{filepath.Join(link, ".toolbelt", "bus")}
	if err := h.taskSandboxOK(); err == nil {
		t.Error("sandbox.read inside .toolbelt accepted")
	}
	h.cfg.Sandbox.Read = nil
	h.cfg.Exec.Argv = []string{filepath.Join(te.root, "agent.sh")}
	_ = os.WriteFile(h.cfg.Exec.Argv[0], []byte("#!/bin/sh\n"), 0o755)
	if err := h.taskSandboxOK(); err == nil {
		t.Error("an exec binary in the repository root (read grant on .toolbelt) accepted")
	}
	task := te.task("TKT-001/s.1")
	task.Workspace = filepath.Join(link, ".toolbelt", "workspaces", "TKT-001")
	task.TicketFile = filepath.Join(link, ".toolbelt", "tickets", "TKT-001.yaml")
	if !h.taskPathsOK(&task) || !strings.HasPrefix(task.Workspace, resolvePath(te.root)) {
		t.Fatalf("symlinked but correct paths refused or not canonicalized: %+v", task)
	}
}
