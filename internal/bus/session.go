package bus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvStateDir overrides the base of the bus private dir (tests, harness).
const EnvStateDir = "TB_BUS_STATE_DIR"

// PrivateDir holds sessions, wake files and harness state. It lives OUTSIDE
// the repo (default $XDG_STATE_HOME/toolbelt/bus/<root hash>) so an agent woken
// with the root as workdir cannot read other agents' tokens or its own wake config.
func PrivateDir(root string) (string, error) {
	base := os.Getenv(EnvStateDir)
	if base == "" {
		if x := os.Getenv("XDG_STATE_HOME"); x != "" && filepath.IsAbs(x) {
			base = filepath.Join(x, "toolbelt", "bus")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("diretório privado do bus: %w", err)
			}
			base = filepath.Join(home, ".local", "state", "toolbelt", "bus")
		}
	}
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("%s precisa ser absoluto", EnvStateDir)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return filepath.Join(base, hashSecret(abs)[:16]), nil
}

// AgentDir is <private>/sessions/<channel>/<name>: only that agent's token and
// .mcp.json (the harness grants read access to this dir alone).
func AgentDir(root, channel, name string) (string, error) {
	if err := ValidName("canal", channel); err != nil {
		return "", err
	}
	if err := ValidName("nome de agente", name); err != nil {
		return "", err
	}
	priv, err := PrivateDir(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(priv, "sessions", channel, name), nil
}

func sessionPath(root, channel, name string) (string, error) {
	dir, err := AgentDir(root, channel, name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "token"), nil
}

// SaveSession writes the CLI session token with mode 0600 (dirs 0700).
func SaveSession(root string, sess *Session) error {
	p, err := sessionPath(root, sess.Channel, sess.Name)
	if err != nil {
		return err
	}
	if err := mkdirPrivate(filepath.Dir(p)); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(sess.Token+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (s *Store) LoadMember(channel, name string) (Member, error) {
	m, _, err := s.LoadSession(channel, name)
	return m, err
}

// LoadSession also returns the token, to pin an MCP connection to the CLI
// session. The token must never be printed.
func (s *Store) LoadSession(channel, name string) (Member, string, error) {
	p, err := sessionPath(s.root, channel, name)
	if err != nil {
		return Member{}, "", err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return Member{}, "", fmt.Errorf("sem sessão de %q no canal %q: rode tb bus join antes", name, channel)
	}
	if err != nil {
		return Member{}, "", err
	}
	token := strings.TrimSpace(string(b))
	m, err := s.Auth(channel, name, token)
	return m, token, err
}

// mkdirPrivate forces 0700 on the leaf dir even if it already existed.
func mkdirPrivate(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}
