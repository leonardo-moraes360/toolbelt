package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Bench.Repetitions != 5 {
		t.Errorf("Default repetitions = %d, want %d", cfg.Bench.Repetitions, 5)
	}
	if cfg.Verify.TestCmd != "go test ./..." {
		t.Errorf("Default verify test_cmd = %q", cfg.Verify.TestCmd)
	}
	if cfg.Verify.HiddenTestCmd != "" {
		t.Errorf("Default verify hidden_test_cmd = %q, want empty", cfg.Verify.HiddenTestCmd)
	}
	if cfg.Verify.BuildCmd != "" {
		t.Errorf("Default verify build_cmd = %q, want empty", cfg.Verify.BuildCmd)
	}
	if cfg.Verify.LintCmd != "" {
		t.Errorf("Default verify lint_cmd = %q, want empty", cfg.Verify.LintCmd)
	}
	if len(cfg.Graph.Languages) < 12 {
		t.Errorf("expected default graph languages, got %v", cfg.Graph.Languages)
	}
	if strings.Join(cfg.Graph.Docs, ",") != "markdown,vault,html,txt" {
		t.Errorf("expected markdown/vault/html/txt docs, got %v", cfg.Graph.Docs)
	}
	if len(cfg.Graph.Frameworks) < 7 {
		t.Errorf("expected default frameworks, got %v", cfg.Graph.Frameworks)
	}
	if cfg.Graph.MaxFileBytes != 512*1024 {
		t.Errorf("unexpected max_file_bytes %d", cfg.Graph.MaxFileBytes)
	}
}

func TestVerifyConfigLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := DefaultConfig()
	cfg.Verify.TestCmd = "pnpm --filter @auditaja/core test"
	cfg.Verify.HiddenTestCmd = "pnpm --filter @auditaja/core test:hidden"
	cfg.Verify.BuildCmd = "pnpm --filter @auditaja/core build"
	cfg.Verify.LintCmd = "pnpm --filter @auditaja/core lint"
	if err := Save(cfg, configPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded := DefaultConfig()
	if err := loadFromFile(configPath, loaded); err != nil {
		t.Fatalf("loadFromFile failed: %v", err)
	}
	if loaded.Verify.TestCmd != cfg.Verify.TestCmd {
		t.Errorf("test_cmd = %q", loaded.Verify.TestCmd)
	}
	if loaded.Verify.HiddenTestCmd != cfg.Verify.HiddenTestCmd {
		t.Errorf("hidden_test_cmd = %q", loaded.Verify.HiddenTestCmd)
	}
	if loaded.Verify.BuildCmd != cfg.Verify.BuildCmd {
		t.Errorf("build_cmd = %q", loaded.Verify.BuildCmd)
	}
	if loaded.Verify.LintCmd != cfg.Verify.LintCmd {
		t.Errorf("lint_cmd = %q", loaded.Verify.LintCmd)
	}
}

func TestEnvConfigLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := DefaultConfig()
	cfg.Env.Allow = []string{"CUSTOM_OK", "APP_ENV"}
	cfg.Env.Pass = []string{"GITHUB_TOKEN"}
	if err := Save(cfg, configPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded := DefaultConfig()
	if err := loadFromFile(configPath, loaded); err != nil {
		t.Fatalf("loadFromFile failed: %v", err)
	}
	if len(loaded.Env.Allow) != 2 || loaded.Env.Allow[0] != "CUSTOM_OK" {
		t.Errorf("env.allow = %#v", loaded.Env.Allow)
	}
	if len(loaded.Env.Pass) != 1 || loaded.Env.Pass[0] != "GITHUB_TOKEN" {
		t.Errorf("env.pass = %#v", loaded.Env.Pass)
	}
}

func TestGraphConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := DefaultConfig()
	cfg.Graph.Languages = []string{"go", "python"}
	cfg.Graph.Docs = []string{"markdown"}
	cfg.Graph.Frameworks = []string{"react", "vue"}
	cfg.Graph.MaxFileBytes = 1024
	if err := Save(cfg, configPath); err != nil {
		t.Fatal(err)
	}
	loaded := DefaultConfig()
	if err := loadFromFile(configPath, loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Graph.Languages) != 2 || loaded.Graph.Languages[1] != "python" {
		t.Fatalf("languages=%v", loaded.Graph.Languages)
	}
	if len(loaded.Graph.Docs) != 1 || loaded.Graph.Docs[0] != "markdown" {
		t.Fatalf("docs=%v", loaded.Graph.Docs)
	}
	if len(loaded.Graph.Frameworks) != 2 || loaded.Graph.Frameworks[0] != "react" {
		t.Fatalf("frameworks=%v", loaded.Graph.Frameworks)
	}
	if loaded.Graph.MaxFileBytes != 1024 {
		t.Fatalf("max_file_bytes=%d", loaded.Graph.MaxFileBytes)
	}
}

