// Package config gerencia a configuração do toolbelt.
// Suporta configuração global (~/.config/toolbelt/config.yaml) e local (.toolbelt/config.yaml),
// onde local sobrepõe global.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultConfigDir é o nome do diretório de configuração local.
const DefaultConfigDir = ".toolbelt"

// DefaultConfigFile é o nome do arquivo de configuração.
const DefaultConfigFile = "config.yaml"

// Config representa a configuração do toolbelt.
type Config struct {
	// Defaults são valores padrão para commands.
	Defaults Defaults `yaml:"defaults"`

	// Sandbox configura o isolamento.
	Sandbox SandboxConfig `yaml:"sandbox"`

	// Graph configura o grafo de código.
	Graph GraphConfig `yaml:"graph"`

	// Vault configura a memória.
	Vault VaultConfig `yaml:"vault"`

	// Logging configura os logs.
	Logging LoggingConfig `yaml:"logging"`

	// Bench configura o benchmark.
	Bench BenchConfig `yaml:"bench"`

	// Verify configura a verificação adversarial.
	Verify VerifyConfig `yaml:"verify"`

	// Review configura review/drift (approved-sha).
	Review ReviewConfig `yaml:"review"`

	// Env configura a allowlist de variáveis passadas a filhos (sh -c).
	Env EnvConfig `yaml:"env"`

	// Router configura o scheduler de tickets (tb router run).
	Router RouterConfig `yaml:"router"`
}

// Defaults são valores padrão para commands.
type Defaults struct {
	Sandbox    bool   `yaml:"sandbox"`
	Timeout    string `yaml:"timeout"`
	JSONOutput bool   `yaml:"json_output"`
}

// SandboxConfig configura o isolamento.
type SandboxConfig struct {
	AllowNet  []string `yaml:"allow_net"`
	CPULimit  int      `yaml:"cpu_limit"`
	MemLimit  string   `yaml:"mem_limit"`
	TimeLimit string   `yaml:"time_limit"`
}

// GraphConfig configura o grafo de código.
type GraphConfig struct {
	AutoIndex      bool     `yaml:"auto_index"`
	IndexOnChange  bool     `yaml:"index_on_change"`
	InferRelations bool     `yaml:"infer_relations"`
	Languages      []string `yaml:"languages"`
	Docs           []string `yaml:"docs"`
	Frameworks     []string `yaml:"frameworks"`
	MaxFileBytes   int64    `yaml:"max_file_bytes"`
}

// VaultConfig configura a memória.
type VaultConfig struct {
	Path         string `yaml:"path"`
	SyncOnChange bool   `yaml:"sync_on_change"`
}

// LoggingConfig configura os logs.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	File   string `yaml:"file"`
	Format string `yaml:"format"`
}

// BenchConfig configura o benchmark.
type BenchConfig struct {
	TasksDir     string `yaml:"tasks_dir"`
	FixturesDir  string `yaml:"fixtures_dir"`
	OutputDir    string `yaml:"output_dir"`
	DefaultAgent string `yaml:"default_agent"`
	Repetitions  int    `yaml:"repetitions"`
	Timeout      string `yaml:"timeout"`
}

// VerifyConfig configura o ruler de testes do tb verify.
type VerifyConfig struct {
	TestCmd       string `yaml:"test_cmd"`
	HiddenTestCmd string `yaml:"hidden_test_cmd"`
	BuildCmd      string `yaml:"build_cmd,omitempty"`
	LintCmd       string `yaml:"lint_cmd,omitempty"`
}

// ReviewConfig configura tb review / tb drift.
type ReviewConfig struct {
	RequireApprovedSHA bool `yaml:"require_approved_sha"`
}

// EnvConfig controla quais variáveis de ambiente chegam aos filhos (sh -c).
type EnvConfig struct {
	Allow []string `yaml:"allow"`
	Pass  []string `yaml:"pass"`
}

// RouterConfig configura tb router / tb watchdog.
type RouterConfig struct {
	MaxWorkers     int                  `yaml:"max_workers" json:"max_workers"`
	Strategy       string               `yaml:"strategy" json:"strategy"`
	AgentCmd       []string             `yaml:"agent_cmd" json:"agent_cmd"`
	DirectDelivery RouterDirectDelivery `yaml:"direct_delivery" json:"direct_delivery"`
	Timeouts       RouterTimeouts       `yaml:"timeouts" json:"timeouts"`
	Retry          RouterRetry          `yaml:"retry" json:"retry"`
	SlowThreshold  float64              `yaml:"slow_threshold" json:"slow_threshold"`
}

// RouterDirectDelivery configura a entrega direta (afinidade ticket -> worker).
type RouterDirectDelivery struct {
	Enabled                   bool   `yaml:"enabled" json:"enabled"`
	SubscriptionTTL           string `yaml:"subscription_ttl" json:"subscription_ttl"`
	MaxSubscriptionsPerWorker int    `yaml:"max_subscriptions_per_worker" json:"max_subscriptions_per_worker"`
}

