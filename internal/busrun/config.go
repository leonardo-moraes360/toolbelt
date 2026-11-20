// Package busrun is the bus harness (`tb bus run`, ADR-013 stage B3): it waits
// for messages without an LLM and wakes the agent through an adapter.
package busrun

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
)

var DefaultWakeOn = []bus.Type{bus.TypeTaskAssign, bus.TypeQuery, bus.TypeReply, bus.TypeUrgent, bus.TypeTaskResult}

// Config is the wake file (see ConfigPath).
type Config struct {
	Adapter     string        `yaml:"adapter"`
	WakeOn      []bus.Type    `yaml:"wake_on"`
	WakeTopics  *bool         `yaml:"wake_topics"` // nil = true; false: topic:* never wakes
	Coalesce    Coalesce      `yaml:"coalesce"`
	Budget      Budget        `yaml:"budget"`
	RewakeAfter time.Duration `yaml:"rewake_after"`
	Timeout     time.Duration `yaml:"timeout"`
	PassEnv     []string      `yaml:"pass_env"`
	SecretFiles []string      `yaml:"secret_files"` // KEY=VALUE files whose values the audit redacts
	Sandbox     SandboxPaths  `yaml:"sandbox"`
	Exec        ExecConfig    `yaml:"exec"`
	Harness     HarnessConfig `yaml:"harness"`
	Notify      NotifyConfig  `yaml:"notify"`
	// RouterTasks: tasks from the router run confined to the ticket worktree,
	// with the harness doing the bus work (docs/bus-router.md, B4a).
	RouterTasks *RouterTasksConfig `yaml:"router_tasks"`
}

type RouterTasksConfig struct {
	From         string        `yaml:"from"`          // the only router agent whose tasks are honored
	Timeout      time.Duration `yaml:"timeout"`       // cap per task (default 30m)
	AllowedTools []string      `yaml:"allowed_tools"` // claude preset; never tb_bus_*
}

// DefaultRouterTaskTools: file tools only; no shell and no bus tools.
var DefaultRouterTaskTools = []string{"Read", "Edit", "Write", "Glob", "Grep"}

// Coalesce batches nearby messages into one wake.
type Coalesce struct {
	Debounce time.Duration `yaml:"debounce"`
	MaxDelay time.Duration `yaml:"max_delay"`
}

// Budget caps wakes per window.
type Budget struct {
	MaxWakes int           `yaml:"max_wakes"`
	Per      time.Duration `yaml:"per"`
}

// SandboxPaths are extra paths Landlock grants the child.
type SandboxPaths struct {
	Read  []string `yaml:"read"`
	Write []string `yaml:"write"`
}

type ExecConfig struct {
	Argv []string `yaml:"argv"`
}

type HarnessConfig struct {
	Vendor         string   `yaml:"vendor"`
	Command        string   `yaml:"command"`
	Session        string   `yaml:"session"`
	PermissionMode string   `yaml:"permission_mode"`
	AllowedTools   []string `yaml:"allowed_tools"`
	Model          string   `yaml:"model"`
	Workdir        string   `yaml:"workdir"`
}

// NotifyConfig: the notify adapter pings the human, never the agent
// (docs/bus-notify-discover.md).
type NotifyConfig struct {
	Backend       string `yaml:"backend"`        // desktop | terminal
	Command       string `yaml:"command"`        // desktop: absolute path; empty = default list
	Urgency       string `yaml:"urgency"`        // low | normal | critical
	UrgentUrgency string `yaml:"urgent_urgency"` // urgency for batches with an urgent message
}

var DefaultAllowedTools = []string{
	"mcp__toolbelt__tb_bus_recv", "mcp__toolbelt__tb_bus_send",
	"mcp__toolbelt__tb_bus_claim", "mcp__toolbelt__tb_bus_complete",
}

