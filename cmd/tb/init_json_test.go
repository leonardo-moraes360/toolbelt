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

func runInitJSON(t *testing.T, tbBin, dir string, env []string, args ...string) map[string]any {
	t.Helper()
	cmd := exec.Command(tbBin, append([]string{"init", "--json"}, args...)...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
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
	return payload
}

func writeInitFixture(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// Acceptance 5 (docs/specs/verify-rulers.md): a typecheck script is a
// typecheck_cmd, not a build_cmd; tsconfig.json + local tsc gives exec tsc.
func TestInitJSONTypecheck(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name:  "typecheck script",
			files: map[string]string{"package.json": `{"scripts":{"test":"jest","typecheck":"tsc --noEmit"}}`, "package-lock.json": "{}\n"},
			want:  "npm run typecheck",
		},
		{
			name: "tsconfig with local tsc",
			files: map[string]string{
				"package.json":          `{"scripts":{"test":"jest"}}`,
				"pnpm-lock.yaml":        "lockfileVersion: '9.0'\n",
				"tsconfig.json":         "{}\n",
				"node_modules/.bin/tsc": "#!/bin/sh\n",
			},
			want: "pnpm exec tsc --noEmit",
		},
		{
			name: "tsconfig with local tsc, npm",
			files: map[string]string{
				"package.json":          `{"scripts":{"test":"jest"}}`,
				"package-lock.json":     "{}\n",
				"tsconfig.json":         "{}\n",
				"node_modules/.bin/tsc": "#!/bin/sh\n",
			},
			want: "npm exec -- tsc --noEmit",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeInitFixture(t, dir, tt.files)
			payload := runInitJSON(t, tbBin, dir, nil)
			if payload["typecheck_cmd"] != tt.want {
				t.Fatalf("typecheck_cmd=%v want %q: %v", payload["typecheck_cmd"], tt.want, payload)
			}
			if _, ok := payload["build_cmd"]; ok {
				t.Fatalf("build_cmd should be absent: %v", payload)
			}
		})
	}
}

// R3: init suggests govulncheck without --write-checks and writes it with it.
func TestInitJSONChecks(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	fakeBin := t.TempDir()
	writeInitFixture(t, fakeBin, map[string]string{"govulncheck": "#!/bin/sh\nexit 0\n"})
	env := append(os.Environ(), "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	want := map[string]any{"name": "govulncheck", "cmd": "govulncheck ./..."}

	for _, write := range []bool{false, true} {
		dir := t.TempDir()
		writeInitFixture(t, dir, map[string]string{"go.mod": "module example.com/demo\n\ngo 1.22\n"})
		var args []string
		if write {
			args = append(args, "--write-checks")
		}
		payload := runInitJSON(t, tbBin, dir, env, args...)
		present, absent := "suggested_checks", "checks"
		if write {
			present, absent = absent, present
		}
		list, _ := payload[present].([]any)
		if len(list) != 1 {
			t.Fatalf("write=%v: %s=%v: %v", write, present, payload[present], payload)
		}
		got, _ := list[0].(map[string]any)
		if got["name"] != want["name"] || got["cmd"] != want["cmd"] {
			t.Fatalf("write=%v: %s[0]=%v want %v", write, present, got, want)
		}
		if _, ok := payload[absent]; ok {
			t.Fatalf("write=%v: %s should be absent: %v", write, absent, payload)
		}
	}
}
