package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/golden/*.json")

// goldenScenario builds a fixture repository and the Config a caller would
// pass for today's `tb verify run --json` (tests ruler, no new config).
type goldenScenario struct {
	name  string
	setup func(t *testing.T, dir string, cfg *Config)
}

func goldenScenarios() []goldenScenario {
	return []goldenScenario{
		{"pass_build_lint", func(_ *testing.T, _ string, cfg *Config) {
			cfg.BuildCmd = "true"
			cfg.LintCmd = "true"
			cfg.VisibleTest = "true"
		}},
		{"pass_no_gates", func(_ *testing.T, _ string, cfg *Config) {
			cfg.VisibleTest = "true"
		}},
		{"lint_fail", func(_ *testing.T, _ string, cfg *Config) {
			cfg.LintCmd = "echo 'x.go:1: unused var'; exit 1"
			cfg.VisibleTest = "true"
		}},
		{"build_fail", func(_ *testing.T, _ string, cfg *Config) {
			cfg.BuildCmd = "echo 'build broke'; exit 2"
			cfg.VisibleTest = "true"
		}},
		{"test_fail_deterministic", func(_ *testing.T, _ string, cfg *Config) {
			cfg.VisibleTest = "echo '--- FAIL: TestX'; exit 1"
		}},
		{"hidden_fail", func(_ *testing.T, _ string, cfg *Config) {
			cfg.VisibleTest = "true"
			cfg.HiddenTest = "echo '--- FAIL: TestHidden'; exit 1"
		}},
		{"missing_command", func(_ *testing.T, _ string, cfg *Config) {
			cfg.VisibleTest = "tb-no-such-command-xyz"
		}},
		{"dry_run", func(_ *testing.T, _ string, cfg *Config) {
			cfg.BuildCmd = "make build"
			cfg.LintCmd = "make lint"
			cfg.VisibleTest = "make test"
			cfg.HiddenTest = "make hidden"
			cfg.DryRun = true
		}},
		{"guard_abort", func(t *testing.T, dir string, cfg *Config) {
			base := gitRev(t, dir)
			writeFile(t, filepath.Join(dir, "tests", "hidden", "h_test.go"), "package hidden\n")
			runGit(t, dir, "add", "-A")
			runGit(t, dir, "commit", "-q", "-m", "touch hidden")
			cfg.BaseRef = base
			cfg.VisibleTest = "true"
		}},
	}
}

func goldenRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	writeFile(t, filepath.Join(dir, "README"), "fixture\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "c")
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// normalizeGolden replaces machine-dependent values (SHAs, temp dirs, the
// env names the allowlist dropped, guard's rule count) with placeholders so
// the golden pins verify's keys and values, not the host.
func normalizeGolden(t *testing.T, raw []byte, dir string) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"head_sha", "workdir", "filtered_env"} {
		if _, ok := m[k]; ok {
			m[k] = "<" + k + ">"
		}
	}
	if p, ok := m["plan"].(map[string]any); ok {
		p["dir"] = "<dir>"
	}
	if g, ok := m["guard"].(map[string]any); ok {
		g["base_ref"], g["head_ref"], g["rules_checked"] = "<sha>", "<sha>", "<n>"
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return bytes.ReplaceAll(out, []byte(dir), []byte("<dir>"))
}

func runGolden(t *testing.T, sc goldenScenario, run func(*Config) (*Result, error)) []byte {
	t.Helper()
	dir := goldenRepo(t)
	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	sc.setup(t, dir, &cfg)
	res, err := run(&cfg)
	if err != nil && !IsMissingCommand(err) {
		t.Fatal(err)
	}
	raw, jerr := res.JSON()
	if jerr != nil {
		t.Fatal(jerr)
	}
	return normalizeGolden(t, raw, dir)
}

func TestGoldenTestsRulerJSON(t *testing.T) {
	for _, sc := range goldenScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			got := runGolden(t, sc, func(cfg *Config) (*Result, error) {
				return Run(context.Background(), cfg)
			})
			path := filepath.Join("testdata", "golden", sc.name+".json")
			if *updateGolden {
				writeFile(t, path, string(got)+"\n")
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(got)) {
				t.Fatalf("golden mismatch %s\n--- want\n%s\n--- got\n%s", sc.name, want, got)
			}
			viaRulers := runGolden(t, sc, runViaRulers(t))
			if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(viaRulers)) {
				t.Fatalf("RunRulers payload differs from Run %s\n--- want\n%s\n--- got\n%s", sc.name, want, viaRulers)
			}
		})
	}
}

// runViaRulers adapts RunRulers([tests]) to Run's (payload, error) shape.
func runViaRulers(t *testing.T) func(*Config) (*Result, error) {
	return func(cfg *Config) (*Result, error) {
		multi, err := RunRulers(context.Background(), cfg, []string{RulerTests})
		if err != nil {
			return nil, err
		}
		res, ok := multi.Payload().(*Result)
		if !ok {
			t.Fatalf("payload %T, want *Result", multi.Payload())
		}
		return res, nil
	}
}
