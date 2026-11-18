package verify

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type headWorkDir struct {
	HeadSHA   string
	WorkDir   string
	Temporary bool
	DirtyWarn bool

	repoDir     string
	parentDir   string
	cleanupOnce sync.Once
}

func attachHeadWorkDir(repoDir, headSHA string) (*headWorkDir, error) {
	currentSHA, err := gitRevParse(repoDir, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("resolver HEAD: %w", err)
	}
	hw := &headWorkDir{
		HeadSHA: headSHA,
		WorkDir: repoDir,
		repoDir: repoDir,
	}
	if headSHA == currentSHA {
		dirty, dirtyErr := gitWorkTreeDirty(repoDir)
		if dirtyErr != nil {
			return nil, dirtyErr
		}
		hw.DirtyWarn = dirty
		return hw, nil
	}

	parent, err := os.MkdirTemp("", "tb-verify-head-*")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(parent, "tree")
	cmd := exec.Command("git", "worktree", "add", "--detach", path, headSHA) //nolint:gosec // SHA resolved via rev-parse
	cmd.Dir = repoDir
	if out, runErr := cmd.CombinedOutput(); runErr != nil {
		_ = os.RemoveAll(parent)
		return nil, fmt.Errorf("git worktree add --detach: %s: %w", strings.TrimSpace(string(out)), runErr)
	}
	hw.WorkDir = path
	hw.Temporary = true
	hw.parentDir = parent
	return hw, nil
}

func (hw *headWorkDir) Cleanup() {
	if hw == nil || !hw.Temporary {
		return
	}
	hw.cleanupOnce.Do(func() {
		cmd := exec.Command("git", "worktree", "remove", "--force", hw.WorkDir) //nolint:gosec // path created by an internal MkdirTemp
		cmd.Dir = hw.repoDir
		_ = cmd.Run()
		if hw.parentDir != "" {
			_ = os.RemoveAll(hw.parentDir)
		}
	})
}

func gitRevParse(repo, ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", ref) //nolint:gosec // operator-supplied refs
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitWorkTreeDirty(repo string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}
