package verify

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/git"
	"github.com/leonardo-moraes360/toolbelt/internal/gitref"
)

const (
	// ConfigSourceNone: no .toolbelt/config.yaml at the base ref (or a
	// symlinked one, which is not followed), so only the
	// global config and the defaults apply. The normal case (base file used,
	// or -c) leaves config_source out of the JSON.
	ConfigSourceNone = "none"

	repoConfigPath = ".toolbelt/config.yaml"

	// SymlinkConfigWarning: the base ref commits the repository config as a
	// symbolic link, which verify does not follow.
	SymlinkConfigWarning = "symlinked " + repoConfigPath + " at the base ref is not followed; using the global config"
)

var checkNameRe = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// RepoConfig is what R0 resolved for one run.
type RepoConfig struct {
	Verify config.VerifyConfig
	Env    config.EnvConfig
	// Source is "" in the normal case and ConfigSourceNone without a base file.
	Source   string
	Warnings []string
}

// LoadVerifyConfig implements R0 (docs/specs/verify-rulers.md).
//
// With explicit (-c), user is the -c file over the defaults and is used
// as is: the repository config is not read at all.
//
// Without it, the verify: and env: sections are defaults < user (global
// config) < the repository's .toolbelt/config.yaml as committed at baseSHA.
// The working-tree file is never used; keys whose working-tree value differs
// from the base file are named in a warning. baseSHA and headSHA must be
// resolved SHAs.
func LoadVerifyConfig(repoDir, baseSHA, headSHA string, user *config.Config, explicit bool) (RepoConfig, error) {
	var rc RepoConfig
	if user == nil {
		user = config.DefaultConfig()
	}
	rc.Verify = user.Verify
	rc.Verify.Checks = append([]config.VerifyCheck(nil), user.Verify.Checks...)
	rc.Env = config.EnvConfig{
		Allow: append([]string(nil), user.Env.Allow...),
		Pass:  append([]string(nil), user.Env.Pass...),
	}
	if explicit {
		if err := ValidateChecks(rc.Verify.Checks); err != nil {
			return rc, err
		}
		return rc, ValidateCoveragePattern(rc.Verify.CoveragePattern)
	}
	if err := gitref.Validate(baseSHA); err != nil {
		return rc, err
	}
	data, found, symlink, err := readBaseFile(repoDir, baseSHA, repoConfigPath)
	if err != nil {
		return rc, err
	}
	var base repoSections
	switch {
	case symlink:
		// A link's target is not committed content verify can pin, so the
		// repository config is ignored rather than followed.
		rc.Source = ConfigSourceNone
		rc.Warnings = append(rc.Warnings, SymlinkConfigWarning)
	case found:
		if base, err = parseRepoSections(data); err != nil {
			return rc, fmt.Errorf("%s malformed at base %s: %w", repoConfigPath, shortSHA(baseSHA), err)
		}
		if err := base.overlay(&rc); err != nil {
			return rc, fmt.Errorf("%s malformed at base %s: %w", repoConfigPath, shortSHA(baseSHA), err)
		}
	default:
		rc.Source = ConfigSourceNone
	}
	if err := ValidateChecks(rc.Verify.Checks); err != nil {
		return rc, err
	}
	if err := ValidateCoveragePattern(rc.Verify.CoveragePattern); err != nil {
		return rc, err
	}
	if !symlink {
		if w := ignoredWorktreeKeys(repoToplevel(repoDir), &base); w != "" {
			rc.Warnings = append(rc.Warnings, w)
		}
	}
	if baseSHA == headSHA {
		rc.Warnings = append(rc.Warnings, "base resolves to head: nothing pins verify's config; CI should pass the merge base as --base")
	}
	return rc, nil
}

// repoSections holds the two sections R0 reads from the repository file.
type repoSections struct {
	Verify yaml.Node `yaml:"verify"`
	Env    yaml.Node `yaml:"env"`
}

func parseRepoSections(data []byte) (repoSections, error) {
	var s repoSections
	err := yaml.Unmarshal(data, &s)
	return s, err
}

func (s *repoSections) overlay(rc *RepoConfig) error {
	if s.Verify.Kind != 0 {
		if err := s.Verify.Decode(&rc.Verify); err != nil {
			return fmt.Errorf("verify: %w", err)
		}
	}
	if s.Env.Kind != 0 {
		if err := s.Env.Decode(&rc.Env); err != nil {
			return fmt.Errorf("env: %w", err)
		}
	}
	return nil
}

// ignoredWorktreeKeys compares the working tree's verify: and env: keys with
// the base file's and returns the warning naming the ones verify ignored,
// or "" when they match.
func ignoredWorktreeKeys(repoDir string, base *repoSections) string {
	data, err := os.ReadFile(filepath.Join(repoDir, repoConfigPath))
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		return fmt.Sprintf("working-tree %s unreadable, ignored (verify reads the base ref): %v", repoConfigPath, err)
	}
	wt, err := parseRepoSections(data)
	if err != nil {
		return fmt.Sprintf("working-tree %s malformed, ignored (verify reads the base ref)", repoConfigPath)
	}
	var keys []string
	keys = append(keys, diffSectionKeys("verify", &base.Verify, &wt.Verify)...)
	keys = append(keys, diffSectionKeys("env", &base.Env, &wt.Env)...)
	if len(keys) == 0 {
		return ""
	}
	return "ignored " + repoConfigPath + " keys that differ from the base ref: " + strings.Join(keys, ", ") +
		" (verify reads the base ref)"
}

// diffSectionKeys lists section.key for every key whose value differs.
func diffSectionKeys(section string, base, wt *yaml.Node) []string {
	b, w := sectionMap(base), sectionMap(wt)
	if b == nil && w == nil {
		if !reflect.DeepEqual(nodeValue(base), nodeValue(wt)) {
			return []string{section}
		}
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, m := range []map[string]any{b, w} {
		for k := range m {
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			if !reflect.DeepEqual(b[k], w[k]) {
				out = append(out, section+"."+k)
			}
		}
	}
	sort.Strings(out)
	return out
}

func sectionMap(n *yaml.Node) map[string]any {
	m, _ := nodeValue(n).(map[string]any)
	return m
}

func nodeValue(n *yaml.Node) any {
	if n == nil || n.Kind == 0 {
		return nil
	}
	var v any
	if err := n.Decode(&v); err != nil {
		return nil
	}
	return v
}

// repoToplevel is the working tree root for repoDir, which may be a
// subdirectory; repoDir itself when git cannot tell.
func repoToplevel(repoDir string) string {
	out, err := git.Output(repoDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return repoDir
	}
	if top := strings.TrimSpace(string(out)); top != "" {
		return top
	}
	return repoDir
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
