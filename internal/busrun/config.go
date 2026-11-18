// Package busrun implementa o harness do barramento (`tb bus run`, ADR-013
// etapa B3): espera mensagens sem LLM e acorda o agente via adapter.
package busrun

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
)

// DefaultWakeOn são os tipos que acordam o agente por padrão.
var DefaultWakeOn = []bus.Type{bus.TypeTaskAssign, bus.TypeQuery, bus.TypeReply, bus.TypeUrgent, bus.TypeTaskResult}

// Config é o arquivo de wake (.toolbelt/bus/wake/<canal>/<nome>.yaml).
type Config struct {
	Adapter     string        `yaml:"adapter"`
	WakeOn      []bus.Type    `yaml:"wake_on"`
	WakeTopics  *bool         `yaml:"wake_topics"` // nil = true; false: topic:* nunca acorda
	Coalesce    Coalesce      `yaml:"coalesce"`
	Budget      Budget        `yaml:"budget"`
	RewakeAfter time.Duration `yaml:"rewake_after"`
	Timeout     time.Duration `yaml:"timeout"`
	PassEnv     []string      `yaml:"pass_env"`
	SecretFiles []string      `yaml:"secret_files"` // KEY=VALOR cujos valores o audit redige
	Sandbox     SandboxPaths  `yaml:"sandbox"`
	Exec        ExecConfig    `yaml:"exec"`
	Harness     HarnessConfig `yaml:"harness"`
}

// Coalesce agrupa mensagens próximas num despertar.
type Coalesce struct {
	Debounce time.Duration `yaml:"debounce"`
	MaxDelay time.Duration `yaml:"max_delay"`
}

// Budget limita despertares por janela.
type Budget struct {
	MaxWakes int           `yaml:"max_wakes"`
	Per      time.Duration `yaml:"per"`
}

// SandboxPaths são caminhos extras liberados ao filho no Landlock.
type SandboxPaths struct {
	Read  []string `yaml:"read"`
	Write []string `yaml:"write"`
}

// ExecConfig configura o adapter exec.
type ExecConfig struct {
	Argv []string `yaml:"argv"`
}

// HarnessConfig configura o adapter harness.
type HarnessConfig struct {
	Vendor         string   `yaml:"vendor"`
	Command        string   `yaml:"command"`
	Session        string   `yaml:"session"`
	PermissionMode string   `yaml:"permission_mode"`
	AllowedTools   []string `yaml:"allowed_tools"`
	Model          string   `yaml:"model"`
	Workdir        string   `yaml:"workdir"`
}

// DefaultAllowedTools são as ferramentas liberadas ao agente acordado.
var DefaultAllowedTools = []string{
	"mcp__toolbelt__tb_bus_recv", "mcp__toolbelt__tb_bus_send",
	"mcp__toolbelt__tb_bus_claim", "mcp__toolbelt__tb_bus_complete",
}

// ConfigPath devolve o caminho padrão do arquivo de wake
// (<privado>/wake/<canal>/<nome>.yaml, fora do repositório).
func ConfigPath(root, channel, name string) (string, error) {
	priv, err := bus.PrivateDir(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(priv, "wake", channel, name+".yaml"), nil
}

// LoadConfig lê, aplica padrões e valida. Recusa arquivo com permissão
// mais aberta que 0600.
func LoadConfig(path string) (*Config, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("arquivo de wake: %w", err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("arquivo de wake %s com permissão %o: use chmod 600", path, st.Mode().Perm())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("arquivo de wake %s: %w", path, err)
	}
	c.applyDefaults()
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("arquivo de wake %s: %w", path, err)
	}
	return &c, nil
}

func (c *Config) applyDefaults() {
	if len(c.WakeOn) == 0 {
		c.WakeOn = append([]bus.Type(nil), DefaultWakeOn...)
	}
	if c.Coalesce.Debounce == 0 {
		c.Coalesce.Debounce = 3 * time.Second
	}
	if c.Coalesce.MaxDelay == 0 {
		c.Coalesce.MaxDelay = 30 * time.Second
	}
	if c.Budget.MaxWakes == 0 {
		c.Budget.MaxWakes = 6
	}
	if c.Budget.Per == 0 {
		c.Budget.Per = time.Hour
	}
	if c.RewakeAfter == 0 {
		c.RewakeAfter = 10 * time.Minute
	}
	if c.Timeout == 0 {
		if c.Adapter == AdapterHarness {
			c.Timeout = 15 * time.Minute
		} else {
			c.Timeout = 30 * time.Second
		}
	}
	if c.Adapter == AdapterHarness {
		if c.Harness.PermissionMode == "" {
			c.Harness.PermissionMode = "default"
		}
		if len(c.Harness.AllowedTools) == 0 {
			c.Harness.AllowedTools = append([]string(nil), DefaultAllowedTools...)
		}
	}
}

func (c *Config) topics() bool { return c.WakeTopics == nil || *c.WakeTopics }

// Adapters suportados.
const (
	AdapterExec    = "exec"
	AdapterHarness = "harness"
)

var permissionModes = map[string]bool{"default": true, "acceptEdits": true, "plan": true}

// Validate checa a config (já com padrões).
func (c *Config) Validate() error {
	for _, t := range c.WakeOn {
		if _, err := bus.ParseType(string(t)); err != nil {
			return err
		}
	}
	if c.Coalesce.Debounce < 0 || c.Coalesce.MaxDelay < c.Coalesce.Debounce {
		return errors.New("coalesce: debounce ≥ 0 e max_delay ≥ debounce")
	}
	if c.Budget.MaxWakes < 1 || c.Budget.Per <= 0 || c.RewakeAfter <= 0 || c.Timeout <= 0 {
		return errors.New("budget.max_wakes, budget.per, rewake_after e timeout precisam ser positivos")
	}
	if err := absPaths("secret_files", c.SecretFiles); err != nil {
		return err
	}
	if err := absPaths("sandbox", append(append([]string{}, c.Sandbox.Read...), c.Sandbox.Write...)); err != nil {
		return err
	}
	switch c.Adapter {
	case AdapterExec:
		if len(c.Exec.Argv) == 0 || !filepath.IsAbs(c.Exec.Argv[0]) {
			return errors.New("exec.argv[0] precisa ser caminho absoluto")
		}
	case AdapterHarness:
		return c.Harness.validate()
	default:
		return fmt.Errorf("adapter %q inválido (use exec ou harness)", c.Adapter)
	}
	return nil
}

func absPaths(field string, paths []string) error {
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			return fmt.Errorf("%s: caminho %q precisa ser absoluto", field, p)
		}
	}
	return nil
}

func (h *HarnessConfig) validate() error {
	if h.Vendor != "claude" {
		return fmt.Errorf("harness.vendor %q não suportado (presets: claude)", h.Vendor)
	}
	if h.Command != "" && !filepath.IsAbs(h.Command) {
		return errors.New("harness.command precisa ser caminho absoluto")
	}
	if !permissionModes[h.PermissionMode] {
		return fmt.Errorf("harness.permission_mode %q recusado (use default, acceptEdits ou plan; bypassPermissions nunca)", h.PermissionMode)
	}
	if h.Workdir != "" && !filepath.IsAbs(h.Workdir) {
		return errors.New("harness.workdir precisa ser caminho absoluto")
	}
	return nil
}
