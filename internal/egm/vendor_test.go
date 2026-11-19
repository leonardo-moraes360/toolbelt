package egm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
)

func testSettings(cmd ...string) *Settings {
	s := &Settings{EgmConfig: config.DefaultEgmConfig()}
	s.AgentCmd = cmd
	return s
}

func TestProfileValidation(t *testing.T) {
	cases := []struct {
		name   string
		edit   func(*Settings)
		reason string
	}{
		{"empty agent_cmd", func(s *Settings) { s.AgentCmd = nil }, "usage"},
		{"relative agent_cmd", func(s *Settings) { s.AgentCmd = []string{"claude"} }, "usage"},
		{"bypass permission mode", func(s *Settings) {
			s.AgentCmd = append(s.AgentCmd, "--permission-mode", "bypassPermissions")
		}, "forbidden_flag"},
		{"dangerously skip", func(s *Settings) {
			s.AgentCmd = append(s.AgentCmd, "--dangerously-skip-permissions")
		}, "forbidden_flag"},
		{"user forbidden flag with value", func(s *Settings) {
			s.ForbiddenFlags = []string{"--add-dir"}
			s.AgentCmd = append(s.AgentCmd, "--add-dir=/")
		}, "forbidden_flag"},
		{"custom without tools_disabled", func(s *Settings) { s.Vendor = VendorCustom }, "usage"},
		{"unknown vendor", func(s *Settings) { s.Vendor = "gpt" }, "usage"},
		{"bad output", func(s *Settings) { s.Output = "xml" }, "usage"},
		{"env NODE_OPTIONS", func(s *Settings) { s.Env.Pass = []string{"NODE_OPTIONS"} }, "env_forbidden"},
		{"env BASH_ENV", func(s *Settings) { s.Env.Pass = []string{"BASH_ENV"} }, "env_forbidden"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := testSettings("/usr/bin/claude")
			tc.edit(s)
			_, err := ResolveProfile(s)
			wantReason(t, err, tc.reason)
		})
	}
}

func TestProfileClaudeDefaults(t *testing.T) {
	p, err := ResolveProfile(testSettings("/usr/bin/claude", "--model", "x"))
	if err != nil {
		t.Fatal(err)
	}
	want := append([]string{"/usr/bin/claude", "--model", "x"}, claudeFlags...)
	if strings.Join(p.Argv, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("argv = %q", p.Argv)
	}
	if p.Output != OutputClaudeJSON || !p.ToolsCheck {
		t.Fatalf("output=%s toolsCheck=%v", p.Output, p.ToolsCheck)
	}
	if len(ArgvSHA(p)) != 64 {
		t.Fatalf("ArgvSHA = %q", ArgvSHA(p))
	}
	// NUL joining keeps ["a b"] and ["a","b"] apart.
	if ArgvSHA(&Profile{Argv: []string{"a b"}}) == ArgvSHA(&Profile{Argv: []string{"a", "b"}}) {
		t.Fatal("ArgvSHA collides on argument boundaries")
	}
}

func TestProfileToolsCheck(t *testing.T) {
	s := testSettings("/usr/bin/claude")
	s.ToolsCheck = "none"
	if p, err := ResolveProfile(s); err != nil || p.ToolsCheck {
		t.Fatalf("tools_check none: %v %+v", err, p)
	}
	s = testSettings("/opt/agent")
	s.Vendor, s.ToolsDisabled = VendorCustom, true
	p, err := ResolveProfile(s)
	if err != nil {
		t.Fatal(err)
	}
	if p.ToolsCheck || p.Output != OutputRaw || len(p.Argv) != 1 {
		t.Fatalf("custom profile = %+v", p)
	}
}

func TestVendorExtractPayload(t *testing.T) {
	cases := []struct {
		name      string
		stdout    string
		payload   string
		toolsUsed bool
		wantErr   bool
	}{
		{"result", `{"type":"result","result":"{\"x\":1}","num_turns":1,"permission_denials":[]}`, `{"x":1}`, false, false},
		{"two turns", `{"result":"r","num_turns":2}`, "r", true, false},
		{"denials", `{"result":"r","num_turns":1,"permission_denials":[{"tool_name":"Bash"}]}`, "r", true, false},
		{"tool_use message", `{"result":"r","num_turns":1,"messages":[{"content":[{"type":"tool_use"}]}]}`, "r", true, false},
		{"is_error", `{"result":"r","is_error":true}`, "", false, true},
		{"no result", `{"num_turns":1}`, "", false, true},
		{"not json", `hello`, "", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, used, err := extractPayload(OutputClaudeJSON, []byte(tc.stdout))
			if (err != nil) != tc.wantErr || used != tc.toolsUsed || string(got) != tc.payload {
				t.Fatalf("got %q used=%v err=%v", got, used, err)
			}
		})
	}
	got, used, err := extractPayload(OutputRaw, []byte("raw text"))
	if err != nil || used || string(got) != "raw text" {
		t.Fatalf("raw: %q %v %v", got, used, err)
	}
}

// writeCredential writes a fake credentials file; never the real one.
func writeCredential(t *testing.T, home string, expires time.Time) string {
	t.Helper()
	p := filepath.Join(home, ".claude", ".credentials.json")
	body, err := json.Marshal(map[string]any{
		"claudeAiOauth": map[string]any{"accessToken": "fake-token", "expiresAt": expires.UnixMilli()},
		"otherAccount":  map[string]any{"secret": "must-not-be-copied"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVendorClaudeAuth(t *testing.T) {
	dir := t.TempDir()
	good := writeCredential(t, filepath.Join(dir, "good"), time.Now().Add(time.Hour))
	if err := checkClaudeAuth(good); err != nil {
		t.Fatalf("valid token refused: %v", err)
	}
	soon := writeCredential(t, filepath.Join(dir, "soon"), time.Now().Add(5*time.Minute))
	wantReason(t, checkClaudeAuth(soon), "auth_expired")
	past := writeCredential(t, filepath.Join(dir, "past"), time.Now().Add(-time.Hour))
	wantReason(t, checkClaudeAuth(past), "auth_expired")
	wantReason(t, checkClaudeAuth(filepath.Join(dir, "missing.json")), "auth_expired")

	home := filepath.Join(dir, "home")
	if err := installClaudeCredential(good, home); err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(home, ".claude", ".credentials.json")
	raw, err := os.ReadFile(copied)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "otherAccount") || !strings.Contains(string(raw), "claudeAiOauth") {
		t.Fatal("the copy must hold only claudeAiOauth")
	}
	if fi, _ := os.Stat(copied); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("copy mode = %v", fi.Mode().Perm())
	}
}
