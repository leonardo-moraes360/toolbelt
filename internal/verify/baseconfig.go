package verify

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/git"
	"github.com/leonardo-moraes360/toolbelt/internal/gitref"
)

const (
	ConfigSourceBase     = "base"
	ConfigSourceWorktree = "worktree"

	repoConfigPath = ".toolbelt/config.yaml"
)

var checkNameRe = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// LoadVerifyConfig implements R0: the verify section comes from the base
// ref's .toolbelt/config.yaml, overlaid on the trusted user config (-c or
// global), so the change under test cannot rewrite its own gate. The
// working-tree file is used only when nothing pins it: no file at base, or
// base equal to HEAD with a dirty tree; that is reported as source
// "worktree" with a warning.
func LoadVerifyConfig(repoDir, baseRef string, user *config.Config) (vc config.VerifyConfig, source string, warnings []string, err error) {
	if user != nil {
		vc = user.Verify
		vc.Checks = append([]config.VerifyCheck(nil), user.Verify.Checks...)
	} else {
		vc = config.DefaultConfig().Verify
	}
	if err := gitref.Validate(baseRef); err != nil {
		return vc, "", nil, err
	}
	data, found, err := readBaseRepoConfig(repoDir, baseRef)
	if err != nil {
		return vc, "", nil, err
	}
	unpinned, reason, err := baseUnpinned(repoDir, baseRef, found)
	if err != nil {
		return vc, "", nil, err
	}
	source = ConfigSourceBase
	if unpinned {
		source = ConfigSourceWorktree
		warnings = append(warnings, reason)
		data, err = readWorktreeRepoConfig(repoDir)
		if err != nil {
			return vc, "", nil, err
		}
	}
	if err := overlayVerifySection(&vc, data, source, baseRef); err != nil {
		return vc, "", nil, err
	}
	if err := ValidateChecks(vc.Checks); err != nil {
		return vc, "", nil, err
	}
	return vc, source, warnings, nil
}

func readBaseRepoConfig(repoDir, baseRef string) (data []byte, found bool, err error) {
	out, err := git.Output(repoDir, "show", "--no-textconv", baseRef+":"+repoConfigPath)
	if err == nil {
		return out, true, nil
	}
	if isMissingPathAtRef(err) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("reading %s from base %s: %w", repoConfigPath, baseRef, err)
}

func readWorktreeRepoConfig(repoDir string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(repoDir, repoConfigPath))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// baseUnpinned reports whether the base file cannot pin verify's commands.
func baseUnpinned(repoDir, baseRef string, found bool) (unpinned bool, reason string, err error) {
	if !found {
		return true, "no " + repoConfigPath + " at base " + shortSHA(baseRef) + ": verify config read from the working tree, not pinned", nil
	}
	headSHA, err := gitRevParse(repoDir, "HEAD")
	if err != nil {
		return false, "", fmt.Errorf("resolving HEAD: %w", err)
	}
	if headSHA != baseRef {
		return false, "", nil
	}
	dirty, err := gitWorkTreeDirty(repoDir)
	if err != nil || !dirty {
		return false, "", err
	}
	return true, "base equals HEAD with a dirty tree: verify config read from the working tree, not pinned", nil
}

func overlayVerifySection(vc *config.VerifyConfig, data []byte, source, baseRef string) error {
	if len(data) == 0 {
		return nil
	}
	var raw struct {
		Verify yaml.Node `yaml:"verify"`
	}
	where := "working tree"
	if source == ConfigSourceBase {
		where = "base " + shortSHA(baseRef)
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("%s malformed at %s: %w", repoConfigPath, where, err)
	}
	if raw.Verify.Kind == 0 {
		return nil
	}
	if err := raw.Verify.Decode(vc); err != nil {
		return fmt.Errorf("%s malformed at %s (verify): %w", repoConfigPath, where, err)
	}
	return nil
}

// ValidateChecks enforces R3's naming rules: [a-z0-9-]{1,32}, unique, and a
// non-empty command.
func ValidateChecks(checks []config.VerifyCheck) error {
	seen := make(map[string]struct{}, len(checks))
	for _, c := range checks {
		if !checkNameRe.MatchString(c.Name) {
			return fmt.Errorf("verify.checks: invalid name %q: want [a-z0-9-]{1,32}", c.Name)
		}
		if _, dup := seen[c.Name]; dup {
			return fmt.Errorf("verify.checks: duplicate name %q", c.Name)
		}
		seen[c.Name] = struct{}{}
		if strings.TrimSpace(c.Cmd) == "" {
			return fmt.Errorf("verify.checks: %q has an empty cmd", c.Name)
		}
	}
	return nil
}

// isMissingPathAtRef tells a path absent at a valid ref apart from a bad ref,
// which must stay an error.
func isMissingPathAtRef(err error) bool {
	var ge *git.Error
	if !errors.As(err, &ge) {
		return false
	}
	msg := strings.ToLower(ge.Stderr)
	return strings.Contains(msg, "does not exist in") || strings.Contains(msg, "exists on disk, but not in")
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
