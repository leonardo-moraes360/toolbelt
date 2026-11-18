package bus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvStateDir sobrepõe a base do diretório privado do bus (testes, harness).
const EnvStateDir = "TB_BUS_STATE_DIR"

// PrivateDir devolve o diretório privado do bus para o repositório root:
// sessões, arquivos de wake e estado do harness. Fica FORA do repositório
// (por padrão $XDG_STATE_HOME/toolbelt/bus/<hash da raiz>) para que um agente
// acordado com a raiz como workdir não leia tokens de outros agentes nem a
// própria config de wake.
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

// AgentDir devolve <privado>/sessions/<canal>/<nome>: só o token e o
// .mcp.json desse agente (o harness libera leitura só deste diretório).
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

// SaveSession grava o token da sessão CLI com modo 0600 (diretórios 0700).
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

// LoadMember lê o token da sessão CLI e autentica no store.
func (s *Store) LoadMember(channel, name string) (Member, error) {
	m, _, err := s.LoadSession(channel, name)
	return m, err
}

// LoadSession é LoadMember que também retorna o token (para prender uma
// conexão MCP à sessão CLI). O token nunca deve ser impresso.
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

// mkdirPrivate cria dir com 0700 e força 0700 no diretório final.
func mkdirPrivate(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}
