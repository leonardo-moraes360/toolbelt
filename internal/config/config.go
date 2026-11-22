// Package config loads toolbelt config: global (~/.config/toolbelt/config.yaml)
// and local (.toolbelt/config.yaml), where local overrides global.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

const DefaultConfigDir = ".toolbelt"

const DefaultConfigFile = "config.yaml"

type Config struct {
	Defaults Defaults `yaml:"defaults"`

	Sandbox SandboxConfig `yaml:"sandbox"`

	Graph GraphConfig `yaml:"graph"`

	Vault VaultConfig `yaml:"vault"`

	Logging LoggingConfig `yaml:"logging"`

	Bench BenchConfig `yaml:"bench"`

	Verify VerifyConfig `yaml:"verify"`

	// review/drift (approved-sha).
	Review ReviewConfig `yaml:"review"`

	// Allowlist of variables passed to children (sh -c).
	Env EnvConfig `yaml:"env"`

	// Ticket scheduler (tb router run).
	Router RouterConfig `yaml:"router"`

	// tb egm (docs/specs/egm-nativo.md).
	Egm EgmConfig `yaml:"egm"`

	// tb merge semantic/prompt budgets (docs/specs/merge-strategies.md);
	// user-only. Zero outside LoadUser, so `tb init` never writes the section
	// (a repository value would only trigger config_untrusted).
	Merge MergeConfig `yaml:"merge,omitempty"`
}

type Defaults struct {
	Sandbox    bool   `yaml:"sandbox"`
	Timeout    string `yaml:"timeout"`
	JSONOutput bool   `yaml:"json_output"`
}

type SandboxConfig struct {
	AllowNet  []string `yaml:"allow_net"`
	CPULimit  int      `yaml:"cpu_limit"`
	MemLimit  string   `yaml:"mem_limit"`
	TimeLimit string   `yaml:"time_limit"`
	// Read and Write are extra absolute paths (~/ allowed) granted to the MCP
	// tb_run sandbox, read-only and read-write, on top of the workdir, the
	// system dirs, PATH, GOROOT and the toolchain caches.
	Read  []string `yaml:"read,omitempty"`
	Write []string `yaml:"write,omitempty"`
	// AllowUnsandboxed lets MCP tb_run run without isolation when Landlock is
	// unavailable. Off by default: tb_run fails closed.
	AllowUnsandboxed bool `yaml:"allow_unsandboxed,omitempty"`
}

type GraphConfig struct {
	AutoIndex      bool     `yaml:"auto_index"`
	IndexOnChange  bool     `yaml:"index_on_change"`
	InferRelations bool     `yaml:"infer_relations"`
	Languages      []string `yaml:"languages"`
	Docs           []string `yaml:"docs"`
	Frameworks     []string `yaml:"frameworks"`
	MaxFileBytes   int64    `yaml:"max_file_bytes"`
	// Vector and Embed are user-only: Load drops them from the repository
	// layer and graph reads them through LoadUser.
	Vector GraphVectorConfig `yaml:"vector,omitempty"`
	Embed  GraphEmbedConfig  `yaml:"embed,omitempty"`
}

// VaultConfig is the vault: section (docs/specs/vault.md, Configuration).
type VaultConfig struct {
	Path         string `yaml:"path"`
	SyncOnChange bool   `yaml:"sync_on_change"` // reserved, no effect in Phase A
	MaxNoteBytes int64  `yaml:"max_note_bytes"`
	// Types are the allowed frontmatter type values.
	Types []string `yaml:"types"`
	// Graph false keeps notes out of the graph even with markdown enabled.
	Graph bool `yaml:"graph"`
}

const maxVaultNoteBytes = 4 << 20

var reVaultType = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// Validate checks the vault: section. path is written into
// .git/info/exclude, so it must be a clean repository-relative path with
// no control or glob characters.
func (v *VaultConfig) Validate() error {
	if err := validVaultPath(v.Path); err != nil {
		return err
	}
	if v.MaxNoteBytes <= 0 || v.MaxNoteBytes > maxVaultNoteBytes {
		return fmt.Errorf("vault.max_note_bytes must be between 1 and %d", maxVaultNoteBytes)
	}
	return validVaultTypes(v.Types)
}

