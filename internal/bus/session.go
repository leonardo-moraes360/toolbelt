package bus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sessionPath devolve .toolbelt/bus/sessions/<canal>/<nome>.token.
func sessionPath(root, channel, name string) (string, error) {
	if err := ValidName("canal", channel); err != nil {
		return "", err
	}
	if err := ValidName("nome de agente", name); err != nil {
		return "", err
	}
	return filepath.Join(Dir(root), "sessions", channel, name+".token"), nil
}

// SaveSession grava o token da sessão CLI com modo 0600 (diretórios 0700).
func SaveSession(root string, sess *Session) error {
	p, err := sessionPath(root, sess.Channel, sess.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
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
	p, err := sessionPath(s.root, channel, name)
	if err != nil {
		return Member{}, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return Member{}, fmt.Errorf("sem sessão de %q no canal %q: rode tb bus join antes", name, channel)
	}
	if err != nil {
		return Member{}, err
	}
	return s.Auth(channel, name, strings.TrimSpace(string(b)))
}
