// Package vault is the Obsidian-compatible Markdown vault (docs/specs/vault.md,
// Phase A): the Markdown files are the only store (ADR-016), so every call
// scans the vault from disk.
package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/mem"
)

// Exit codes of the tb vault commands.
const (
	ExitOK    = 0
	ExitFail  = 1
	ExitError = 2
)

// Error is a refused call with an exit code and a machine reason.
type Error struct {
	Code   int
	Reason string
	Msg    string
}

func (e *Error) Error() string { return e.Reason + ": " + e.Msg }

func errorf(code int, reason, format string, a ...any) *Error {
	return &Error{Code: code, Reason: reason, Msg: fmt.Sprintf(format, a...)}
}

// Vault is a resolved vault: the repository root, the vault directory and
// its config.
type Vault struct {
	Root string // repository root, symlinks resolved
	Dir  string // the vault directory, symlinks resolved, inside Root
	Rel  string // Dir relative to Root, with "/"
	Cfg  config.VaultConfig
}

// Open resolves the vault from cwd (R0, R1): the root is the parent of the
// .toolbelt/ directory found from cwd, the config is read from that root's
// .toolbelt/config.yaml (never from the process cwd), and vault.path must
// resolve inside the root.
func Open(cwd string) (*Vault, error) {
	if !filepath.IsAbs(cwd) {
		return nil, errorf(ExitError, "usage", "cwd must be an absolute path")
	}
	tbDir := mem.LocateRepoToolbeltDir(cwd)
	if fi, err := os.Stat(tbDir); err != nil || !fi.IsDir() {
		return nil, errorf(ExitError, "no_toolbelt_dir", "no .toolbelt/ directory above %s: run tb init", cwd)
	}
	root, err := filepath.EvalSymlinks(filepath.Dir(tbDir))
	if err != nil {
		return nil, errorf(ExitError, "no_toolbelt_dir", "%v", err)
	}
	cfg, err := loadConfig(root)
	if err != nil {
		return nil, err
	}
	return openAt(root, cfg)
}

func loadConfig(root string) (config.VaultConfig, error) {
	path := filepath.Join(root, config.DefaultConfigDir, config.DefaultConfigFile)
	c, err := config.LoadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		c = config.DefaultConfig()
	case err != nil:
		return config.VaultConfig{}, errorf(ExitError, "config", "%v", err)
	}
	if err := c.Vault.Validate(); err != nil {
		return config.VaultConfig{}, errorf(ExitError, "config", "%v", err)
	}
	return c.Vault, nil
}

func openAt(root string, cfg config.VaultConfig) (*Vault, error) {
	joined := filepath.Join(root, filepath.Clean(filepath.FromSlash(cfg.Path)))
	dir, err := filepath.EvalSymlinks(joined)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errorf(ExitFail, "vault_not_found", "vault %s does not exist: run tb init or fix vault.path", cfg.Path)
	}
	if err != nil {
		return nil, errorf(ExitError, "vault_path_outside_repo", "%v", err)
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return nil, errorf(ExitError, "vault_path_outside_repo", "vault.path %s resolves outside the repository", cfg.Path)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil, errorf(ExitFail, "vault_not_found", "vault %s is not a directory", cfg.Path)
	}
	return &Vault{Root: root, Dir: dir, Rel: filepath.ToSlash(rel), Cfg: cfg}, nil
}
