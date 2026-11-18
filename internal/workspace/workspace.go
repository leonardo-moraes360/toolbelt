package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Scope struct {
	BaseCommit     string   `json:"base_commit"`
	AllowedPaths   []string `json:"allowed_paths"`
	ForbiddenPaths []string `json:"forbidden_paths"`
}

type Workspace struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	BaseRef   string    `json:"base_ref"`
	Branch    string    `json:"branch"`
	CreatedAt time.Time `json:"created_at"`
	Scope     Scope     `json:"scope"`
}

type Manager struct {
	baseDir string
	repoDir string
	// gitArgs: "-c" options prefixed to every git call in Create/Remove (see
	// SetGitArgs); run, when non-nil, executes those commands (see SetRunner).
	gitArgs []string
	run     func(*exec.Cmd) ([]byte, error)
}

// SetGitArgs sets global git args (e.g. "-c", "core.hooksPath=/dev/null")
// inserted before the subcommand in Create/Remove. Optional and additive:
// without it, the user's full git config applies, hooks included. The router
// uses it so worktrees it creates never run the user's hooks/fsmonitor.
func (m *Manager) SetGitArgs(args ...string) { m.gitArgs = append([]string(nil), args...) }

// SetRunner replaces execution of the Create/Remove git commands (returning
// combined output) so the caller can track the child processes: the router
// registers them so they are never mistaken for escaped descendants.
func (m *Manager) SetRunner(fn func(*exec.Cmd) ([]byte, error)) { m.run = fn }

func (m *Manager) combined(cmd *exec.Cmd) ([]byte, error) {
	if m.run != nil {
		return m.run(cmd)
	}
	return cmd.CombinedOutput()
}

func (m *Manager) gitCmd(dir string, args ...string) *exec.Cmd {
	full := append(append([]string(nil), m.gitArgs...), args...)
	cmd := exec.Command("git", full...) //nolint:gosec // args controlled by the workspace manager
	cmd.Dir = dir
	return cmd
}

func (m *Manager) head(dir string) (string, error) {
	out, err := m.combined(m.gitCmd(dir, "rev-parse", "HEAD"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func NewManager(repoDir string) (*Manager, error) {
	abs, err := filepath.Abs(repoDir)
	if err != nil {
		return nil, err
	}
	base := filepath.Join(abs, ".toolbelt", "workspaces")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, err
	}
	return &Manager{baseDir: base, repoDir: abs}, nil
}

func (m *Manager) Create(id, baseRef string, scope Scope) (*Workspace, error) {
	if id == "" {
		id = fmt.Sprintf("ws-%d", time.Now().UnixNano())
	}
	path := filepath.Join(m.baseDir, id)
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("workspace %s já existe", id)
	}
	branch := "tb-workspace/" + id
	cmd := m.gitCmd(m.repoDir, "worktree", "add", "-b", branch, path, baseRef)
	if out, err := m.combined(cmd); err != nil {
		return nil, fmt.Errorf("%s: %w", string(out), err)
	}
	hash, _ := m.head(path)
	if scope.BaseCommit == "" {
		scope.BaseCommit = hash
	}
	return &Workspace{ID: id, Path: path, BaseRef: baseRef, Branch: branch, CreatedAt: time.Now(), Scope: scope}, nil
}

func (m *Manager) List() ([]*Workspace, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var res []*Workspace
	var current string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			current = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") && strings.HasPrefix(current, m.baseDir) {
			res = append(res, &Workspace{
				ID:     filepath.Base(current),
				Path:   current,
				Branch: strings.TrimPrefix(line, "branch refs/heads/"),
			})
			current = ""
		}
	}
	return res, nil
}

func (m *Manager) Remove(id string) error {
	path := filepath.Join(m.baseDir, id)
	cmd := m.gitCmd(m.repoDir, "worktree", "remove", "--force", path)
	if out, err := m.combined(cmd); err != nil {
		return fmt.Errorf("%s: %w", string(out), err)
	}
	_, _ = m.combined(m.gitCmd(m.repoDir, "branch", "-D", "tb-workspace/"+id))
	return nil
}

func (m *Manager) Snapshot(ws *Workspace) (string, error) {
	cmd := exec.Command("git", "stash", "push", "-u", "-m", "tb-snapshot")
	cmd.Dir = ws.Path
	_, _ = cmd.CombinedOutput()
	return gitHEAD(ws.Path)
}

func (m *Manager) Rollback(ws *Workspace, ref string) error {
	if out, err := exec.Command("git", "-C", ws.Path, "reset", "--hard", ref).CombinedOutput(); err != nil { //nolint:gosec // internal snapshot ref
		return fmt.Errorf("%s: %w", string(out), err)
	}
	if out, err := exec.Command("git", "-C", ws.Path, "clean", "-fdx").CombinedOutput(); err != nil { //nolint:gosec // controlled worktree cleanup
		return fmt.Errorf("%s: %w", string(out), err)
	}
	return nil
}

func (m *Manager) Allowed(ws *Workspace, path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(abs, ws.Path) {
		return fmt.Errorf("fora do workspace")
	}
	rel, err := filepath.Rel(ws.Path, abs)
	if err != nil {
		return err
	}
	for _, p := range ws.Scope.ForbiddenPaths {
		if matchScope(rel, p) {
			return fmt.Errorf("proibido: %s", p)
		}
	}
	if len(ws.Scope.AllowedPaths) == 0 {
		return nil
	}
	for _, p := range ws.Scope.AllowedPaths {
		if matchScope(rel, p) {
			return nil
		}
	}
	return fmt.Errorf("fora do scope permitido")
}

func matchScope(rel, pattern string) bool {
	if ok, _ := filepath.Match(pattern, rel); ok {
		return true
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(rel, prefix)
	}
	return false
}

func gitHEAD(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