func validVaultPath(p string) error {
	switch {
	case p == "" || filepath.IsAbs(p) || strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\`):
		return fmt.Errorf("vault.path must be a relative path inside the repository")
	case strings.ContainsAny(p, "*?[]!\\") || strings.HasPrefix(p, "#") || strings.IndexFunc(p, unicode.IsControl) >= 0:
		return fmt.Errorf("vault.path must not contain control, glob or backslash characters")
	case strings.TrimSpace(p) != p:
		return fmt.Errorf("vault.path must not start or end with spaces")
	}
	if c := filepath.ToSlash(filepath.Clean(p)); c == "." || c == ".." || strings.HasPrefix(c, "../") {
		return fmt.Errorf("vault.path must stay inside the repository")
	}
	return nil
}

func validVaultTypes(types []string) error {
	if len(types) == 0 {
		return fmt.Errorf("vault.types must not be empty")
	}
	seen := map[string]bool{}
	for _, t := range types {
		if !reVaultType.MatchString(t) {
			return fmt.Errorf("vault.types value %q must match %s", t, reVaultType)
		}
		if seen[t] {
			return fmt.Errorf("vault.types has %q twice", t)
		}
		seen[t] = true
	}
	return nil
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	File   string `yaml:"file"`
	Format string `yaml:"format"`
}

type BenchConfig struct {
	TasksDir     string `yaml:"tasks_dir"`
	FixturesDir  string `yaml:"fixtures_dir"`
	OutputDir    string `yaml:"output_dir"`
	DefaultAgent string `yaml:"default_agent"`
	Repetitions  int    `yaml:"repetitions"`
	Timeout      string `yaml:"timeout"`
}

type VerifyConfig struct {
	TestCmd       string `yaml:"test_cmd"`
	HiddenTestCmd string `yaml:"hidden_test_cmd"`
	BuildCmd      string `yaml:"build_cmd,omitempty"`
	LintCmd       string `yaml:"lint_cmd,omitempty"`
}

type ReviewConfig struct {
	RequireApprovedSHA bool `yaml:"require_approved_sha"`
}

// EnvConfig controls which env vars reach child processes (sh -c).
type EnvConfig struct {
	Allow []string `yaml:"allow"`
	Pass  []string `yaml:"pass"`
}

// RouterConfig is shared by tb router and tb watchdog.
type RouterConfig struct {
	MaxWorkers     int                  `yaml:"max_workers" json:"max_workers"`
	Strategy       string               `yaml:"strategy" json:"strategy"`
	AgentCmd       []string             `yaml:"agent_cmd" json:"agent_cmd"`
	DirectDelivery RouterDirectDelivery `yaml:"direct_delivery" json:"direct_delivery"`
	Timeouts       RouterTimeouts       `yaml:"timeouts" json:"timeouts"`
	Retry          RouterRetry          `yaml:"retry" json:"retry"`
	SlowThreshold  float64              `yaml:"slow_threshold" json:"slow_threshold"`
	// Bus: bus slots b1..bN (docs/bus-router.md, B4b). Read only from trusted
	// config (global or -c), never from the repository's config.
	Bus RouterBus `yaml:"bus" json:"bus"`
}

// RouterBus hands tickets to agents on a bus channel.
type RouterBus struct {
	Channel      string `yaml:"channel" json:"channel"`             // empty = no bus slots
	As           string `yaml:"as" json:"as"`                       // the router's CLI session in that channel
	Slots        int    `yaml:"slots" json:"slots"`                 // default 1 when channel is set
	To           string `yaml:"to" json:"to"`                       // role:worker (default) or agent:<name>
	ClaimTimeout string `yaml:"claim_timeout" json:"claim_timeout"` // default 5m
}

// Enabled reports whether bus slots are configured.
func (b *RouterBus) Enabled() bool { return b.Channel != "" }

// ApplyDefaults fills slots, to and claim_timeout when a channel is set.
func (b *RouterBus) ApplyDefaults() {
	if !b.Enabled() {
		return
	}
	if b.Slots == 0 {
		b.Slots = 1
	}
	if b.To == "" {
		b.To = "role:worker"
	}
	if b.ClaimTimeout == "" {
		b.ClaimTimeout = "5m"
	}
}

func (b *RouterBus) validate() error {
	if !b.Enabled() {
		return nil
	}
	if b.As == "" {
		return errors.New("router.bus.as is required with router.bus.channel")
	}
	if b.Slots < 1 {
		return fmt.Errorf("router.bus.slots must be >= 1 (current %d)", b.Slots)
	}
	if b.To != "role:worker" && (!strings.HasPrefix(b.To, "agent:") || b.To == "agent:") {
		return fmt.Errorf("router.bus.to %q invalid (use role:worker or agent:<name>)", b.To)
	}
	if d, err := time.ParseDuration(b.ClaimTimeout); err != nil || d <= 0 {
		return fmt.Errorf("invalid router.bus.claim_timeout %q (e.g. 5m)", b.ClaimTimeout)
	}
	return nil
}

// RouterDirectDelivery: ticket -> worker affinity.
type RouterDirectDelivery struct {
	Enabled                   bool   `yaml:"enabled" json:"enabled"`
	SubscriptionTTL           string `yaml:"subscription_ttl" json:"subscription_ttl"`
	MaxSubscriptionsPerWorker int    `yaml:"max_subscriptions_per_worker" json:"max_subscriptions_per_worker"`
}

// RouterTimeouts bounds each ticket and the whole router run.
type RouterTimeouts struct {
	PerTicket string `yaml:"per_ticket" json:"per_ticket"`
	Global    string `yaml:"global" json:"global"`
}

type RouterRetry struct {
	MaxAttempts int `yaml:"max_attempts" json:"max_attempts"`
}

func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		MaxWorkers: 3,
		Strategy:   "dependency",
		AgentCmd:   []string{},
		DirectDelivery: RouterDirectDelivery{
			Enabled:                   true,
			SubscriptionTTL:           "30m",
			MaxSubscriptionsPerWorker: 5,
		},
		Timeouts:      RouterTimeouts{PerTicket: "10m", Global: "1h"},
		Retry:         RouterRetry{MaxAttempts: 2},
		SlowThreshold: 0.8,
	}
}

func (r *RouterConfig) Validate() error {
	r.Bus.ApplyDefaults()
	if r.MaxWorkers < 0 || (r.MaxWorkers == 0 && !r.Bus.Enabled()) {
		return fmt.Errorf("router.max_workers must be >= 1, or 0 with router.bus.channel (current %d)", r.MaxWorkers)
	}
	if err := r.Bus.validate(); err != nil {
		return err
	}
	switch r.Strategy {
	case "", "fifo", "priority", "dependency":
	default:
		return fmt.Errorf("invalid router.strategy %q (use fifo, priority or dependency)", r.Strategy)
	}
	for name, v := range map[string]string{
		"router.direct_delivery.subscription_ttl": r.DirectDelivery.SubscriptionTTL,
		"router.timeouts.per_ticket":              r.Timeouts.PerTicket,
		"router.timeouts.global":                  r.Timeouts.Global,
	} {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return fmt.Errorf("invalid %s %q (e.g. 10m)", name, v)
		}
	}
	if r.DirectDelivery.MaxSubscriptionsPerWorker < 1 {
		return fmt.Errorf("router.direct_delivery.max_subscriptions_per_worker must be >= 1")
	}
	if r.Retry.MaxAttempts < 1 {
		return fmt.Errorf("router.retry.max_attempts must be >= 1")
	}
	if r.SlowThreshold <= 0 || r.SlowThreshold > 1 {
		return fmt.Errorf("router.slow_threshold must be in (0, 1] (current %v)", r.SlowThreshold)
	}
	return nil
}

func DefaultConfig() *Config {
	return &Config{
		Egm: DefaultEgmConfig(),
		Defaults: Defaults{
			Sandbox:    false,
			Timeout:    "30s",
			JSONOutput: false,
		},
		Sandbox: SandboxConfig{
			AllowNet:  []string{},
			CPULimit:  100,
			MemLimit:  "1G",
			TimeLimit: "5m",
		},
		Graph: GraphConfig{
			AutoIndex:      true,
			IndexOnChange:  true,
			InferRelations: false,
			Languages: []string{
				"go", "javascript", "python", "java", "kotlin", "csharp",
				"cpp", "rust", "php", "ruby", "swift", "shell",
			},
			Docs: []string{"markdown", "vault", "html", "txt"},
			Frameworks: []string{
				"react", "vue", "svelte", "astro", "angular", "routes", "mdx",
			},
			MaxFileBytes: 512 * 1024,
			Vector:       DefaultGraphVectorConfig(),
			Embed:        DefaultGraphEmbedConfig(),
		},
		Vault: VaultConfig{
			Path:         ".toolbelt/vault",
			SyncOnChange: true,
			MaxNoteBytes: 256 << 10,
			Types:        []string{"fact", "decision", "context", "log"},
			Graph:        true,
		},
		Logging: LoggingConfig{
			Level:  "info",
			File:   ".toolbelt/logs/tb.log",
			Format: "text",
		},
		Bench: BenchConfig{
			TasksDir:     "bench/tasks",
			FixturesDir:  "bench/fixtures",
			OutputDir:    "bench/results",
			DefaultAgent: "",
			Repetitions:  5,
			Timeout:      "10m",
		},
		Verify: VerifyConfig{
			TestCmd:       "go test ./...",
			HiddenTestCmd: "",
		},
		Router: DefaultRouterConfig(),
	}
}

// When non-empty, Load uses only this file (-c/--config).
var overrideFile string

// SetFile("") restores the global+local merge.
func SetFile(path string) {
	overrideFile = strings.TrimSpace(path)
}

func File() string {
	return overrideFile
}

// Load merges global then local; after SetFile(path) it loads only that file
// over the defaults.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	if overrideFile != "" {
		if err := loadFromFile(overrideFile, cfg); err != nil {
			return nil, fmt.Errorf("loading config %s: %w", overrideFile, err)
		}
		return cfg, nil
	}

	globalPath := GlobalConfigPath()
	if err := loadFromFile(globalPath, cfg); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("loading global config: %w", err)
	}

	// Local overrides global, except graph.vector and graph.embed: they pick
	// what native code loads, so the repository layer never sets them.
	vector, embed := cfg.Graph.Vector, cfg.Graph.Embed
	localPath := LocalConfigPath()
	if err := loadFromFile(localPath, cfg); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("loading local config: %w", err)
	}
	cfg.Graph.Vector, cfg.Graph.Embed = vector, embed

	return cfg, nil
}

// LoadTrusted loads only config an agent working in the repository cannot
// write: the -c/--config file when set, else the global config. The local
// .toolbelt/config.yaml lives inside the repository, so MCP security knobs
// (env.pass, sandbox.read/write/allow_unsandboxed) never come from it.
func LoadTrusted() (*Config, error) {
	cfg := DefaultConfig()
	path := overrideFile
	if path == "" {
		path = GlobalConfigPath()
	}
	if err := loadFromFile(path, cfg); err != nil && !(overrideFile == "" && errors.Is(err, os.ErrNotExist)) {
		return nil, fmt.Errorf("loading trusted config %s: %w", path, err)
	}
	return cfg, nil
}

func GlobalConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.Getenv("HOME")
	}
	return filepath.Join(configDir, "toolbelt", DefaultConfigFile)
}

func LocalConfigPath() string {
	return filepath.Join(DefaultConfigDir, DefaultConfigFile)
}

func loadFromFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

func LoadFile(path string) (*Config, error) {
	cfg := DefaultConfig()
	if err := loadFromFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Save(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	return os.WriteFile(path, data, VersionedFileMode) //nolint:gosec // config.yaml é versionável, sem segredos
}

// Init: dir is the full config dir path (e.g. ~/.config/toolbelt or /project/.toolbelt).
func Init(dir string) error {
	_, err := InitDetailed(dir)
	return err
}

// InitDetailed is like Init but also returns toolchain detection details (PATH warnings).
func InitDetailed(dir string) (DetectedCmd, error) {
	var detected DetectedCmd
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return detected, fmt.Errorf("creating config dir: %w", err)
	}

	subdirs := []string{"vault", "logs", "bench"}
	for _, sub := range subdirs {
		subpath := filepath.Join(dir, sub)
		if err := os.MkdirAll(subpath, 0o755); err != nil {
			return detected, fmt.Errorf("creating %s dir: %w", sub, err)
		}
	}

	configPath := filepath.Join(dir, DefaultConfigFile)
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		return detected, nil
	}

	cfg := DefaultConfig()
	// Left out so a repository config written by init never sets user-only
	// keys (which would draw config_untrusted warnings); defaults apply.
	cfg.Graph.Vector, cfg.Graph.Embed = GraphVectorConfig{}, GraphEmbedConfig{}
	detected = applyDetectedVerifyCmds(cfg, dir)
	if err := Save(cfg, configPath); err != nil {
		return detected, fmt.Errorf("saving default config: %w", err)
	}

	return detected, nil
}

func applyDetectedVerifyCmds(cfg *Config, dir string) DetectedCmd {
	var detected DetectedCmd
	root, ok := projectRootForInit(dir)
	if !ok {
		return detected
	}
	detected = DetectTestCmdDetailed(root)
	cfg.Verify.TestCmd = detected.Cmd
	if cmd := DetectBuildCmd(root); cmd != "" {
		cfg.Verify.BuildCmd = cmd
	}
	if cmd := DetectLintCmd(root); cmd != "" {
		cfg.Verify.LintCmd = cmd
	}
	if detected.Cmd == "" && detected.Warning == "" {
		detected.Warning = "no test toolchain detected; verify.test_cmd left empty (does not use go test without go.mod)"
	}
	return detected
}

// Exists: dir is the full path (e.g. /project/.toolbelt).
func Exists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}