func TestInit(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".toolbelt")

	err := Init(configDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Error(".toolbelt directory was not created")
	}

	configFile := filepath.Join(configDir, DefaultConfigFile)
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("config.yaml was not created")
	}

	subdirs := []string{"vault", "logs", "bench"}
	for _, sub := range subdirs {
		subpath := filepath.Join(configDir, sub)
		if _, err := os.Stat(subpath); os.IsNotExist(err) {
			t.Errorf("subdirectory %s was not created", sub)
		}
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".toolbelt")

	if Exists(configDir) {
		t.Error("Exists should return false before Init")
	}

	if err := Init(configDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if !Exists(configDir) {
		t.Error("Exists should return true after Init")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Bench.Timeout = "60s"
	cfg.Bench.Repetitions = 10

	if err := Save(cfg, configPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded := DefaultConfig()
	if err := loadFromFile(configPath, loaded); err != nil {
		t.Fatalf("loadFromFile failed: %v", err)
	}

	if loaded.Bench.Timeout != "60s" {
		t.Errorf("Loaded timeout = %q, want %q", loaded.Bench.Timeout, "60s")
	}
	if loaded.Bench.Repetitions != 10 {
		t.Errorf("Loaded repetitions = %d, want %d", loaded.Bench.Repetitions, 10)
	}
}

func TestLoadHonorsSetFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "explicit.yaml")
	cfg := DefaultConfig()
	cfg.Bench.DefaultAgent = "echo FROM_SET_FILE"
	cfg.Bench.TasksDir = "/abs/tasks"
	if err := Save(cfg, cfgPath); err != nil {
		t.Fatalf("Save: %v", err)
	}

	SetFile(cfgPath)
	t.Cleanup(func() { SetFile("") })

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Bench.DefaultAgent != "echo FROM_SET_FILE" {
		t.Fatalf("DefaultAgent = %q", loaded.Bench.DefaultAgent)
	}
	if loaded.Bench.TasksDir != "/abs/tasks" {
		t.Fatalf("TasksDir = %q", loaded.Bench.TasksDir)
	}
	if File() != cfgPath {
		t.Fatalf("File() = %q, want %q", File(), cfgPath)
	}
}

func TestDefaultBenchOutputDir(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Bench.OutputDir != "bench/results" {
		t.Fatalf("OutputDir = %q, want bench/results", cfg.Bench.OutputDir)
	}
}

func TestLocalOverridesGlobal(t *testing.T) {
	tmpDir := t.TempDir()

	globalDir := filepath.Join(tmpDir, "global")
	os.MkdirAll(globalDir, 0o755)
	globalCfg := DefaultConfig()
	globalCfg.Bench.Timeout = "30s"
	globalCfg.Bench.Repetitions = 5
	Save(globalCfg, filepath.Join(globalDir, "config.yaml"))

	localDir := filepath.Join(tmpDir, "local")
	os.MkdirAll(localDir, 0o755)
	localCfg := DefaultConfig()
	localCfg.Bench.Timeout = "60s"
	Save(localCfg, filepath.Join(localDir, "config.yaml"))

	cfg := DefaultConfig()
	loadFromFile(filepath.Join(globalDir, "config.yaml"), cfg)
	loadFromFile(filepath.Join(localDir, "config.yaml"), cfg)

	if cfg.Bench.Timeout != "60s" {
		t.Errorf("Local should override global, got timeout = %q", cfg.Bench.Timeout)
	}
}

