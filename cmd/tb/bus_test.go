package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
)

func runBus(t *testing.T, tbBin, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(tbBin, append([]string{"--json", "bus"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("tb bus %v: %v\n%s%s", args, err, out, stderr)
	}
	return out
}

type mcpClient struct {
	t   *testing.T
	in  io.WriteCloser
	out *bufio.Reader
	id  int
}

func (c *mcpClient) call(tool string, args map[string]any) (string, bool) {
	c.t.Helper()
	c.id++
	req := map[string]any{"jsonrpc": "2.0", "id": c.id, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args}}
	b, _ := json.Marshal(req)
	if _, err := fmt.Fprintf(c.in, "%s\n", b); err != nil {
		c.t.Fatal(err)
	}
	line, err := c.out.ReadBytes('\n')
	if err != nil {
		c.t.Fatalf("lendo resposta MCP: %v", err)
	}
	var resp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(line, &resp); err != nil || len(resp.Result.Content) == 0 {
		c.t.Fatalf("resposta MCP inválida: %s (%v)", line, err)
	}
	return resp.Result.Content[0].Text, resp.Result.IsError
}

func (c *mcpClient) must(tool string, args map[string]any) string {
	c.t.Helper()
	out, isErr := c.call(tool, args)
	if isErr {
		c.t.Fatalf("%s: %s", tool, out)
	}
	return out
}

func unmarshalOut(t *testing.T, out []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(out, v); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, out)
	}
}

// busCLISetup creates channel fx via the CLI, joins the owner and returns the worker's invite.
func busCLISetup(t *testing.T, tbBin, dir string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	var created struct {
		Invite struct{ Code string } `json:"invite"`
	}
	unmarshalOut(t, runBus(t, tbBin, dir, "channel", "create", "fx", "--owner", "claude-main", "--ttl", "1h"), &created)
	var workerInv struct{ Code string }
	unmarshalOut(t, runBus(t, tbBin, dir, "invite", "fx", "--role", "worker"), &workerInv)
	joinOut := runBus(t, tbBin, dir, "join", "fx", "--invite", created.Invite.Code, "--name", "claude-main", "--vendor", "anthropic")
	agentDir, err := bus.AgentDir(dir, "fx", "claude-main")
	if err != nil {
		t.Fatal(err)
	}
	token, err := os.ReadFile(filepath.Join(agentDir, "token"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(joinOut), strings.TrimSpace(string(token))) {
		t.Fatal("join imprimiu o token de sessão")
	}
	return workerInv.Code
}

func startMCP(t *testing.T, tbBin, dir string, args ...string) *mcpClient {
	t.Helper()
	mcp := exec.Command(tbBin, append([]string{"mcp"}, args...)...)
	mcp.Dir = dir
	in, err := mcp.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	outPipe, err := mcp.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := mcp.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Close(); _ = mcp.Wait() })
	return &mcpClient{t: t, in: in, out: bufio.NewReader(outPipe)}
}

type busEnvelopeOut struct {
	Params struct {
		Type string `json:"type"`
		From string `json:"from"`
	} `json:"params"`
}

// Owner via CLI (one process per command) and worker via `tb mcp` (long-lived
// stdio process) exchange messages through the same store.
func TestBusCLIToMCPEndToEnd(t *testing.T) {
	tbBin := buildTB(t)
	dir := t.TempDir()
	workerCode := busCLISetup(t, tbBin, dir)
	c := startMCP(t, tbBin, dir)
	c.must("tb_bus_join", map[string]any{"channel": "fx", "invite": workerCode, "name": "grok-1", "vendor": "xai", "wake": "notify"})

	// Worker long-polls; the owner sends from another process while it waits.
	done := make(chan string, 1)
	go func() {
		out, isErr := c.call("tb_bus_recv", map[string]any{"wait": "10s"})
		if isErr {
			out = "ERRO: " + out
		}
		done <- out
	}()
	time.Sleep(300 * time.Millisecond)
	runBus(t, tbBin, dir, "send", "--channel", "fx", "--as", "claude-main", "--to", "agent:grok-1",
		"--type", "task.assign", "--thread", "TKT-001", "--payload", `{"ticket":"TKT-001"}`)
	var got string
	select {
	case got = <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("long-poll do MCP não recebeu a mensagem da CLI")
	}
	if !strings.Contains(got, `"task.assign"`) || !strings.Contains(got, `"agent:claude-main"`) {
		t.Fatalf("recv MCP: %s", got)
	}

	c.must("tb_bus_claim", map[string]any{"thread": "TKT-001"})
	c.must("tb_bus_complete", map[string]any{"thread": "TKT-001", "epoch": 1, "result": map[string]any{"verdict": "PASS", "commit": "a1b2c3d"}})

	var envs []busEnvelopeOut
	unmarshalOut(t, runBus(t, tbBin, dir, "recv", "--channel", "fx", "--as", "claude-main"), &envs)
	if len(envs) != 1 || envs[0].Params.Type != "task.result" || envs[0].Params.From != "agent:grok-1" {
		t.Fatalf("dono deveria receber task.result do worker: %+v", envs)
	}
	unmarshalOut(t, runBus(t, tbBin, dir, "log", "fx"), &envs)
	if len(envs) != 2 {
		t.Fatalf("log: %+v", envs)
	}
}