// ConfigPath: <private>/wake/<channel>/<name>.yaml, outside the repo.
func ConfigPath(root, channel, name string) (string, error) {
	priv, err := bus.PrivateDir(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(priv, "wake", channel, name+".yaml"), nil
}

// LoadConfig rejects a file with permissions looser than 0600.
func LoadConfig(path string) (*Config, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("wake file: %w", err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("wake file %s has permission %o: use chmod 600", path, st.Mode().Perm())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("wake file %s: %w", path, err)
	}
	c.applyDefaults()
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("wake file %s: %w", path, err)
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
	c.timingDefaults()
	c.adapterDefaults()
	c.routerTaskDefaults()
}

func (c *Config) routerTaskDefaults() {
	rt := c.RouterTasks
	if rt == nil {
		return
	}
	if rt.Timeout == 0 {
		rt.Timeout = 30 * time.Minute
	}
	if len(rt.AllowedTools) == 0 {
		rt.AllowedTools = append([]string(nil), DefaultRouterTaskTools...)
	}
}

// timingDefaults: notify waits longer for a human and times out sooner.
func (c *Config) timingDefaults() {
	if c.RewakeAfter == 0 {
		c.RewakeAfter = 10 * time.Minute
		if c.Adapter == AdapterNotify {
			c.RewakeAfter = time.Hour // a human may take a while
		}
	}
	if c.Timeout == 0 && c.RouterTasks != nil {
		c.Timeout = time.Hour // tasks are capped by router_tasks.timeout and the payload's
	}
	if c.Timeout == 0 {
		switch c.Adapter {
		case AdapterHarness:
			c.Timeout = 15 * time.Minute
		case AdapterNotify:
			c.Timeout = 10 * time.Second
		default:
			c.Timeout = 30 * time.Second
		}
	}
}

func (c *Config) adapterDefaults() {
	if c.Adapter == AdapterNotify {
		if c.Notify.Urgency == "" {
			c.Notify.Urgency = "normal"
		}
		if c.Notify.UrgentUrgency == "" {
			c.Notify.UrgentUrgency = "critical"
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

const (
	AdapterExec    = "exec"
	AdapterHarness = "harness"
	AdapterNotify  = "notify"

	BackendDesktop  = "desktop"
	BackendTerminal = "terminal"
)

var urgencies = map[string]bool{"low": true, "normal": true, "critical": true}

var permissionModes = map[string]bool{"default": true, "acceptEdits": true, "plan": true}

// Validate expects defaults already applied.
func (c *Config) Validate() error {
	if err := c.validateCommon(); err != nil {
		return err
	}
	switch c.Adapter {
	case AdapterExec, AdapterHarness, AdapterNotify:
	default:
		return fmt.Errorf("adapter %q invalid (use exec, harness or notify)", c.Adapter)
	}
	if err := c.foreignBlocks(); err != nil {
		return err
	}
	if err := c.validateRouterTasks(); err != nil {
		return err
	}
	switch c.Adapter {
	case AdapterExec:
		if len(c.Exec.Argv) == 0 || !filepath.IsAbs(c.Exec.Argv[0]) {
			return errors.New("exec.argv[0] must be an absolute path")
		}
	case AdapterHarness:
		return c.Harness.validate()
	case AdapterNotify:
		return c.validateNotify()
	}
	return nil
}

func (c *Config) validateCommon() error {
	for _, t := range c.WakeOn {
		if _, err := bus.ParseType(string(t)); err != nil {
			return err
		}
	}
	if c.Coalesce.Debounce < 0 || c.Coalesce.MaxDelay < c.Coalesce.Debounce {
		return errors.New("coalesce: debounce >= 0 and max_delay >= debounce")
	}
	if c.Budget.MaxWakes < 1 || c.Budget.Per <= 0 || c.RewakeAfter <= 0 || c.Timeout <= 0 {
		return errors.New("budget.max_wakes, budget.per, rewake_after and timeout must be positive")
	}
	if err := absPaths("secret_files", c.SecretFiles); err != nil {
		return err
	}
	return absPaths("sandbox", append(append([]string{}, c.Sandbox.Read...), c.Sandbox.Write...))
}

// foreignBlocks refuses mixing notify with another adapter's block: a
// notify: block elsewhere, or an exec:/harness: block with notify. Mixes of
// exec and harness stay accepted, as before notify existed.
func (c *Config) foreignBlocks() error {
	if c.Adapter != AdapterNotify {
		if c.Notify != (NotifyConfig{}) {
			return fmt.Errorf("notify: block not allowed with adapter %q", c.Adapter)
		}
		return nil
	}
	if len(c.Exec.Argv) > 0 {
		return errors.New("exec: block not allowed with adapter notify")
	}
	if !reflect.DeepEqual(c.Harness, HarnessConfig{}) {
		return errors.New("harness: block not allowed with adapter notify")
	}
	return nil
}

// goos is the platform the notify backend targets (tests override it).
var goos = runtime.GOOS

func (c *Config) validateNotify() error {
	if len(c.PassEnv) > 0 || len(c.SecretFiles) > 0 || len(c.Sandbox.Read) > 0 || len(c.Sandbox.Write) > 0 {
		return errors.New("pass_env, secret_files and sandbox are not allowed with adapter notify")
	}
	n := c.Notify
	if !urgencies[n.Urgency] || !urgencies[n.UrgentUrgency] {
		return errors.New("notify.urgency and notify.urgent_urgency must be low, normal or critical")
	}
	return n.validateBackend()
}

func (n *NotifyConfig) validateBackend() error {
	switch n.Backend {
	case BackendTerminal:
		if n.Command != "" {
			return errors.New("notify.command is only for backend desktop")
		}
	case BackendDesktop:
		if goos != "linux" && goos != "darwin" {
			return fmt.Errorf("notify_unsupported: backend desktop is not available on %s (use terminal)", goos)
		}
		if n.Command != "" && !filepath.IsAbs(n.Command) {
			return errors.New("notify.command must be an absolute path")
		}
		if goos == "darwin" && n.Command != "" {
			return errors.New("notify.command is not used on macOS (always /usr/bin/osascript)")
		}
	case "":
		return errors.New("notify.backend is required (desktop or terminal)")
	default:
		return fmt.Errorf("notify.backend %q invalid (use desktop or terminal)", n.Backend)
	}
	return nil
}

func absPaths(field string, paths []string) error {
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			return fmt.Errorf("%s: path %q must be absolute", field, p)
		}
	}
	return nil
}

func (h *HarnessConfig) validate() error {
	if h.Vendor != "claude" {
		return fmt.Errorf("harness.vendor %q not supported (presets: claude)", h.Vendor)
	}
	if h.Command != "" && !filepath.IsAbs(h.Command) {
		return errors.New("harness.command must be an absolute path")
	}
	if !permissionModes[h.PermissionMode] {
		return fmt.Errorf("harness.permission_mode %q refused (use default, acceptEdits or plan; never bypassPermissions)", h.PermissionMode)
	}
	if h.Workdir != "" && !filepath.IsAbs(h.Workdir) {
		return errors.New("harness.workdir must be an absolute path")
	}
	return nil
}

func (c *Config) validateRouterTasks() error {
	rt := c.RouterTasks
	if rt == nil {
		return nil
	}
	if c.Adapter == AdapterNotify {
		return errors.New("router_tasks is not allowed with adapter notify (the agent cannot be confined)")
	}
	if err := bus.ValidName("router_tasks.from", rt.From); err != nil {
		return err
	}
	if rt.Timeout <= 0 {
		return errors.New("router_tasks.timeout must be positive")
	}
	for _, t := range rt.AllowedTools {
		if strings.HasPrefix(t, "mcp__") {
			return fmt.Errorf("router_tasks.allowed_tools: %q not allowed (a confined task has no MCP or bus tools)", t)
		}
	}
	return nil
}
