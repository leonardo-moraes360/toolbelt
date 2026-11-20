// Package busdiscover builds the read-only catalog of `tb bus discover`
// (docs/bus-notify-discover.md, part 2): known agent CLIs on PATH and the
// MCP client configs that run `tb mcp`. It executes nothing, opens no
// connection and never prints another server's command, args or env.
package busdiscover

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	Version = "bus-discover/0.1"
	// maxConfigBytes caps an MCP config read.
	maxConfigBytes = 4 << 20
)

// Kinds of an inspected config path.
const (
	KindFile     = "file"
	KindMissing  = "missing"
	KindSymlink  = "symlink"
	KindOther    = "other"
	KindTooLarge = "too_large"
)

// KnownCLI is one row of the built-in table; a CLI outside it is never reported.
type KnownCLI struct {
	Name     string
	Vendor   string
	Headless bool
	Preset   string
	Wake     string
}

var KnownCLIs = []KnownCLI{
	{Name: "aider", Vendor: "aider", Headless: false, Wake: "manual"},
	{Name: "claude", Vendor: "anthropic", Headless: true, Preset: "claude", Wake: "harness"},
	{Name: "codex", Vendor: "openai", Headless: true, Wake: "exec"},
	{Name: "gemini", Vendor: "google", Headless: true, Wake: "exec"},
	{Name: "opencode", Vendor: "sst", Headless: true, Wake: "exec"},
}

type Options struct {
	PathEnv string // PATH to search
	Home    string
	Cwd     string // --cwd; empty = repository root of WorkDir
	WorkDir string // current directory
	Self    string // the running tb, for same_tb
	GOOS    string
}

type Catalog struct {
	V                  string      `json:"v"`
	TB                 string      `json:"tb"`
	SkippedPathEntries []string    `json:"skipped_path_entries"`
	CLIs               []CLI       `json:"clis"`
	MCPConfigs         []MCPConfig `json:"mcp_configs"`
	Project            *string     `json:"project"`
}

type CLI struct {
	Name             string `json:"name"`
	Vendor           string `json:"vendor"`
	Path             string `json:"path"`
	Target           string `json:"target"`
	WritableByOthers bool   `json:"writable_by_others"`
	Headless         bool   `json:"headless"`
	Preset           string `json:"preset,omitempty"`
	Wake             string `json:"wake"`
}

type MCPConfig struct {
	Client       string     `json:"client"`
	Path         string     `json:"path"`
	Kind         string     `json:"kind"`
	Parses       *bool      `json:"parses,omitempty"`
	Parsed       *bool      `json:"parsed,omitempty"` // codex: existence only
	Scope        string     `json:"scope,omitempty"`
	TB           *[]TBEntry `json:"tb,omitempty"`
	OtherServers *int       `json:"other_servers,omitempty"`
}

type TBEntry struct {
	Server string  `json:"server"`
	Path   string  `json:"path"`
	SameTB *bool   `json:"same_tb"`
	BusAs  *string `json:"bus_as"`
}

// Discover builds the catalog. Errors only for a broken environment; every
// per-file problem is reported in the catalog instead.
func Discover(o *Options) (*Catalog, error) {
	c := &Catalog{V: Version, TB: o.Self, SkippedPathEntries: []string{}, CLIs: []CLI{}, MCPConfigs: []MCPConfig{}}
	c.CLIs, c.SkippedPathEntries = findCLIs(o.PathEnv)
	project := projectDir(o)
	if project != "" {
		c.Project = &project
	}
	for _, t := range configTargets(o, project) {
		c.MCPConfigs = append(c.MCPConfigs, inspect(t, o.Self)...)
	}
	return c, nil
}

func findCLIs(pathEnv string) (clis []CLI, skipped []string) {
	clis, skipped = []CLI{}, []string{}
	var dirs []string
	for _, d := range filepath.SplitList(pathEnv) {
		if d == "" || !filepath.IsAbs(d) {
			skipped = append(skipped, d)
			continue
		}
		dirs = append(dirs, d)
	}
	for _, k := range KnownCLIs {
		for _, d := range dirs {
			p := filepath.Join(d, k.Name)
			st, err := os.Stat(p)
			if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0o111 == 0 {
				continue
			}
			target, err := filepath.EvalSymlinks(p)
			if err != nil {
				target = p
			}
			clis = append(clis, CLI{Name: k.Name, Vendor: k.Vendor, Path: p, Target: target,
				WritableByOthers: st.Mode().Perm()&0o022 != 0, Headless: k.Headless, Preset: k.Preset, Wake: k.Wake})
			break
		}
	}
	sort.Strings(skipped)
	return clis, skipped
}