func TestBusChannelExtendCLI(t *testing.T) {
	tbBin := buildTB(t)
	dir := t.TempDir()
	busCLISetup(t, tbBin, dir)
	var ch struct {
		Name      string     `json:"name"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	unmarshalOut(t, runBus(t, tbBin, dir, "channel", "extend", "fx", "--ttl", "1h"), &ch)
	if ch.Name != "fx" || ch.ExpiresAt == nil || time.Until(*ch.ExpiresAt) < 50*time.Minute {
		t.Fatalf("extend 1h: %+v", ch)
	}
	ch.ExpiresAt = nil
	unmarshalOut(t, runBus(t, tbBin, dir, "channel", "extend", "fx", "--ttl", "0"), &ch)
	if ch.Name != "fx" {
		t.Fatalf("extend 0: %+v", ch)
	}
	if ch.ExpiresAt != nil {
		t.Fatalf("extend 0 deveria remover expiração: %+v", ch)
	}
	cmd := exec.Command(tbBin, "bus", "channel", "extend", "fx")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "--ttl is required") {
		t.Fatalf("sem --ttl: %v %s", err, out)
	}
}

// AC12: channel show (JSON and text) and watchdog output carry no Agent
// Card session, token, invite code or payload.
func TestBusStatusNoSecretsCLI(t *testing.T) {
	tbBin := buildTB(t)
	dir := t.TempDir()
	workerCode := busCLISetup(t, tbBin, dir)
	runBus(t, tbBin, dir, "join", "fx", "--invite", workerCode, "--name", "grok-1", "--role", "worker", "--session", "resume-handle-xyz")
	runBus(t, tbBin, dir, "send", "--channel", "fx", "--as", "claude-main", "--to", "agent:grok-1", "--type", "query", "--payload", `{"secret":"payload-xyz"}`)
	runBus(t, tbBin, dir, "claim", "T1", "--channel", "fx", "--as", "grok-1")
	agentDir, err := bus.AgentDir(dir, "fx", "grok-1")
	if err != nil {
		t.Fatal(err)
	}
	token, err := os.ReadFile(filepath.Join(agentDir, "token"))
	if err != nil {
		t.Fatal(err)
	}
	text := exec.Command(tbBin, "bus", "channel", "show", "fx")
	text.Dir = dir
	textOut, err := text.Output()
	if err != nil {
		t.Fatal(err)
	}
	outs := map[string][]byte{
		"show --json":     runBus(t, tbBin, dir, "channel", "show", "fx"),
		"show":            textOut,
		"watchdog --json": runBus(t, tbBin, dir, "watchdog", "fx", "--dry-run"),
	}
	for name, out := range outs {
		for _, secret := range []string{strings.TrimSpace(string(token)), workerCode, "resume-handle-xyz", "payload-xyz"} {
			if strings.Contains(string(out), secret) {
				t.Fatalf("%s leaks a secret:\n%s", name, out)
			}
		}
	}
	if !strings.Contains(string(outs["show --json"]), `"T1"`) {
		t.Fatalf("show lacks the claim:\n%s", outs["show --json"])
	}
}

// AC12 (bus-notify-discover): a GUI agent joined from the CLI and bound with
// tb mcp --bus-as reads over MCP, and tb_bus_join on that connection fails
// without replacing the token the harness uses.
func TestBusGUIOnboardingBusAs(t *testing.T) {
	tbBin := buildTB(t)
	dir := t.TempDir()
	workerCode := busCLISetup(t, tbBin, dir)
	agentDir, err := bus.AgentDir(dir, "fx", "claude-main")
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(agentDir, "token"))
	if err != nil {
		t.Fatal(err)
	}
	c := startMCP(t, tbBin, dir, "--bus-as", "fx/claude-main", "--bus-root", dir)
	c.must("tb_bus_recv", map[string]any{})
	for _, args := range []map[string]any{
		{"channel": "fx", "invite": workerCode, "name": "claude-main", "cwd": dir},
		{"channel": "fx", "invite": workerCode, "name": "other", "cwd": dir},
	} {
		if out, isErr := c.call("tb_bus_join", args); !isErr {
			t.Fatalf("tb_bus_join on a bound connection succeeded: %s", out)
		}
	}
	after, err := os.ReadFile(filepath.Join(agentDir, "token"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("the CLI session token changed")
	}
	c.must("tb_bus_recv", map[string]any{})
}

// AC13 (bus-notify-discover): discover lists known CLIs and runs none.
func TestBusDiscoverCLI(t *testing.T) {
	tbBin := buildTB(t)
	bin, home := t.TempDir(), t.TempDir()
	ran := filepath.Join(t.TempDir(), "ran")
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho x > "+ran+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(tbBin, "--json", "bus", "discover")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "PATH="+bin, "HOME="+home)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	var cat struct {
		V    string `json:"v"`
		CLIs []struct {
			Name string `json:"name"`
		} `json:"clis"`
	}
	unmarshalOut(t, out, &cat)
	if cat.V != "bus-discover/0.1" || len(cat.CLIs) != 1 || cat.CLIs[0].Name != "claude" {
		t.Fatalf("catalog: %s", out)
	}
	if _, err := os.Stat(ran); err == nil {
		t.Fatal("discover executed a CLI")
	}
}

// AC8 (bus-notify-discover): --check with notify on a card that says
// manual prints one warning and exits 0.
func TestBusRunNotifyWakeMismatchCLI(t *testing.T) {
	tbBin := buildTB(t)
	dir := t.TempDir()
	busCLISetup(t, tbBin, dir)
	cfg := filepath.Join(t.TempDir(), "wake.yaml")
	if err := os.WriteFile(cfg, []byte("adapter: notify\nnotify:\n  backend: terminal\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(tbBin, "bus", "run", "--channel", "fx", "--as", "claude-main", "--config", cfg, "--check")
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if out, err := cmd.Output(); err != nil {
		t.Fatalf("--check: %v\n%s%s", err, out, stderr.String())
	}
	if n := strings.Count(stderr.String(), "warning: wake_mismatch (card says manual, wake file says notify)"); n != 1 {
		t.Fatalf("stderr:\n%s", stderr.String())
	}
}

func TestFormatTreeAlignsAndIndents(t *testing.T) {
	got := formatTree([]bus.TreeNode{
		{Thread: "TKT-001", State: "claimed", Owner: "agent:claude-main", Epoch: 1, OpenChildren: 1},
		{Thread: "TKT-001/api", Parent: "TKT-001", Depth: 1, State: "completed", Owner: "agent:codex-1", Epoch: 1},
		{Thread: "TKT-001/ui", Parent: "TKT-001", Depth: 1, State: "assigned"},
	})
	want := "TKT-001        claimed    agent:claude-main  epoch 1  open 1\n" +
		"  TKT-001/api  completed  agent:codex-1      epoch 1\n" +
		"  TKT-001/ui   assigned\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	sub := formatTree([]bus.TreeNode{{Thread: "A/b", Parent: "A", Depth: 1, State: "assigned"}})
	if strings.HasPrefix(sub, " ") {
		t.Fatalf("a --thread subtree starts at column 0: %q", sub)
	}
}

// send --parent, tree (text and JSON) and the complete warning, through the CLI.
func TestBusCLIDelegationTree(t *testing.T) {
	tbBin := buildTB(t)
	dir := t.TempDir()
	busCLISetup(t, tbBin, dir)
	send := func(args ...string) {
		runBus(t, tbBin, dir, append([]string{"send", "--channel", "fx", "--as", "claude-main", "--to", "role:worker", "--type", "task.assign"}, args...)...)
	}
	send("--thread", "TKT-001", "--payload", `{"note":"CLI-SECRET"}`)
	runBus(t, tbBin, dir, "claim", "--channel", "fx", "--as", "claude-main", "TKT-001")
	send("--thread", "TKT-001/api", "--parent", "TKT-001")

	text := exec.Command(tbBin, "bus", "tree", "fx")
	text.Dir = dir
	out, err := text.CombinedOutput()
	if err != nil {
		t.Fatalf("bus tree: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "TKT-001") || !strings.Contains(string(out), "  TKT-001/api") || strings.Contains(string(out), "CLI-SECRET") {
		t.Fatalf("tree text:\n%s", out)
	}
	var tr bus.TreeResult
	unmarshalOut(t, runBus(t, tbBin, dir, "tree", "fx", "--thread", "TKT-001"), &tr)
	if len(tr.Nodes) != 2 || tr.Nodes[1].Parent != "TKT-001" || tr.Nodes[0].OpenChildren != 1 {
		t.Fatalf("tree json: %+v", tr)
	}

	bad := exec.Command(tbBin, "bus", "send", "--channel", "fx", "--as", "claude-main", "--to", "role:worker",
		"--type", "task.assign", "--thread", "X", "--parent", "NOPE")
	bad.Dir = dir
	if out, err := bad.CombinedOutput(); err == nil || !strings.Contains(string(out), "parent_unknown") {
		t.Fatalf("unknown parent: %v\n%s", err, out)
	}

	done := exec.Command(tbBin, "bus", "complete", "--channel", "fx", "--as", "claude-main", "--epoch", "1", "TKT-001")
	done.Dir = dir
	var stderr bytes.Buffer
	done.Stderr = &stderr
	if err := done.Run(); err != nil {
		t.Fatalf("complete: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "warning: TKT-001 completed with 1 open children (TKT-001/api)") {
		t.Fatalf("stderr: %q", stderr.String())
	}
}

// One live client per bus identity: a second `tb mcp --bus-as`, a `tb bus run`
// and a re-join with a bound invite are all refused while the first is up.
func TestBusIdentityHeldByOneClient(t *testing.T) {
	tbBin := buildTB(t)
	dir := t.TempDir()
	busCLISetup(t, tbBin, dir)
	first := exec.Command(tbBin, "mcp", "--bus-as", "fx/claude-main", "--bus-root", dir)
	first.Dir = dir
	in, err := first.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	outPipe, err := first.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	c := &mcpClient{t: t, in: in, out: bufio.NewReader(outPipe)}
	c.must("tb_bus_recv", map[string]any{})
	pid := fmt.Sprintf("pid %d", first.Process.Pid)

	refused := func(name string, cmd *exec.Cmd) {
		t.Helper()
		cmd.Dir = dir
		var stderr strings.Builder
		cmd.Stderr = &stderr
		err := cmd.Run()
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("%s: want an exit error, got %v", name, err)
		}
		if ee.ExitCode() != 2 {
			t.Fatalf("%s: exit %d, want 2\n%s", name, ee.ExitCode(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "session_in_use") || (name != "dead harness pid" && !strings.Contains(stderr.String(), pid)) {
			t.Fatalf("%s: stderr must name session_in_use and %q:\n%s", name, pid, stderr.String())
		}
	}
	refused("second mcp", exec.Command(tbBin, "mcp", "--bus-as", "fx/claude-main", "--bus-root", dir))
	wake := filepath.Join(t.TempDir(), "wake.yaml")
	if err := os.WriteFile(wake, []byte("adapter: notify\nnotify:\n  backend: terminal\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	refused("bus run", exec.Command(tbBin, "bus", "run", "--channel", "fx", "--as", "claude-main", "--config", wake, "--check"))
	var inv struct{ Code string }
	unmarshalOut(t, runBus(t, tbBin, dir, "invite", "fx", "--role", "supervisor", "--name", "claude-main"), &inv)
	refused("re-join", exec.Command(tbBin, "bus", "join", "fx", "--invite", inv.Code, "--name", "claude-main"))

	// A harness pid that is gone no longer vouches for the identity.
	refused("dead harness pid", exec.Command(tbBin, "mcp", "--bus-as", "fx/claude-main", "--bus-root", dir, "--bus-holder-pid", "2147483646"))

	// A crashed holder leaves the identity free.
	_ = first.Process.Kill()
	_ = first.Wait()
	second := startMCP(t, tbBin, dir, "--bus-as", "fx/claude-main", "--bus-root", dir)
	second.must("tb_bus_recv", map[string]any{})
}
