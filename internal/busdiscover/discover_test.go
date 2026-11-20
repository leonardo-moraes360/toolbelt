//go:build unix

package busdiscover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func write(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

type fixture struct {
	home, bin, self, ran string
}

// newFixture: a fake HOME and PATH with claude (a recording script) and codex.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{home: t.TempDir(), bin: t.TempDir()}
	f.ran = filepath.Join(t.TempDir(), "ran")
	rec := "#!/bin/sh\necho ran >> " + f.ran + "\n"
	write(t, filepath.Join(f.bin, "claude"), rec, 0o755)
	write(t, filepath.Join(f.bin, "codex"), rec, 0o755)
	write(t, filepath.Join(f.bin, "not-an-agent"), rec, 0o755)
	f.self = filepath.Join(t.TempDir(), "tb")
	write(t, f.self, "", 0o755)
	return f
}

func (f *fixture) discover(t *testing.T, cwd string) *Catalog {
	t.Helper()
	c, err := Discover(&Options{PathEnv: f.bin + string(os.PathListSeparator) + "node_modules/.bin",
		Home: f.home, Cwd: cwd, WorkDir: "/", Self: f.self, GOOS: "linux"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func find(c *Catalog, client string) []MCPConfig {
	var out []MCPConfig
	for _, m := range c.MCPConfigs {
		if m.Client == client {
			out = append(out, m)
		}
	}
	return out
}

// AC13: table CLIs only, relative PATH entries skipped, nothing executed.
func TestDiscoverCLIs(t *testing.T) {
	f := newFixture(t)
	c := f.discover(t, "")
	if len(c.CLIs) != 2 || c.CLIs[0].Name != "claude" || c.CLIs[0].Preset != "claude" || c.CLIs[0].Wake != "harness" ||
		c.CLIs[1].Name != "codex" || c.CLIs[1].Preset != "" {
		t.Fatalf("clis: %+v", c.CLIs)
	}
	if len(c.SkippedPathEntries) != 1 || c.SkippedPathEntries[0] != "node_modules/.bin" {
		t.Fatalf("skipped: %v", c.SkippedPathEntries)
	}
	if _, err := os.Stat(f.ran); err == nil {
		t.Fatal("discover executed a CLI")
	}
	// AC17: outside a repository and without --cwd there is no project.
	if c.Project != nil || len(find(c, "project")) != 0 {
		t.Fatalf("project: %v", c.Project)
	}
}

// mcpFixture writes the configs of AC14/AC16 and returns the catalog.
func mcpFixture(t *testing.T) (c *Catalog, raw, other string) {
	t.Helper()
	f := newFixture(t)
	write(t, filepath.Join(f.home, ".config", "Claude", "claude_desktop_config.json"),
		`{"mcpServers":{"vault":{"command":"/usr/bin/x","args":["--url","https://secret.example/k"],"env":{"API_KEY":"sk-live-123"}}}}`, 0o600)
	other = filepath.Join(t.TempDir(), "tb")
	write(t, other, "", 0o755)
	write(t, filepath.Join(f.home, ".claude.json"), `{"mcpServers":{"toolbelt":{"command":"`+f.self+`","args":["mcp","--bus-as","ch/marin","--bus-root","/secret/root"],"env":{"TB_X":"v"}}},
	 "projects":{"/p/b":{"mcpServers":{"old":{"command":"`+other+`","args":["mcp","--bus-as","\u001b]52;c;x\u0007/y"]}}},"/p/a":{"mcpServers":{}}}}`, 0o600)
	proj := t.TempDir()
	write(t, filepath.Join(proj, ".mcp.json"), `{"mcpServers":{"a":{"command":"tb","args":["mcp"]},"b":{"command":"sh","args":["-c","tb mcp"]}}}`, 0o600)
	write(t, filepath.Join(f.home, ".codex", "config.toml"), "x = 1\n", 0o600)
	c = f.discover(t, proj)
	b, _ := json.Marshal(c)
	return c, string(b), other
}

// AC14: only the tb entry's shape leaves; other servers are counted.
func TestDiscoverMCPNoSecrets(t *testing.T) {
	c, raw, _ := mcpFixture(t)
	for _, secret := range []string{"sk-live-123", "API_KEY", "secret.example", "/secret/root", "TB_X", "/usr/bin/x"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("catalog leaks %q: %s", secret, raw)
		}
	}
	desk := find(c, "claude-desktop")
	if len(desk) != 1 || len(*desk[0].TB) != 0 || *desk[0].OtherServers != 1 || !*desk[0].Parses {
		t.Fatalf("desktop: %s", raw)
	}
	if cx := find(c, "codex"); len(cx) != 1 || cx[0].Parsed == nil || *cx[0].Parsed || cx[0].Parses != nil {
		t.Fatalf("codex: %s", raw)
	}
	if cu := find(c, "cursor"); len(cu) != 1 || cu[0].Kind != KindMissing {
		t.Fatalf("cursor: %s", raw)
	}
}

// AC16: tb matching, same_tb and per-project scopes.
func TestDiscoverMCPMatching(t *testing.T) {
	c, raw, other := mcpFixture(t)
	cc := find(c, "claude-code")
	if len(cc) != 2 || cc[0].Scope != "user" || cc[1].Scope != "project:/p/b" {
		t.Fatalf("claude-code scopes: %s", raw)
	}
	if user := (*cc[0].TB)[0]; !*user.SameTB || *user.BusAs != "ch/marin" || user.Server != "toolbelt" {
		t.Fatalf("user tb: %+v", user)
	}
	if old := (*cc[1].TB)[0]; *old.SameTB || old.Path != other || *old.BusAs != InvalidBusAs {
		t.Fatalf("another tb: %+v", old)
	}
	p := find(c, "project")
	if len(p) != 1 || len(*p[0].TB) != 1 || (*p[0].TB)[0].SameTB != nil || *p[0].OtherServers != 1 {
		t.Fatalf("project: %s", raw)
	}
}

// AC15: symlink, FIFO, oversized and invalid files are never read out.
func TestDiscoverUnsafeConfigs(t *testing.T) {
	f := newFixture(t)
	secret := filepath.Join(t.TempDir(), "id_ed25519")
	write(t, secret, "PRIVATE KEY", 0o600)
	cases := map[string]func(dir string){
		KindSymlink: func(dir string) {
			if err := os.Symlink(secret, filepath.Join(dir, ".mcp.json")); err != nil {
				t.Fatal(err)
			}
		},
		KindOther: func(dir string) {
			if err := syscall.Mkfifo(filepath.Join(dir, ".mcp.json"), 0o600); err != nil {
				t.Skip("mkfifo:", err)
			}
		},
		KindTooLarge: func(dir string) {
			write(t, filepath.Join(dir, ".mcp.json"), strings.Repeat(" ", maxConfigBytes+1), 0o600)
		},
		KindFile: func(dir string) { write(t, filepath.Join(dir, ".mcp.json"), `{"mcpServers": PRIVATE KEY`, 0o600) },
	}
	for kind, mk := range cases {
		dir := t.TempDir()
		mk(dir)
		c := f.discover(t, dir)
		p := find(c, "project")
		raw, _ := json.Marshal(c)
		if len(p) != 1 || p[0].Kind != kind || strings.Contains(string(raw), "PRIVATE") {
			t.Fatalf("%s: %s", kind, raw)
		}
		if kind == KindFile && (p[0].Parses == nil || *p[0].Parses) {
			t.Fatalf("invalid JSON parsed: %s", raw)
		}
	}
}
