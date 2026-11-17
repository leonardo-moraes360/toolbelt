package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func assertFirstWakeArgv(t *testing.T, dir string) {
	t.Helper()
	argv, err := os.ReadFile(filepath.Join(dir, "fakeclaude.log"))
	if err != nil {
		t.Fatal(err)
	}
	log := string(argv)
	for _, want := range []string{`"-p"`, `"--output-format","stream-json"`, `"--permission-mode","default"`, "mcp__toolbelt__tb_bus_recv"} {
		if !strings.Contains(log, want) {
			t.Errorf("argv sem %s: %s", want, log)
		}
	}
	if strings.Contains(log, "conteudo-privado") || strings.Contains(log, "bypass") || strings.Contains(log, "--resume") {
		t.Fatalf("argv do primeiro despertar: %s", log)
	}
}

func buildFakeClaude(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "claude")
	build := exec.Command("go", "build", "-o", bin, "./cmd/tb/testdata/fakeclaude")
	build.Dir = findModuleRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fakeclaude: %v\n%s", err, out)
	}
	return bin
}

// TestBusRunHarnessClaudeEndToEnd: o harness acorda um "claude" fake, que
// entra pelo MCP com --bus-as, lê a fila, faz claim/complete; o session_id
// volta para o próximo --resume.
func TestBusRunHarnessClaudeEndToEnd(t *testing.T) {
	tbBin := buildTB(t)
	fake := buildFakeClaude(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	var created struct {
		Invite struct{ Code string } `json:"invite"`
	}
	unmarshalOut(t, runBus(t, tbBin, dir, "channel", "create", "fx", "--owner", "boss"), &created)
	var inv struct{ Code string }
	unmarshalOut(t, runBus(t, tbBin, dir, "invite", "fx", "--role", "worker"), &inv)
	runBus(t, tbBin, dir, "join", "fx", "--invite", created.Invite.Code, "--name", "boss")
	runBus(t, tbBin, dir, "send", "--channel", "fx", "--as", "boss", "--to", "role:worker",
		"--type", "task.assign", "--thread", "T-1", "--payload", `{"segredo":"conteudo-privado"}`)

	cfgPath := filepath.Join(dir, ".toolbelt", "bus", "wake", "fx", "claude-w.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := "adapter: harness\nharness:\n  vendor: claude\n  command: " + fake + "\nsandbox:\n  read: [" + filepath.Dir(tbBin) + "]\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func() {
		t.Helper()
		cmd := exec.Command(tbBin, "bus", "run", "--channel", "fx", "--as", "claude-w", "--invite", inv.Code,
			"--role", "worker", "--once", "--allow-unsandboxed")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("tb bus run: %v\n%s", err, out)
		}
	}
	run()

	var envs []busEnvelopeOut
	unmarshalOut(t, runBus(t, tbBin, dir, "log", "fx", "--thread", "T-1"), &envs)
	if len(envs) != 2 || envs[1].Params.Type != "task.result" || envs[1].Params.From != "agent:claude-w" {
		audit, _ := os.ReadFile(filepath.Join(dir, ".toolbelt", "bus", "run", "fx", "claude-w.jsonl"))
		t.Fatalf("thread T-1: %+v\naudit: %s", envs, audit)
	}
	state, err := os.ReadFile(filepath.Join(dir, ".toolbelt", "bus", "run", "fx", "claude-w.state"))
	if err != nil || !strings.Contains(string(state), `"session":"fake-session-1"`) {
		t.Fatalf("session_id não gravado: %s (%v)", state, err)
	}
	assertFirstWakeArgv(t, dir)

	// segundo despertar retoma a sessão
	runBus(t, tbBin, dir, "send", "--channel", "fx", "--as", "boss", "--to", "agent:claude-w", "--type", "task.assign", "--thread", "T-2")
	run()
	argv, _ := os.ReadFile(filepath.Join(dir, "fakeclaude.log"))
	lines := strings.Split(strings.TrimSpace(string(argv)), "\n")
	var second []string
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &second); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(second, " "), "--resume fake-session-1") {
		t.Fatalf("segundo despertar sem --resume: %v", second)
	}
}
