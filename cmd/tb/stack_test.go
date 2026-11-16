package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStackUnknownOpNoUsage(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	out, code := runTBCapture(t, tbBin, "", "stack", "nope")
	if code == 0 {
		t.Fatalf("op desconhecida deveria sair ≠ 0\n%s", out)
	}
	if !strings.Contains(out, "operação desconhecida") {
		t.Fatalf("output: %s", out)
	}
	if strings.Contains(out, "Usage:") {
		t.Fatalf("não deve dumpar Usage:\n%s", out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("saída deveria ser JSON com error no step: %v\n%s", err, out)
	}
	steps, _ := payload["steps"].([]any)
	if len(steps) == 0 {
		t.Fatalf("steps vazios: %s", out)
	}
	first, _ := steps[0].(map[string]any)
	if first["error"] == nil {
		t.Fatalf("error ausente no step: %s", out)
	}
}

func TestStackNoOpsShortErrorNoUsage(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	stderr, code := runTBCapture(t, tbBin, "", "stack")
	if code == 0 {
		t.Fatalf("stack sem ops deveria sair ≠ 0\n%s", stderr)
	}
	for _, op := range []string{"verify", "review", "drift", "guard"} {
		if !strings.Contains(stderr, op) {
			t.Fatalf("stderr deveria listar op %q:\n%s", op, stderr)
		}
	}
	if strings.Contains(stderr, "Usage:") {
		t.Fatalf("não deve dumpar Usage:\n%s", stderr)
	}
}

func TestStackHelpClarifiesNotBranchStack(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	out, code := runTBCapture(t, tbBin, "", "stack", "--help")
	if code != 0 {
		t.Fatalf("stack --help deveria sair 0\n%s", out)
	}
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "verificação") && !strings.Contains(lower, "verify") {
		t.Fatalf("help deveria deixar claro que empilha verificação:\n%s", out)
	}
	if !strings.Contains(lower, "branch") && !strings.Contains(lower, "graphite") && !strings.Contains(lower, "git-stack") {
		t.Fatalf("help deveria esclarecer que não é stack de branches:\n%s", out)
	}
}

func TestStackCLIAggregateJSON(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-m", "base")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nfunc X() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rd.md"), []byte("---\ntitle: t\nscope:\n  - a.go\n  - rd.md\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-m", "head")

	cmd := exec.Command(tbBin, "stack", "guard", "drift", "--ticket", "rd.md", "--base", "HEAD~1", "--head", "HEAD")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("stack: %v\n%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if payload["ok"] != true {
		t.Fatalf("ok=%v\n%s", payload["ok"], out)
	}
	steps, _ := payload["steps"].([]any)
	if len(steps) != 2 {
		t.Fatalf("steps=%d\n%s", len(steps), out)
	}
}
