package gitref

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/leonardo-moraes360/toolbelt/internal/git"
)

func Validate(ref string) error {
	if ref == "" {
		return fmt.Errorf("empty ref")
	}
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("invalid ref %q: must not start with '-'", ref)
	}
	for _, r := range ref {
		if r == 0 || unicode.IsControl(r) || unicode.IsSpace(r) {
			return fmt.Errorf("invalid ref %q: forbidden characters", ref)
		}
	}
	return nil
}

func Resolve(repo, ref string) (string, error) {
	if err := Validate(ref); err != nil {
		return "", err
	}
	out, err := git.Output(repo, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("ref missing or invalid %q: %w", ref, err)
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "", fmt.Errorf("ref missing or invalid %q", ref)
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
