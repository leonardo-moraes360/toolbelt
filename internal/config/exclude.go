package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	gitExcludeBegin = "# >>> toolbelt local/cache (tb init)"
	gitExcludeEnd   = "# <<< toolbelt local/cache"
)

// Files tb init writes to be versioned (config.yaml, agent_docs.yaml,
// AGENTS.md) hold no secrets, so they follow the repository's default file
// mode instead of 0600.
const VersionedFileMode = 0o644

// LocalCacheExcludePatterns are local state, never versioned. The vault is
// versioned (docs/specs/vault.md, G5); only its .obsidian/ is excluded, as
// an extra pattern derived from vault.path.
var LocalCacheExcludePatterns = []string{
	".toolbelt/graph.json",
	".toolbelt/logs/",
	".toolbelt/workspaces/",
	".toolbelt/captures/",
	".toolbelt/bench/",
	".toolbelt/bus/",
}

// legacyExcludePatterns are lines older tb versions wrote inside the
// section; they are dropped when an orphan begin marker is repaired.
var legacyExcludePatterns = []string{".toolbelt/vault/"}

// VaultExcludePattern is the extra exclude for a vault path: its Obsidian
// workspace config, which can hold personal data.
func VaultExcludePattern(vaultPath string) string {
	return strings.TrimSuffix(filepath.ToSlash(filepath.Clean(vaultPath)), "/") + "/.obsidian/"
}

// EnsureLocalGitExclude writes tb's section in .git/info/exclude with
// LocalCacheExcludePatterns plus extra (for example VaultExcludePattern).
func EnsureLocalGitExclude(repoRoot string, extra ...string) (string, error) {
	excludePath, ok := gitInfoExcludePath(repoRoot)
	if !ok {
		return "", nil
	}
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return "", fmt.Errorf("creating .git/info: %w", err)
	}

	existing, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	section := buildExcludeSection(extra)
	updated := upsertExcludeSection(string(existing), section)
	if updated == string(existing) {
		return excludePath, nil
	}
	if err := os.WriteFile(excludePath, []byte(updated), 0o600); err != nil {
		return "", err
	}
	return excludePath, nil
}

func gitInfoExcludePath(repoRoot string) (string, bool) {
	gitPath := filepath.Join(repoRoot, ".git")
	st, err := os.Stat(gitPath)
	if err != nil {
		return "", false
	}
	if st.IsDir() {
		return filepath.Join(gitPath, "info", "exclude"), true
	}
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", false
	}
	line := strings.TrimSpace(string(data))
	const prefix = "gitdir:"
	if !strings.HasPrefix(strings.ToLower(line), prefix) {
		return "", false
	}
	gitDir := strings.TrimSpace(line[len(prefix):])
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoRoot, gitDir)
	}
	return filepath.Join(gitCommonDir(gitDir), "info", "exclude"), true
}

// For a worktree (or a submodule with commondir) git only reads
// $GIT_COMMON_DIR/info/exclude, never the info/ of the worktree's gitdir.
func gitCommonDir(gitDir string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if err != nil {
		return gitDir
	}
	common := strings.TrimSpace(string(data))
	if common == "" {
		return gitDir
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(gitDir, common)
	}
	return filepath.Clean(common)
}

const excludeComment = "# cache/derived — config and invariants stay versionable"

func buildExcludeSection(extra []string) string {
	var b strings.Builder
	b.WriteString(gitExcludeBegin)
	b.WriteByte('\n')
	b.WriteString(excludeComment + "\n")
	for _, p := range append(append([]string{}, LocalCacheExcludePatterns...), extra...) {
		b.WriteString(p)
		b.WriteByte('\n')
	}
	b.WriteString(gitExcludeEnd)
	b.WriteByte('\n')
	return b.String()
}

func upsertExcludeSection(existing, section string) string {
	start := strings.Index(existing, gitExcludeBegin)
	if start < 0 {
		if existing == "" {
			return section
		}
		out := strings.TrimRight(existing, "\n") + "\n\n" + section
		return out
	}
	end := strings.Index(existing[start:], gitExcludeEnd)
	if end < 0 {
		return repairOrphanBegin(existing, start, section)
	}
	end = start + end + len(gitExcludeEnd)
	for end < len(existing) && (existing[end] == '\n' || existing[end] == '\r') {
		end++
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(existing[:start], "\n"))
	if start > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString(section)
	b.WriteString(existing[end:])
	return b.String()
}

// repairOrphanBegin handles a begin marker without its end marker: the
// orphan marker and, after it, the lines tb writes or wrote are dropped;
// every other line (the user's) is kept, and a fresh section is appended.
func repairOrphanBegin(existing string, start int, section string) string {
	known := map[string]bool{gitExcludeBegin: true, excludeComment: true}
	for _, p := range append(append([]string{}, LocalCacheExcludePatterns...), legacyExcludePatterns...) {
		known[p] = true
	}
	var kept []string
	for _, l := range strings.Split(existing[start:], "\n") {
		t := strings.TrimRight(l, "\r")
		if known[t] || strings.HasSuffix(t, "/.obsidian/") {
			continue
		}
		kept = append(kept, l)
	}
	head := strings.TrimRight(existing[:start], "\n")
	tail := strings.TrimRight(strings.Join(kept, "\n"), "\n")
	var b strings.Builder
	for _, part := range []string{head, tail} {
		if strings.TrimSpace(part) != "" {
			b.WriteString(part)
			b.WriteString("\n\n")
		}
	}
	b.WriteString(section)
	return b.String()
}