func TestRouterConfigDefaults(t *testing.T) {
	r := DefaultConfig().Router
	if r.MaxWorkers != 3 || r.Strategy != "dependency" || !r.DirectDelivery.Enabled ||
		r.DirectDelivery.SubscriptionTTL != "30m" || r.DirectDelivery.MaxSubscriptionsPerWorker != 5 ||
		r.Timeouts.PerTicket != "10m" || r.Timeouts.Global != "1h" || r.Retry.MaxAttempts != 2 || r.SlowThreshold != 0.8 {
		t.Fatalf("defaults do router inesperados: %+v", r)
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRouterConfigOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yml := "router:\n  max_workers: 5\n  agent_cmd: [\"sh\", \"-c\", \"true\"]\n  direct_delivery:\n    enabled: false\n  retry:\n    max_attempts: 4\n"
	if err := os.WriteFile(path, []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Router
	if got.MaxWorkers != 5 || got.DirectDelivery.Enabled || got.Retry.MaxAttempts != 4 || len(got.AgentCmd) != 3 {
		t.Fatalf("override: %+v", got)
	}
	// Unmentioned fields keep their defaults.
	if got.DirectDelivery.SubscriptionTTL != "30m" || got.Timeouts.PerTicket != "10m" || got.Strategy != "dependency" {
		t.Fatalf("padrões perdidos: %+v", got)
	}
}

func TestRouterConfigValidate(t *testing.T) {
	bad := []func(*RouterConfig){
		func(r *RouterConfig) { r.MaxWorkers = 0 },
		func(r *RouterConfig) { r.Strategy = "xyz" },
		func(r *RouterConfig) { r.Timeouts.PerTicket = "abc" },
		func(r *RouterConfig) { r.Timeouts.Global = "0s" },
		func(r *RouterConfig) { r.DirectDelivery.SubscriptionTTL = "" },
		func(r *RouterConfig) { r.DirectDelivery.MaxSubscriptionsPerWorker = 0 },
		func(r *RouterConfig) { r.Retry.MaxAttempts = 0 },
		func(r *RouterConfig) { r.SlowThreshold = 1.5 },
	}
	for i, mut := range bad {
		r := DefaultRouterConfig()
		mut(&r)
		if err := r.Validate(); err == nil {
			t.Errorf("caso %d: esperava erro de validação", i)
		}
	}
}

func TestVaultConfigValidate(t *testing.T) {
	ok := DefaultConfig().Vault
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*VaultConfig){
		"absolute":   func(v *VaultConfig) { v.Path = "/tmp/v" },
		"dotdot":     func(v *VaultConfig) { v.Path = "../v" },
		"dot":        func(v *VaultConfig) { v.Path = "." },
		"glob":       func(v *VaultConfig) { v.Path = "v*" },
		"newline":    func(v *VaultConfig) { v.Path = "v\nx" },
		"backslash":  func(v *VaultConfig) { v.Path = `a\b` },
		"spaces":     func(v *VaultConfig) { v.Path = " v" },
		"hash":       func(v *VaultConfig) { v.Path = "#v" },
		"zero bytes": func(v *VaultConfig) { v.MaxNoteBytes = 0 },
		"too big":    func(v *VaultConfig) { v.MaxNoteBytes = 5 << 20 },
		"no types":   func(v *VaultConfig) { v.Types = nil },
		"dup types":  func(v *VaultConfig) { v.Types = []string{"a", "a"} },
		"bad type":   func(v *VaultConfig) { v.Types = []string{"A"} },
	} {
		c := ok
		c.Types = append([]string(nil), ok.Types...)
		mut(&c)
		if c.Validate() == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestUnknownKeysWarnedNotFatal(t *testing.T) {
	var buf strings.Builder
	old := warnOut
	warnOut = &buf
	t.Cleanup(func() { warnOut = old; warnedFiles = sync.Map{} })
	warnedFiles = sync.Map{}

	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "defaults:\n  sandbox: true\ngraph:\n  auto_index: true\n  index_on_change: false\nverify:\n  checks:\n    - name: a\n      cmd: b\n      bogus: 1\nlogging:\n  level: info\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	if err := loadFromFile(path, cfg); err != nil {
		t.Fatalf("unknown keys must not fail: %v", err)
	}
	if cfg.Graph.IndexOnChange {
		t.Error("known key was not applied")
	}
	if err := loadFromFile(path, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	want := "warning: " + path + ": 4 unknown config keys ignored: defaults, graph.auto_index, logging, verify.checks.bogus ("
	if strings.Count(got, "\n") != 1 || !strings.Contains(got, want) {
		t.Errorf("want one line containing %q, got:\n%s", want, got)
	}
	if strings.Contains(got, "index_on_change") {
		t.Errorf("known key reported:\n%s", got)
	}
}

func TestDefaultConfigHasNoUnknownKeys(t *testing.T) {
	data, err := yaml.Marshal(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if got := unknownKeys(doc.Content[0], reflect.TypeOf(Config{}), ""); len(got) != 0 {
		t.Errorf("a written config warns about itself: %v", got)
	}
}

func TestUnknownKeysMergeInlineAndSkips(t *testing.T) {
	parse := func(y string, typ any) []string {
		var doc yaml.Node
		if err := yaml.Unmarshal([]byte(y), &doc); err != nil {
			t.Fatal(err)
		}
		return unknownKeys(doc.Content[0], reflect.TypeOf(typ), "")
	}
	got := parse("base: &b {test_cmd: a, bogus: 1}\nverify:\n  <<: *b\n  lint_cmd: x\n", struct {
		Base   map[string]any `yaml:"base"`
		Verify VerifyConfig   `yaml:"verify"`
	}{})
	if len(got) != 1 || got[0] != "verify.bogus" {
		t.Errorf("merge key: got %v, want [verify.bogus]", got)
	}
	type embedded struct {
		In string `yaml:"in"`
	}
	type local struct {
		embedded `yaml:",inline"`
		Skip     string `yaml:"-"`
		hidden   string
		Name     string `yaml:"name"`
	}
	_ = local{}.hidden
	got = parse("in: 1\nname: a\nSkip: b\nhidden: c\n", local{})
	if len(got) != 2 || got[0] != "Skip" || got[1] != "hidden" {
		t.Errorf("inline/skip: got %v, want [Skip hidden]", got)
	}
}

func TestUnknownKeysDedupAcrossPathSpellings(t *testing.T) {
	var buf strings.Builder
	old := warnOut
	warnOut = &buf
	t.Cleanup(func() { warnOut = old; warnedFiles = sync.Map{} })
	warnedFiles = sync.Map{}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "c.yaml"), []byte("logging: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	for _, p := range []string{"c.yaml", filepath.Join(dir, "c.yaml"), "./c.yaml"} {
		if err := loadFromFile(p, DefaultConfig()); err != nil {
			t.Fatal(err)
		}
	}
	if n := strings.Count(buf.String(), "unknown config key ignored: logging"); n != 1 {
		t.Errorf("warned %d times, want 1:\n%s", n, buf.String())
	}
}