// RouterTimeouts limita a duração por ticket e a execução global do router.
type RouterTimeouts struct {
	PerTicket string `yaml:"per_ticket" json:"per_ticket"`
	Global    string `yaml:"global" json:"global"`
}

// RouterRetry controla re-tentativas após falha de worker.
type RouterRetry struct {
	MaxAttempts int `yaml:"max_attempts" json:"max_attempts"`
}

// DefaultRouterConfig devolve os padrões do router.
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

// Validate checa os valores do router (strategy, durações, limites).
func (r *RouterConfig) Validate() error {
	if r.MaxWorkers < 1 {
		return fmt.Errorf("router.max_workers deve ser >= 1 (atual %d)", r.MaxWorkers)
	}
	switch r.Strategy {
	case "", "fifo", "priority", "dependency":
	default:
		return fmt.Errorf("router.strategy inválida %q (use fifo, priority ou dependency)", r.Strategy)
	}
	for name, v := range map[string]string{
		"router.direct_delivery.subscription_ttl": r.DirectDelivery.SubscriptionTTL,
		"router.timeouts.per_ticket":              r.Timeouts.PerTicket,
		"router.timeouts.global":                  r.Timeouts.Global,
	} {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return fmt.Errorf("%s inválido %q (ex.: 10m)", name, v)
		}
	}
	if r.DirectDelivery.MaxSubscriptionsPerWorker < 1 {
		return fmt.Errorf("router.direct_delivery.max_subscriptions_per_worker deve ser >= 1")
	}
	if r.Retry.MaxAttempts < 1 {
		return fmt.Errorf("router.retry.max_attempts deve ser >= 1")
	}
	if r.SlowThreshold <= 0 || r.SlowThreshold > 1 {
		return fmt.Errorf("router.slow_threshold deve estar em (0, 1] (atual %v)", r.SlowThreshold)
	}
	return nil
}

// DefaultConfig retorna a configuração padrão.
func DefaultConfig() *Config {
	return &Config{
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
			Docs: []string{"markdown", "html", "txt"},
			Frameworks: []string{
				"react", "vue", "svelte", "astro", "angular", "routes", "mdx",
			},
			MaxFileBytes: 512 * 1024,
		},
		Vault: VaultConfig{
			Path:         ".toolbelt/vault",
			SyncOnChange: true,
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

// overrideFile, quando não vazio, faz Load usar só esse arquivo (flag -c/--config).
var overrideFile string

// SetFile define o arquivo de configuração explícito (ex.: -c/--config).
// Passe "" para limpar e voltar ao merge global+local.
func SetFile(path string) {
	overrideFile = strings.TrimSpace(path)
}

// File retorna o caminho explícito definido por SetFile, ou "".
func File() string {
	return overrideFile
}

// Load carrega a configuração, mesclando global e local.
// Se SetFile tiver sido chamado com um path, carrega só esse arquivo sobre os defaults.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	if overrideFile != "" {
		if err := loadFromFile(overrideFile, cfg); err != nil {
			return nil, fmt.Errorf("loading config %s: %w", overrideFile, err)
		}
		return cfg, nil
	}

	// Carrega configuração global
	globalPath := GlobalConfigPath()
	if err := loadFromFile(globalPath, cfg); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("loading global config: %w", err)
	}

	// Carrega configuração local (sobrepõe global)
	localPath := LocalConfigPath()
	if err := loadFromFile(localPath, cfg); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("loading local config: %w", err)
	}

	return cfg, nil
}

// GlobalConfigPath retorna o caminho da configuração global.
func GlobalConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.Getenv("HOME")
	}
	return filepath.Join(configDir, "toolbelt", DefaultConfigFile)
}

// LocalConfigPath retorna o caminho da configuração local.
func LocalConfigPath() string {
	return filepath.Join(DefaultConfigDir, DefaultConfigFile)
}

// loadFromFile carrega configuração de um arquivo YAML.
func loadFromFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

// LoadFile carrega um arquivo de configuração YAML específico.
func LoadFile(path string) (*Config, error) {
	cfg := DefaultConfig()
	if err := loadFromFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save salva a configuração em um arquivo.
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

// Init cria o diretório de configuração e o arquivo de configuração padrão.
// dir é o caminho completo do diretório (ex: /home/user/.config/toolbelt ou /project/.toolbelt)
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
		detected.Warning = "nenhuma toolchain de teste detectada; verify.test_cmd ficou vazio (não usa go test sem go.mod)"
	}
	return detected
}

// Exists verifica se o diretório de configuração existe.
// dir é o caminho completo (ex: /project/.toolbelt)
func Exists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}
