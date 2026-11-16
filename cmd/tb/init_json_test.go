package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInitJSONPayload(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(tbBin, "init", "--json")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("init --json: %v\nstdout=%s\nstderr=%s", err, out, ee.Stderr)
		}
		t.Fatalf("init --json: %v\n%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if payload["path"] == nil || payload["path"] == "" {
		t.Fatalf("path ausente: %s", out)
	}
	if payload["test_cmd"] != "go test ./..." {
		t.Fatalf("test_cmd=%v want go test ./...", payload["test_cmd"])
	}
	agents, ok := payload["agents"].(map[string]any)
	if !ok {
		t.Fatalf("agents ausente: %s", out)
	}
	if _, ok := agents["sources"].([]any); !ok {
		t.Fatalf("agents.sources deveria ser []: %s", out)
	}
}

func TestMemInitJSONPayload(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	dir := t.TempDir()
	cmd := exec.Command(tbBin, "mem", "init", "--json")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("mem init --json: %v\nstdout=%s\nstderr=%s", err, out, ee.Stderr)
		}
		t.Fatalf("mem init --json: %v\n%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	for _, key := range []string{"path", "created", "count", "invariants"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("faltando %s: %s", key, out)
		}
	}
	if payload["created"] != true {
		t.Fatalf("created=%v", payload["created"])
	}
	invs, _ := payload["invariants"].([]any)
	if len(invs) == 0 {
		t.Fatalf("invariants vazias: %s", out)
	}
}
