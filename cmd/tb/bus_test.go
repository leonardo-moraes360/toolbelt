package main

import (
	"bufio"
	"encoding/json"
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
	if string(before) != string(after) {
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
