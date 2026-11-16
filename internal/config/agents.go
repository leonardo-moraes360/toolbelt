package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	AgentsMDName  = "AGENTS.md"
	ClaudeMDName  = "CLAUDE.md"
	AgentDocsFile = "agent_docs.yaml"
	maxHintLen    = 120
)

var (
	agentDocNames = []string{AgentsMDName, ClaudeMDName}

	toolAlternation = `go|npm|pnpm|yarn|bun|cargo|pytest|python3|python|make|task|ruff|golangci-lint|tox|cmake|ninja|mvn|gradle|dotnet|poetry|uv`

	hintCommandRe = regexp.MustCompile("(?i)`((?:" + toolAlternation + ")[^`]*)`")
	fencedCmdRe   = regexp.MustCompile("(?im)^(?: {0,3})(?:\\$ )?((?:" + toolAlternation + ")\\b[^\\n`]*)$")
	proseHintRe   = regexp.MustCompile("(?i)\\b(?:use|run|prefer|try|execute)\\s+((?:" + toolAlternation + ")\\b[^\\n]*)")

	shellFenceLangs = map[string]bool{
		"": true, "sh": true, "bash": true, "shell": true, "zsh": true,
		"console": true, "terminal": true, "shellsession": true, "text": true,
	}

	proseStopWords = map[string]bool{
		"when": true, "before": true, "after": true, "for": true, "to": true,
		"if": true, "while": true, "with": true, "and": true, "or": true,
		"as": true, "in": true, "on": true, "the": true, "a": true, "an": true,
		"from": true, "into": true, "via": true, "instead": true, "rather": true,
	}
)

type AgentDocsResult struct {
	Sources []string
	Hints   []string
	Seeded  bool
	Record  string
}

type agentDocsRecord struct {
	Sources []string `yaml:"sources"`
	Hints   []string `yaml:"hints"`
}

func DiscoverAgentDocs(repoRoot string) ([]string, error) {
	var found []string
	for _, name := range agentDocNames {
		path := filepath.Join(repoRoot, name)
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", name, err)
		}
		if info.IsDir() {
			continue
		}
		found = append(found, name)
	}
	return found, nil
}

func ExtractCommandHints(content string) []string {
	seen := map[string]struct{}{}
	var hints []string
	add := func(raw string) {
		cmd := normalizeHint(raw)
		if !isCommandHint(cmd) {
			return
		}
		if _, ok := seen[cmd]; ok {
			return
		}
		seen[cmd] = struct{}{}
		hints = append(hints, cmd)
	}

	stripped, fenceHints := extractShellFenceHints(content)
	for _, h := range fenceHints {
		add(h)
	}
	for _, m := range hintCommandRe.FindAllStringSubmatch(stripped, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range fencedCmdRe.FindAllStringSubmatch(stripped, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range proseHintRe.FindAllStringSubmatch(stripped, -1) {
		if len(m) > 1 {
			add(trimProseCommand(m[1]))
		}
	}
	sort.Strings(hints)
	return hints
}

func extractShellFenceHints(content string) (stripped string, hints []string) {
	var b strings.Builder
	i := 0
	for i < len(content) {
		if !strings.HasPrefix(content[i:], "```") {
			b.WriteByte(content[i])
			i++
			continue
		}
		fenceStart := i
		i += 3
		for i < len(content) && content[i] == '`' {
			i++
		}
		lineEnd := strings.IndexByte(content[i:], '\n')
		lang := ""
		bodyStart := i
		if lineEnd >= 0 {
			lang = strings.ToLower(strings.TrimSpace(content[i : i+lineEnd]))
			if idx := strings.IndexAny(lang, " \t{"); idx >= 0 {
				lang = lang[:idx]
			}
			bodyStart = i + lineEnd + 1
		}
		closeIdx := strings.Index(content[bodyStart:], "```")
		if closeIdx < 0 {
			b.WriteString(content[fenceStart:])
			break
		}
		body := content[bodyStart : bodyStart+closeIdx]
		closeEnd := bodyStart + closeIdx
		for closeEnd < len(content) && content[closeEnd] == '`' {
			closeEnd++
		}
		if shellFenceLangs[lang] {
			for _, line := range strings.Split(body, "\n") {
				line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "$ "))
				norm := normalizeHint(line)
				if isCommandHint(norm) {
					hints = append(hints, norm)
				}
			}
		}
		b.WriteByte('\n')
		i = closeEnd
	}
	return b.String(), hints
}

func looksLikeToolCommand(line string) bool {
	return hintCommandRe.MatchString("`"+line+"`") || fencedCmdRe.MatchString(line)
}

func normalizeHint(raw string) string {
	cmd := strings.TrimSpace(raw)
	cmd = strings.Trim(cmd, "$ ")
	cmd = strings.Join(strings.Fields(cmd), " ")
	return cmd
}

