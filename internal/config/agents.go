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
)

var (
	agentDocNames = []string{AgentsMDName, ClaudeMDName}

	hintCommandRe = regexp.MustCompile("(?i)`((?:go|npm|pnpm|yarn|bun|cargo|pytest|python|make|task|ruff|golangci-lint|tox|cmake|ninja|mvn|gradle|dotnet|poetry|uv)[^`]*)`")
	fencedCmdRe   = regexp.MustCompile("(?im)^(?: {0,3})(?:\\$ )?((?:go|npm|pnpm|yarn|bun|cargo|pytest|python|make|task|ruff|golangci-lint|tox|cmake|ninja|mvn|gradle|dotnet|poetry|uv)\\b[^\\n`]*)$")
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
		cmd := strings.TrimSpace(raw)
		cmd = strings.Trim(cmd, "$ ")
		if cmd == "" {
			return
		}
		if _, ok := seen[cmd]; ok {
			return
		}
		seen[cmd] = struct{}{}
		hints = append(hints, cmd)
	}
	for _, m := range hintCommandRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range fencedCmdRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	sort.Strings(hints)
	return hints
}

func MinimalAgentsMD(verify VerifyConfig) string {
	testCmd := strings.TrimSpace(verify.TestCmd)
	if testCmd == "" {
		testCmd = DefaultConfig().Verify.TestCmd
	}

	var b strings.Builder
	b.WriteString("# AGENTS\n\n")
	b.WriteString("## Toolbelt\n\n")
	b.WriteString("Este repositório pode usar [toolbelt](https://github.com/leonardo-moraes360/toolbelt) (`tb`) como harness para agentes.\n\n")
	b.WriteString("### Toolchain detectada\n\n")
	b.WriteString("- testes (`verify.test_cmd`): `")
	b.WriteString(testCmd)
	b.WriteString("`\n")
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
	if cmd := DetectTestCmd(repoRoot); cmd != "" {
		cfg.Verify.TestCmd = cmd
	}
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
