package gitref

import (
	"fmt"
	"os/exec"
	"strings"
	"unicode"
)

func Validate(ref string) error {
	if ref == "" {
		return fmt.Errorf("ref vazia")
	}
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("ref inválida %q: não pode começar com '-'", ref)
	}
	for _, r := range ref {
		if r == 0 || unicode.IsControl(r) || unicode.IsSpace(r) {
			return fmt.Errorf("ref inválida %q: caracteres proibidos", ref)
		}
	}
	return nil
}

func Resolve(repo, ref string) (string, error) {
	if err := Validate(ref); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "rev-parse", "--verify", "--end-of-options", ref+"^{commit}") //nolint:gosec // ref validada
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("ref inexistente ou inválida %q: %s", ref, msg)
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "", fmt.Errorf("ref inexistente ou inválida %q", ref)
	}
	return sha, nil
}

func ResolvePair(repo, base, head string) (baseSHA, headSHA string, err error) {
	baseSHA, err = Resolve(repo, base)
	if err != nil {
		return "", "", fmt.Errorf("base: %w", err)
	}
	headSHA, err = Resolve(repo, head)
	if err != nil {
		return "", "", fmt.Errorf("head: %w", err)
	}
	return baseSHA, headSHA, nil
}
