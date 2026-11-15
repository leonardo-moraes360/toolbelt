// Package config gerencia a configuração do toolbelt.
// Suporta configuração global (~/.config/toolbelt/config.yaml) e local (.toolbelt/config.yaml),
// onde local sobrepõe global.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	AutoIndex      bool `yaml:"auto_index"`
	IndexOnChange  bool `yaml:"index_on_change"`
	InferRelations bool `yaml:"infer_relations"`
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
			OutputDir:    ".toolbelt/bench",
			DefaultAgent: "",
			Repetitions:  5,
			Timeout:      "10m",
		},
		Verify: VerifyConfig{
			TestCmd:       "go test ./...",
			HiddenTestCmd: "go test ./tests/hidden/...",
		},
	}
}

// Load carrega a configuração, mesclando global e local.
func Load() (*Config, error) {
	cfg := DefaultConfig()

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

	return os.WriteFile(path, data, 0o600)
}

// Init cria o diretório de configuração e o arquivo de configuração padrão.
// dir é o caminho completo do diretório (ex: /home/user/.config/toolbelt ou /project/.toolbelt)
func Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	// Cria subdiretórios
	subdirs := []string{"vault", "logs", "bench"}
	for _, sub := range subdirs {
		subpath := filepath.Join(dir, sub)
		if err := os.MkdirAll(subpath, 0o755); err != nil {
			return fmt.Errorf("creating %s dir: %w", sub, err)
		}
	}

	// Salva configuração padrão
	configPath := filepath.Join(dir, DefaultConfigFile)
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		cfg := DefaultConfig()
		if err := Save(cfg, configPath); err != nil {
			return fmt.Errorf("saving default config: %w", err)
		}
	}

	return nil
}

// Exists verifica se o diretório de configuração existe.
// dir é o caminho completo (ex: /project/.toolbelt)
func Exists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}