// projectDir: --cwd as given, else the git repository containing WorkDir.
func projectDir(o *Options) string {
	if o.Cwd != "" {
		if abs, err := filepath.Abs(o.Cwd); err == nil {
			return abs
		}
		return o.Cwd
	}
	dir := o.WorkDir
	for dir != "" {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

type target struct {
	client string
	path   string
	codex  bool
}

func configTargets(o *Options, project string) []target {
	h := o.Home
	ts := []target{{client: "claude-code", path: filepath.Join(h, ".claude.json")}}
	switch o.GOOS {
	case "darwin":
		ts = append(ts, target{client: "claude-desktop", path: filepath.Join(h, "Library", "Application Support", "Claude", "claude_desktop_config.json")})
	case "linux":
		ts = append(ts, target{client: "claude-desktop", path: filepath.Join(h, ".config", "Claude", "claude_desktop_config.json")})
	}
	ts = append(ts,
		target{client: "codex", path: filepath.Join(h, ".codex", "config.toml"), codex: true},
		target{client: "cursor", path: filepath.Join(h, ".cursor", "mcp.json")})
	if project != "" {
		ts = append(ts, target{client: "project", path: filepath.Join(project, ".mcp.json")})
	}
	return ts
}

func inspect(t target, self string) []MCPConfig {
	base := MCPConfig{Client: t.client, Path: t.path}
	raw, kind := safeRead(t.path)
	base.Kind = kind
	if kind != KindFile {
		return []MCPConfig{base}
	}
	if t.codex {
		f := false
		base.Parsed = &f
		return []MCPConfig{base}
	}
	scopes, ok := parseServers(raw, t.client == "claude-code")
	base.Parses = &ok
	if !ok {
		return []MCPConfig{base}
	}
	out := make([]MCPConfig, 0, len(scopes))
	for _, sc := range scopes {
		e := base
		e.Scope = sc.name
		entries, others := classify(sc.servers, self)
		e.TB, e.OtherServers = &entries, &others
		out = append(out, e)
	}
	return out
}

// safeRead never follows a symlink, never opens a non-regular file (a FIFO
// would block) and never reads more than maxConfigBytes.
func safeRead(path string) ([]byte, string) {
	st, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, KindMissing
	case err != nil:
		return nil, KindOther
	case st.Mode()&fs.ModeSymlink != 0:
		return nil, KindSymlink
	case !st.Mode().IsRegular():
		return nil, KindOther
	case st.Size() > maxConfigBytes:
		return nil, KindTooLarge
	}
	f, err := os.OpenFile(path, os.O_RDONLY|openFlags, 0)
	if err != nil {
		return nil, KindOther
	}
	defer f.Close()
	if fst, err := f.Stat(); err != nil || !fst.Mode().IsRegular() {
		return nil, KindOther
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxConfigBytes+1))
	if err != nil {
		return nil, KindOther
	}
	if len(raw) > maxConfigBytes {
		return nil, KindTooLarge
	}
	return raw, KindFile
}

type server struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type scope struct {
	name    string
	servers map[string]json.RawMessage
}

// parseServers returns the mcpServers maps; claude-code also has one per
// project. Parse errors are dropped: they can quote file bytes.
func parseServers(raw []byte, perProject bool) ([]scope, bool) {
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
		Projects   map[string]struct {
			MCPServers map[string]json.RawMessage `json:"mcpServers"`
		} `json:"projects"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return nil, false
	}
	name := ""
	if perProject {
		name = "user"
	}
	scopes := []scope{{name: name, servers: doc.MCPServers}}
	if !perProject {
		return scopes, true
	}
	paths := make([]string, 0, len(doc.Projects))
	for p, v := range doc.Projects {
		if len(v.MCPServers) > 0 {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	for _, p := range paths {
		scopes = append(scopes, scope{name: "project:" + p, servers: doc.Projects[p].MCPServers})
	}
	return scopes, true
}

// classify finds `tb mcp` entries: command an absolute path named tb, or a
// bare tb, with args[0] == "mcp". Shell wrappers are not interpreted.
func classify(servers map[string]json.RawMessage, self string) (entries []TBEntry, others int) {
	entries = []TBEntry{}
	keys := make([]string, 0, len(servers))
	for k := range servers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var s server
		if json.Unmarshal(servers[k], &s) != nil || !isTB(&s) {
			others++
			continue
		}
		e := TBEntry{Server: k, Path: s.Command, BusAs: busAs(s.Args)}
		if filepath.IsAbs(s.Command) {
			same := sameFile(s.Command, self)
			e.SameTB = &same
		}
		entries = append(entries, e)
	}
	return entries, others
}

func isTB(s *server) bool {
	if len(s.Args) == 0 || s.Args[0] != "mcp" {
		return false
	}
	return s.Command == "tb" || (filepath.IsAbs(s.Command) && filepath.Base(s.Command) == "tb")
}

func busAs(args []string) *string {
	for i, a := range args {
		if v, ok := strings.CutPrefix(a, "--bus-as="); ok {
			return &v
		}
		if a == "--bus-as" && i+1 < len(args) {
			v := args[i+1]
			return &v
		}
	}
	return nil
}

func sameFile(a, b string) bool {
	sa, err := os.Stat(a)
	if err != nil {
		return false
	}
	sb, err := os.Stat(b)
	return err == nil && os.SameFile(sa, sb)
}