func trimProseCommand(raw string) string {
	fields := strings.Fields(raw)
	var out []string
	for i, f := range fields {
		clean := strings.Trim(f, ".,;:`\"'")
		if clean == "" {
			continue
		}
		low := strings.ToLower(clean)
		if i > 0 && proseStopWords[low] {
			break
		}
		out = append(out, clean)
	}
	return strings.Join(out, " ")
}

func isCommandHint(cmd string) bool {
	if cmd == "" || len(cmd) > maxHintLen {
		return false
	}
	if strings.ContainsAny(cmd, "\n\r{}") || strings.Contains(cmd, ":=") {
		return false
	}
	if strings.Contains(cmd, "func ") || strings.Contains(cmd, "=>") {
		return false
	}
	if !looksLikeToolCommand(cmd) {
		return false
	}
	return true
}

func MinimalAgentsMD(verify VerifyConfig) string {
	testCmd := strings.TrimSpace(verify.TestCmd)

	var b strings.Builder
	b.WriteString("# AGENTS\n\n")
	b.WriteString("## Toolbelt\n\n")
	b.WriteString("Este repositório pode usar [toolbelt](https://github.com/leonardo-moraes360/toolbelt) (`tb`) como harness para agentes.\n\n")
	b.WriteString("### Toolchain detectada\n\n")
	if testCmd != "" {
		b.WriteString("- testes (`verify.test_cmd`): `")
		b.WriteString(testCmd)
		b.WriteString("`\n")
	} else {
		b.WriteString("- testes (`verify.test_cmd`): _(não detectado — configure em `.toolbelt/config.yaml`)_\n")
	}
	if cmd := strings.TrimSpace(verify.BuildCmd); cmd != "" {
		b.WriteString("- build (`verify.build_cmd`): `")
		b.WriteString(cmd)
		b.WriteString("`\n")
	}
	if cmd := strings.TrimSpace(verify.LintCmd); cmd != "" {
		b.WriteString("- lint (`verify.lint_cmd`): `")
		b.WriteString(cmd)
		b.WriteString("`\n")
	}
	b.WriteString("\n### Uso do tb\n\n")
	b.WriteString("- `tb verify` — roda test/build/lint configurados em `.toolbelt/config.yaml`\n")
	b.WriteString("- `tb guard check` — checagens anti-trapaça no diff\n")
	b.WriteString("- `tb mem` — invariantes fora da compactação do contexto\n\n")
	b.WriteString("Ajuste `verify.test_cmd` (e, se houver, `build_cmd`/`lint_cmd`) se a detecção estiver incorreta.\n")
	return b.String()
}

func SeedAgentsMD(repoRoot string, verify VerifyConfig) (bool, error) {
	path := filepath.Join(repoRoot, AgentsMDName)
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat %s: %w", AgentsMDName, err)
	}
	if err := os.WriteFile(path, []byte(MinimalAgentsMD(verify)), 0o600); err != nil {
		return false, fmt.Errorf("writing %s: %w", AgentsMDName, err)
	}
	return true, nil
}

func verifyForAgentsSeed(repoRoot, toolbeltDir string) VerifyConfig {
	cfg := DefaultConfig()
	configPath := filepath.Join(toolbeltDir, DefaultConfigFile)
	if err := loadFromFile(configPath, cfg); err == nil {
		return cfg.Verify
	}
	cfg.Verify.TestCmd = DetectTestCmd(repoRoot)
	cfg.Verify.BuildCmd = DetectBuildCmd(repoRoot)
	cfg.Verify.LintCmd = DetectLintCmd(repoRoot)
	return cfg.Verify
}

func HandleAgentDocs(repoRoot, toolbeltDir string, writeAgents bool) (*AgentDocsResult, error) {
	result := &AgentDocsResult{}

	if writeAgents {
		seeded, err := SeedAgentsMD(repoRoot, verifyForAgentsSeed(repoRoot, toolbeltDir))
		if err != nil {
			return nil, err
		}
		result.Seeded = seeded
	}

	sources, err := DiscoverAgentDocs(repoRoot)
	if err != nil {
		return nil, err
	}
	result.Sources = sources

	hintSet := map[string]struct{}{}
	var hints []string
	for _, name := range sources {
		data, err := os.ReadFile(filepath.Join(repoRoot, name))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		for _, h := range ExtractCommandHints(string(data)) {
			if _, ok := hintSet[h]; ok {
				continue
			}
			hintSet[h] = struct{}{}
			hints = append(hints, h)
		}
	}
	sort.Strings(hints)
	result.Hints = hints

	if len(sources) > 0 {
		recordPath := filepath.Join(toolbeltDir, AgentDocsFile)
		rec := agentDocsRecord{Sources: sources, Hints: hints}
		data, err := yaml.Marshal(&rec)
		if err != nil {
			return nil, fmt.Errorf("marshaling agent docs record: %w", err)
		}
		if err := os.MkdirAll(toolbeltDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating config dir: %w", err)
		}
		if err := os.WriteFile(recordPath, data, 0o600); err != nil {
			return nil, fmt.Errorf("writing %s: %w", AgentDocsFile, err)
		}
		result.Record = recordPath
	}

	return result, nil
}
