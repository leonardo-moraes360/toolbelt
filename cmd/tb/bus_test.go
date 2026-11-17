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

// busCLISetup cria o canal fx pela CLI, faz join do dono e devolve o convite do worker.
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
	token, err := os.ReadFile(filepath.Join(dir, ".toolbelt", "bus", "sessions", "fx", "claude-main.token"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(joinOut), strings.TrimSpace(string(token))) {
		t.Fatal("join imprimiu o token de sessão")
	}
	return workerInv.Code
}

func startMCP(t *testing.T, tbBin, dir string) *mcpClient {
	t.Helper()
	mcp := exec.Command(tbBin, "mcp")
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

// TestBusCLIToMCPEndToEnd: dono via CLI (um processo por comando) e worker
// via `tb mcp` (processo stdio de longa duração) trocam mensagens no mesmo store.
func TestBusCLIToMCPEndToEnd(t *testing.T) {
	tbBin := buildTB(t)
	dir := t.TempDir()
	workerCode := busCLISetup(t, tbBin, dir)
	c := startMCP(t, tbBin, dir)
	c.must("tb_bus_join", map[string]any{"channel": "fx", "invite": workerCode, "name": "grok-1", "vendor": "xai", "wake": "notify"})

	// worker espera em long-poll; o dono envia de outro processo durante a espera.
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
	c.must("tb_bus_complete", map[string]any{"thread": "TKT-001", "result": map[string]any{"verdict": "PASS", "commit": "a1b2c3d"}})

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
