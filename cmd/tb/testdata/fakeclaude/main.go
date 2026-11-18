// fakeclaude mimics `claude -p ... --output-format stream-json`: it starts the
// MCP server from --mcp-config, reads the queue with tb_bus_recv, claims and
// completes the task.assign thread and prints events with session_id. It logs
// argv to ./fakeclaude.log for the test to inspect.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type mcpConfig struct {
	MCPServers map[string]struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	} `json:"mcpServers"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fakeclaude:", err)
		os.Exit(1)
	}
}

func flag(args []string, name string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return ""
}

func run() error {
	args := os.Args[1:]
	logLine, _ := json.Marshal(args)
	f, err := os.OpenFile("fakeclaude.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	fmt.Fprintf(f, "%s\n", logLine)
	f.Close()

	raw, err := os.ReadFile(flag(args, "--mcp-config"))
	if err != nil {
		return err
	}
	var cfg mcpConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return err
	}
	srv := cfg.MCPServers["toolbelt"]
	cmd := exec.Command(srv.Command, srv.Args...)
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() { in.Close(); _ = cmd.Wait() }()
	rd := bufio.NewReader(out)
	id := 0
	call := func(tool string, a map[string]any) (string, error) {
		id++
		req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call",
			"params": map[string]any{"name": tool, "arguments": a}})
		fmt.Fprintf(in, "%s\n", req)
		line, err := rd.ReadBytes('\n')
		if err != nil {
			return "", err
		}
		var resp struct {
			Result struct {
				Content []struct{ Text string } `json:"content"`
				IsError bool                    `json:"isError"`
			} `json:"result"`
		}
		if err := json.Unmarshal(line, &resp); err != nil || len(resp.Result.Content) == 0 {
			return "", fmt.Errorf("resposta MCP: %s", line)
		}
		if resp.Result.IsError {
			return "", fmt.Errorf("%s: %s", tool, resp.Result.Content[0].Text)
		}
		return resp.Result.Content[0].Text, nil
	}
	text, err := call("tb_bus_recv", map[string]any{})
	if err != nil {
		return err
	}
	var envs []struct {
		Params struct {
			Type   string `json:"type"`
			Thread string `json:"thread"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(text), &envs); err != nil {
		return err
	}
	session := "fake-session-1"
	if r := flag(args, "--resume"); r != "" {
		session = r + "+"
	}
	fmt.Printf(`{"type":"system","subtype":"init","session_id":%q}`+"\n", session)
	for _, e := range envs {
		if e.Params.Type != "task.assign" || e.Params.Thread == "" {
			continue
		}
		if _, err := call("tb_bus_claim", map[string]any{"thread": e.Params.Thread}); err != nil {
			return err
		}
		if _, err := call("tb_bus_complete", map[string]any{"thread": e.Params.Thread,
			"result": map[string]any{"verdict": "PASS", "by": "fakeclaude"}}); err != nil {
			return err
		}
	}
	fmt.Printf(`{"type":"result","session_id":%q,"result":%q}`+"\n", session, strings.Repeat("ok", 1))
	return nil
}
