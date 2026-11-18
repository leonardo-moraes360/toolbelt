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

// VersionedFileMode é o modo dos arquivos que o tb init grava para serem
// versionados (config.yaml, agent_docs.yaml, AGENTS.md): sem segredos, então
// seguem o modo padrão de arquivo do repositório em vez de 0600.
const VersionedFileMode = 0o644

var LocalCacheExcludePatterns = []string{
	".toolbelt/graph.json",
	".toolbelt/logs/",
	".toolbelt/vault/",
	".toolbelt/workspaces/",
	".toolbelt/captures/",
	".toolbelt/bench/",
	".toolbelt/bus/",
}

func EnsureLocalGitExclude(repoRoot string) (string, error) {
	excludePath, ok := gitInfoExcludePath(repoRoot)
	if !ok {
		return "", nil
	}
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return "", fmt.Errorf("criando .git/info: %w", err)
	}

	existing, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	section := buildExcludeSection()
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

// gitCommonDir resolve o diretório comum de uma worktree (ou submódulo com
// commondir): o git só lê $GIT_COMMON_DIR/info/exclude, nunca o info/ do
// gitdir da worktree.
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

func buildExcludeSection() string {
	var b strings.Builder
	b.WriteString(gitExcludeBegin)
	b.WriteByte('\n')
	b.WriteString("# cache/derivados — config e invariantes ficam versionáveis\n")
	for _, p := range LocalCacheExcludePatterns {
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
		return strings.TrimRight(existing[:start], "\n") + "\n\n" + section
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
