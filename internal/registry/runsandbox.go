package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/execenv"
	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

// runShellScript runs the tb_run command as `sh -c` in dir. $1 is the
// directory and $2 the command, so neither is ever spliced into shell text.
const runShellScript = `cd -- "$1" || exit 1; exec sh -c "$2"`

// runInSandbox runs an MCP tb_run command under Landlock (M8ven review,
// finding 8). The child can write only to the repository root, its own temp
// dir and the toolchain caches; it can read the system dirs, PATH, GOROOT and
// whatever the human lists in the global config's sandbox.read/write. $HOME, /tmp and the
// rest of the filesystem stay closed. Without Landlock it fails closed unless
// config sets sandbox.allow_unsandboxed.
func runInSandbox(ctx context.Context, command, cwdArg string, timeout time.Duration, envOpts execenv.Options) (map[string]any, error) {
	// Sandbox knobs come only from config the child cannot write: the
	// repository (and its .toolbelt/config.yaml) is writable inside the sandbox.
	cfg, err := config.LoadTrusted()
	if err != nil {
		return nil, err
	}

	root, dir, err := runDirs(cwdArg)
	if err != nil {
		return nil, err
	}

	tmp, err := os.MkdirTemp("", "tb-run-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	env := execenv.Environ(envOpts)
	env = append(env, "TMPDIR="+tmp, "TMP="+tmp, "TEMP="+tmp)

	paths, err := runSandboxPaths(&cfg.Sandbox, root, env)
	if err != nil {
		return nil, err
	}
	paths = append(paths, sandbox.PathAccess{Path: tmp})

	sb, err := sandbox.New(sandbox.Config{
		WorkDir:          root,
		AllowedPaths:     paths,
		Timeout:          timeout,
		AllowUnsandboxed: cfg.Sandbox.AllowUnsandboxed,
		Env:              env,
		ProcessGroup:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("tb_run refused: %w (over MCP, opt out with sandbox.allow_unsandboxed: true in the global toolbelt config)", err)
	}
	res, err := sb.Run(ctx, "sh", "-c", runShellScript, "tb-run", dir, command)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"exit_code": res.ExitCode,
		"output":    res.Output,
		"timeout":   res.TimedOut,
		"sandboxed": res.Sandboxed,
	}, nil
}

// runDirs returns the sandbox root (the repository holding .toolbelt, or the
// server's working directory) and the directory to run in, which must be the
// root or below it.
func runDirs(cwdArg string) (root, dir string, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	root, err = realPath(ticket.FindRoot(wd))
	if err != nil {
		return "", "", err
	}
	if cwdArg == "" {
		cwdArg = wd
	} else if !filepath.IsAbs(cwdArg) {
		cwdArg = filepath.Join(wd, cwdArg)
	}
	dir, err = realPath(cwdArg)
	if err != nil {
		return "", "", fmt.Errorf("cwd: %w", err)
	}
	if dir != root && !within(dir, root) {
		return "", "", fmt.Errorf("cwd %s is outside the repository %s: tb_run only runs inside it", dir, root)
	}
	return root, dir, nil
}

// runSandboxPaths lists what the child may touch besides the root.
func runSandboxPaths(sc *config.SandboxConfig, root string, env []string) ([]sandbox.PathAccess, error) {
	home, _ := os.UserHomeDir()
	ps := &pathSet{root: root, seen: map[string]bool{root: true}}
	// Human-listed paths first: a typo must fail loudly, not be skipped.
	if err := ps.addConfigured(sc, home); err != nil {
		return nil, err
	}
	for _, p := range cachePaths(env, home) {
		ps.add(p, false)
	}
	for _, p := range toolchainReadPaths(env, home) {
		ps.add(p, true)
	}
	return ps.out, nil
}

// pathSet collects sandbox paths, skipping duplicates, missing paths and
// anything under the root (already writable).
type pathSet struct {
	root string
	seen map[string]bool
	out  []sandbox.PathAccess
}

func (ps *pathSet) add(p string, readOnly bool) {
	p, err := realPath(p)
	if err != nil || ps.seen[p] || within(p, ps.root) {
		return
	}
	ps.seen[p] = true
	ps.out = append(ps.out, sandbox.PathAccess{Path: p, ReadOnly: readOnly})
}

func (ps *pathSet) addConfigured(sc *config.SandboxConfig, home string) error {
	for _, list := range []struct {
		paths    []string
		readOnly bool
	}{{sc.Write, false}, {sc.Read, true}} {
		for _, p := range list.paths {
			abs, err := expandHome(p, home)
			if err != nil {
				return err
			}
			if _, err := os.Stat(abs); err != nil {
				return fmt.Errorf("sandbox path %s: %w", p, err)
			}
			ps.add(abs, list.readOnly)
		}
	}
	return nil
}

// cachePaths are the toolchain caches, granted writable: a cache holds no
// credentials, and builds need it (go, npm, pip, uv, cargo).
func cachePaths(env []string, home string) []string {
	cacheDir, _ := os.UserCacheDir()
	gopath := envOr(env, "GOPATH", filepath.Join(home, "go"))
	var out []string
	for _, p := range []string{
		envOr(env, "GOCACHE", filepath.Join(cacheDir, "go-build")),
		envOr(env, "GOMODCACHE", filepath.Join(gopath, "pkg", "mod")),
		filepath.Join(gopath, "pkg", "sumdb"), // checksum DB cache (toolchain switches verify here)
		envOr(env, "npm_config_cache", envOr(env, "NPM_CONFIG_CACHE", filepath.Join(home, ".npm"))),
		envOr(env, "PIP_CACHE_DIR", filepath.Join(cacheDir, "pip")),
		envOr(env, "UV_CACHE_DIR", filepath.Join(cacheDir, "uv")),
		filepath.Join(envOr(env, "CARGO_HOME", filepath.Join(home, ".cargo")), "registry"),
	} {
		if filepath.IsAbs(p) { // no home/cache dir: the default is relative, skip it
			out = append(out, p)
		}
	}
	return out
}

// toolchainReadPaths are the PATH entries, GOROOT and the GOENV dir, granted
// read-only. A PATH entry that is /, $HOME or an ancestor of $HOME would open
// the whole home: skipped.
func toolchainReadPaths(env []string, home string) []string {
	var out []string
	pathVar := envOr(env, "PATH", "")
	for _, d := range filepath.SplitList(pathVar) {
		if !filepath.IsAbs(d) || d == "/" || (home != "" && (d == home || within(home, d))) {
			continue
		}
		out = append(out, d)
	}
	if goroot := goRoot(env, pathVar); goroot != "" {
		out = append(out, goroot)
	}
	// go env settings (GOPROXY, GOPRIVATE...): Landlock rules are per
	// directory, so grant the dir holding the GOENV file.
	if cfgDir, err := os.UserConfigDir(); err == nil {
		out = append(out, filepath.Dir(envOr(env, "GOENV", filepath.Join(cfgDir, "go", "env"))))
	}
	return out
}

// goRoot finds GOROOT: from the environment, or as the parent of the bin dir
// holding the go binary found on PATH.
func goRoot(env []string, pathVar string) string {
	if r := envOr(env, "GOROOT", ""); r != "" {
		return r
	}
	for _, d := range filepath.SplitList(pathVar) {
		bin := filepath.Join(d, "go")
		if st, err := os.Stat(bin); err != nil || st.IsDir() {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(bin); err == nil {
			bin = resolved
		}
		if binDir := filepath.Dir(bin); filepath.Base(binDir) == "bin" {
			return filepath.Dir(binDir)
		}
		return ""
	}
	return ""
}

// envOr returns key from the child env (last value wins, as in exec.Cmd),
// then from this process, then def.
func envOr(env []string, key, def string) string {
	v := ""
	for _, e := range env {
		if k, val, ok := strings.Cut(e, "="); ok && k == key {
			v = val
		}
	}
	if v == "" {
		v = os.Getenv(key)
	}
	if v == "" {
		return def
	}
	return v
}

func expandHome(p, home string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home == "" {
			return "", fmt.Errorf("sandbox path %s: no home directory", p)
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("sandbox path %s must be absolute (or start with ~/)", p)
	}
	return filepath.Clean(p), nil
}

func realPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// within reports whether path is strictly below base.
func within(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
