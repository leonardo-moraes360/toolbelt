package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/envelope"
)

// envelopeRepo is a git repository with two commits, a ticket file and a Go
// file, enough for every stable --json command to produce its envelope.
func envelopeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-b", "main")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "t")
	write("go.mod", "module example.com/m\n\ngo 1.21\n")
	write("a.go", "package m\n\nfunc A() {}\n")
	git("add", ".")
	git("commit", "-m", "one")
	write("b.go", "package m\n\nfunc B() { A() }\n")
	write("rd.md", "---\ntitle: t\nscope:\n  - b.go\n  - rd.md\n---\n")
	git("add", ".")
	git("commit", "-m", "two")
	return dir
}

func runTBJSON(t *testing.T, bin, dir string, env []string, args ...string) (stdout []byte, code int) {
	t.Helper()
	c := exec.Command(bin, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), env...)
	var out bytes.Buffer
	c.Stdout = &out
	err := c.Run()
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("tb %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return out.Bytes(), code
}

// TestEveryStableJSONCommandCarriesV lists every stable --json command and
// checks the envelope kind in "v" (item J1).
func TestEveryStableJSONCommandCarriesV(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	bin := buildTB(t)
	dir := envelopeRepo(t)
	graphEnv := []string{"TB_GRAPH=on"}

	cases := []struct {
		kind string
		env  []string
		args []string
	}{
		{envelope.Version, nil, []string{"version"}},
		{envelope.Init, nil, []string{"init"}},
		{envelope.Mem, nil, []string{"mem", "init"}},
		{envelope.Mem, nil, []string{"mem", "list"}},
		{envelope.Mem, nil, []string{"mem", "check"}},
		{envelope.Mem, nil, []string{"mem", "search", "tests"}},
		{envelope.Verify, nil, []string{"verify", "run", "--dry-run", "--test-cmd", "true"}},
		{envelope.Guard, nil, []string{"guard", "check", "--base", "HEAD~1", "--head", "HEAD"}},
		{envelope.Review, nil, []string{"review", "rd.md", "--base", "HEAD~1", "--head", "HEAD"}},
		{envelope.Drift, nil, []string{"drift", "--ticket", "rd.md", "--base", "HEAD~1", "--head", "HEAD"}},
		{envelope.Stack, nil, []string{"stack", "guard", "--base", "HEAD~1", "--head", "HEAD"}},
		{envelope.SandboxCheck, nil, []string{"sandbox", "check"}},
		{envelope.SandboxRun, nil, []string{"sandbox", "run", "--allow-unsandboxed", "--", "true"}},
		{envelope.GraphIndex, graphEnv, []string{"graph", "index"}},
		{envelope.GraphRebuild, graphEnv, []string{"graph", "rebuild"}},
		{envelope.GraphStats, graphEnv, []string{"graph", "stats"}},
		{envelope.GraphSearch, graphEnv, []string{"graph", "search", "A"}},
		{envelope.GraphCallers, graphEnv, []string{"graph", "callers", "--function", "A"}},
		{envelope.GraphDeps, graphEnv, []string{"graph", "deps", "--file", "b.go"}},
		{envelope.GraphImpact, graphEnv, []string{"graph", "impact", "--file", "a.go"}},
		{envelope.GraphExplain, graphEnv, []string{"graph", "explain", "a.go"}},
		{envelope.GraphPath, graphEnv, []string{"graph", "path", "a.go", "b.go"}},
		{envelope.TicketList, nil, []string{"ticket", "list"}},
		{envelope.Ticket, nil, []string{"ticket", "create", "--title", "x"}},
		{envelope.Ticket, nil, []string{"ticket", "show", "TKT-001"}},
		{envelope.Workspace, nil, []string{"workspace", "list"}},
	}
	for _, tc := range cases {
		args := append([]string{"--json"}, tc.args...)
		out, _ := runTBJSON(t, bin, dir, tc.env, args...)
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(out, &doc); err != nil {
			t.Errorf("tb %v: stdout is not one JSON object: %v\n%s", tc.args, err, out)
			continue
		}
		var v string
		_ = json.Unmarshal(doc["v"], &v)
		if !envelope.ValidKind(v) {
			t.Errorf("tb %v: v = %q, want <kind>/<major>.<minor>", tc.args, v)
		}
		if v != tc.kind {
			t.Errorf("tb %v: v = %q, want %q", tc.args, v, tc.kind)
		}
	}
}

func TestJSONErrorBody(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	bin := buildTB(t)
	dir := t.TempDir()
	cases := []struct {
		args       []string
		wantCode   int
		wantReason string
	}{
		{[]string{"mem", "show", "nope"}, 1, "error"},
		{[]string{"ticket", "show"}, 2, "usage"},
		{[]string{"mem", "nope"}, 2, "usage"},
		{[]string{"sandbox", "run", "--allow-unsandboxed", "no-such-command-tb"}, 127, "command_not_found"},
	}
	for _, tc := range cases {
		out, code := runTBJSON(t, bin, dir, nil, append([]string{"--json"}, tc.args...)...)
		if code != tc.wantCode {
			t.Errorf("tb %v: exit %d, want %d", tc.args, code, tc.wantCode)
		}
		var body envelope.ErrorBody
		if err := json.Unmarshal(out, &body); err != nil {
			t.Errorf("tb %v: stdout is not an error body: %v\n%s", tc.args, err, out)
			continue
		}
		if body.V != envelope.Error || body.Reason != tc.wantReason || body.ExitCode != tc.wantCode || body.Error == "" {
			t.Errorf("tb %v: body = %+v", tc.args, body)
		}
	}
}
