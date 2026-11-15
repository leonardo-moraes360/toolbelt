package bench

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExpandAgentCommandQuotesPrompt(t *testing.T) {
	prompt := `say "hi"; touch /tmp/x; echo $(whoami)`
	got := ExpandAgentCommand("agent --prompt {prompt} --dir {workdir}", prompt, "/tmp/work", "30s")
	quoted := shellSingleQuote(prompt)
	if !strings.Contains(got, quoted) {
		t.Fatalf("prompt not single-quoted: %s", got)
	}
	if strings.Contains(got, "{prompt}") {
		t.Fatalf("placeholder not replaced: %s", got)
	}
	if !strings.HasPrefix(quoted, "'") || !strings.HasSuffix(quoted, "'") {
		t.Fatalf("expected POSIX single quotes: %q", quoted)
	}
	if !strings.Contains(got, shellSingleQuote("/tmp/work")) {
		t.Fatalf("workdir not quoted: %s", got)
	}
}

func TestShellSingleQuoteEscapesApostrophe(t *testing.T) {
	got := shellSingleQuote(`it's`)
	want := `'it'\''s'`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestExecuteAgentPromptInjectionSafe(t *testing.T) {
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "pwned")
	outFile := filepath.Join(tmp, "prompt.out")
	prompt := fmt.Sprintf(`hello"; touch %s; echo $(echo INJECTED) 'quotes' and more`, marker)

	agent := AgentConfig{
		ID:           "safe-echo",
		Command:      `printf '%s' {prompt} > ` + shellSingleQuote(outFile),
		OutputParser: "none",
	}
	runner := NewRunner(agent, tmp, tmp, tmp, 1)
	task := &Task{
		ID:      "inj",
		Prompt:  prompt,
		Timeout: "10s",
		Verification: Verification{
			Command:          "true",
			ExpectedExitCode: 0,
			Timeout:          "5s",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, exitCode, err := runner.executeAgent(ctx, task, tmp)
	if err != nil {
		t.Fatalf("executeAgent: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exit code %d", exitCode)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("shell injection created marker file")
	}
	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != prompt {
		t.Fatalf("prompt mismatch\ngot:  %q\nwant: %q", got, prompt)
	}
}

func TestExpandAgentCommandResistsDollarAndSemicolon(t *testing.T) {
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "pwned")
	prompt := fmt.Sprintf(`a'; touch %s; echo $(uname) "b"`, marker)
	cmd := ExpandAgentCommand(`printf '%s' {prompt}`, prompt, tmp, "1s")

	proc := exec.Command("sh", "-c", cmd)
	var stdout bytes.Buffer
	proc.Stdout = &stdout
	if err := proc.Run(); err != nil {
		t.Fatalf("sh -c: %v", err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("injection via ; or $() succeeded")
	}
	if stdout.String() != prompt {
		t.Fatalf("got %q want %q", stdout.String(), prompt)
	}
}
