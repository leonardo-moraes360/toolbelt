package gitref

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/leonardo-moraes360/toolbelt/internal/git"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
)

// DefaultBase is the implicit base of verify/guard/review/stack.
const DefaultBase = "HEAD~1"

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

// ResolvePair resolves base and head. When base is the implicit default
// (HEAD~1) and the repository holds exactly one commit — a brand-new repo —
// base falls back to head (empty diff) with a warning instead of failing:
// build/lint/tests still run, and there is no prior tree to cheat against.
// Shallow clones and orphan branches next to other history stay fail-closed,
// since there an empty diff would hide edits to protected paths.
func ResolvePair(repo, base, head string) (baseSHA, headSHA string, err error) {
	headSHA, err = Resolve(repo, head)
	if err != nil {
		return "", "", fmt.Errorf("head: %w", err)
	}
	baseSHA, err = Resolve(repo, base)
	if err != nil {
		if base == DefaultBase && isSingleCommitRepo(repo, headSHA) {
			log.Warn("single-commit history: HEAD~1 missing, using head as base (empty diff)", "head", headSHA)
			return headSHA, headSHA, nil
		}
		return "", "", fmt.Errorf("base: %w", err)
	}
	return baseSHA, headSHA, nil
}

func isSingleCommitRepo(repo, headSHA string) bool {
	if !IsRootCommit(repo, headSHA) {
		return false
	}
	shallow, err := git.Output(repo, "rev-parse", "--is-shallow-repository")
	if err != nil || strings.TrimSpace(string(shallow)) != "false" {
		return false
	}
	count, err := git.Output(repo, "rev-list", "--all", "--count")
	if err != nil {
		return false
	}
	// --all misses a detached HEAD that no ref points to; count it as one.
	n := strings.TrimSpace(string(count))
	return n == "1" || n == "0"
}

// IsRootCommit reports whether sha has no parents.
func IsRootCommit(repo, sha string) bool {
	out, err := git.Output(repo, "rev-list", "--parents", "-n", "1", "--end-of-options", sha)
	if err != nil {
		return false
	}
	return len(strings.Fields(string(out))) == 1
}
