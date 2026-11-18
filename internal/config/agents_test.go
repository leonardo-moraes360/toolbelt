package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverAgentDocs_FindsAgentsAndClaude(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, AgentsMDName), []byte("# hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ClaudeMDName), []byte("see AGENTS.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := DiscoverAgentDocs(root)
	if err != nil {
		t.Fatalf("DiscoverAgentDocs: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("found=%v, want AGENTS.md and CLAUDE.md", found)
	}
	if found[0] != AgentsMDName || found[1] != ClaudeMDName {
		t.Fatalf("order/names = %v", found)
	}
}

func TestDiscoverAgentDocs_Absent(t *testing.T) {
	found, err := DiscoverAgentDocs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf("expected empty, got %v", found)
	}
}

func TestExtractCommandHints(t *testing.T) {
	content := "# Build\n\nRun `go test ./...` and `task lint:log`.\n\n```\nnpm test\ncargo test -p ruff_linter\n```\n"
	hints := ExtractCommandHints(content)
	joined := strings.Join(hints, "\n")
	for _, want := range []string{"go test ./...", "task lint:log", "npm test", "cargo test -p ruff_linter"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("hints missing %q: %v", want, hints)
		}
	}
}

func TestExtractCommandHints_DropsLongGoFences(t *testing.T) {
	content := `# AGENTS

Run ` + "`go test ./...`" + ` and ` + "`task lint:log`" + `.

` + "```go\n" + `func CharmtonePantera() color.Color {
	return lipgloss.Color("#FF278C")
}
` + "```\n\n" + "```go\n" + `func TestSomething(t *testing.T) {
	mock := NewMock()
	go func() {
		mock.Run()
	}()
}
` + "```\n\n" + "```\n" + `go test ./internal/stringext/
task lint:log
` + "```\n"
	hints := ExtractCommandHints(content)
	joined := strings.Join(hints, "\n")
	for _, noise := range []string{"CharmtonePantera", "TestSomething", "lipgloss", "NewMock", "go func"} {
		if strings.Contains(joined, noise) {
			t.Fatalf("noisy hint %q leaked: %v", noise, hints)
		}
	}
	for _, want := range []string{"go test ./...", "task lint:log", "go test ./internal/stringext/"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("useful hint missing %q: %v", want, hints)
		}
	}
}

func TestExtractCommandHints_ProseWithoutBackticks(t *testing.T) {
	content := "# Notes\nUse make test when changing args.\nPrefer pytest for the python suite.\nRun npm test before pushing.\n"
	hints := ExtractCommandHints(content)
	joined := strings.Join(hints, "\n")
	for _, want := range []string{"make test", "pytest", "npm test"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("prose hint missing %q: %v", want, hints)
		}
	}
}

func TestSeedAgentsMD_CreatesWhenAbsent(t *testing.T) {
	root := t.TempDir()
	seeded, err := SeedAgentsMD(root, VerifyConfig{TestCmd: "go test ./..."})
	if err != nil {
		t.Fatal(err)
	}
	if !seeded {
		t.Fatal("expected seeded=true")
	}
	data, err := os.ReadFile(filepath.Join(root, AgentsMDName))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "tb verify") {
		t.Fatalf("seed content missing tb verify: %s", data)
	}
	if !strings.Contains(text, "go test ./...") {
		t.Fatalf("seed must mention detected test_cmd: %s", data)
	}
}

func TestSeedAgentsMD_MentionsToolchainCmds(t *testing.T) {
	root := t.TempDir()
	verify := VerifyConfig{
		TestCmd:  "npm test",
		BuildCmd: "npm run build",
		LintCmd:  "npm run lint",
	}
	seeded, err := SeedAgentsMD(root, verify)
	if err != nil {
		t.Fatal(err)
	}
	if !seeded {
		t.Fatal("expected seeded=true")
	}
	data, err := os.ReadFile(filepath.Join(root, AgentsMDName))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"npm test", "npm run build", "npm run lint"} {
		if !strings.Contains(text, want) {
			t.Fatalf("seed missing %q: %s", want, text)
		}
	}
}

func TestSeedAgentsMD_OmitsEmptyBuildLint(t *testing.T) {
	root := t.TempDir()
	seeded, err := SeedAgentsMD(root, VerifyConfig{TestCmd: "make check"})
	if err != nil {
		t.Fatal(err)
	}
	if !seeded {
		t.Fatal("expected seeded=true")
	}
	data, err := os.ReadFile(filepath.Join(root, AgentsMDName))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "make check") {
		t.Fatalf("seed missing test_cmd: %s", text)
	}
	if strings.Contains(text, "verify.build_cmd") {
		t.Fatalf("seed must omit empty build_cmd: %s", text)
	}
	if strings.Contains(text, "verify.lint_cmd") {
		t.Fatalf("seed must omit empty lint_cmd: %s", text)
	}
}

func TestSeedAgentsMD_EmptyTestCmdDoesNotFallbackToGo(t *testing.T) {
	root := t.TempDir()
	seeded, err := SeedAgentsMD(root, VerifyConfig{
		BuildCmd: "npm run build",
		LintCmd:  "npm run lint",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !seeded {
		t.Fatal("expected seeded=true")
	}
	data, err := os.ReadFile(filepath.Join(root, AgentsMDName))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "go test") {
		t.Fatalf("seed must not invent go test without go.mod: %s", text)
	}
	if !strings.Contains(text, "npm run build") {
		t.Fatalf("seed missing build_cmd: %s", text)
	}
	if !strings.Contains(text, "verify.test_cmd") {
		t.Fatalf("seed must still mention test_cmd field: %s", text)
	}
}

func TestSeedAgentsMD_NeverOverwrites(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, AgentsMDName)
	original := "# custom agents rules\nnever overwrite me\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	seeded, err := SeedAgentsMD(root, VerifyConfig{TestCmd: "cargo test"})
	if err != nil {
		t.Fatal(err)
	}
	if seeded {
		t.Fatal("must not overwrite existing AGENTS.md")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("AGENTS.md was modified: %q", data)
	}
}

func TestHandleAgentDocs_WriteAgentsUsesDetectedToolchain(t *testing.T) {
	root := t.TempDir()
	toolbeltDir := filepath.Join(root, DefaultConfigDir)
	if err := Init(toolbeltDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"test":"node test.js"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.Verify.TestCmd = "npm test"
	cfg.Verify.LintCmd = "npm run lint"
	if err := Save(cfg, filepath.Join(toolbeltDir, DefaultConfigFile)); err != nil {
		t.Fatal(err)
	}

	result, err := HandleAgentDocs(root, toolbeltDir, true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Seeded {
		t.Fatal("expected seed with --write-agents")
	}
	data, err := os.ReadFile(filepath.Join(root, AgentsMDName))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "npm test") {
		t.Fatalf("seed must cite config test_cmd: %s", text)
	}
	if !strings.Contains(text, "npm run lint") {
		t.Fatalf("seed must cite config lint_cmd: %s", text)
	}
}

func TestHandleAgentDocs_RegistersExisting(t *testing.T) {
	root := t.TempDir()
	toolbeltDir := filepath.Join(root, DefaultConfigDir)
	content := "Use `go test ./cmd/` and `cmake -B build`.\n"
	if err := os.WriteFile(filepath.Join(root, AgentsMDName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := HandleAgentDocs(root, toolbeltDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Seeded {
		t.Fatal("should not seed without flag")
	}
	if len(result.Sources) != 1 || result.Sources[0] != AgentsMDName {
		t.Fatalf("sources=%v", result.Sources)
	}
	if len(result.Hints) == 0 {
		t.Fatal("expected command hints from AGENTS.md")
	}
	recordPath := filepath.Join(toolbeltDir, AgentDocsFile)
	if result.Record != recordPath {
		t.Fatalf("record=%q want %q", result.Record, recordPath)
	}
	if _, err := os.Stat(recordPath); err != nil {
		t.Fatalf("agent_docs.yaml not written: %v", err)
	}
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), AgentsMDName) {
		t.Fatalf("record missing source: %s", data)
	}
	if !strings.Contains(string(data), "go test") {
		t.Fatalf("record missing hints: %s", data)
	}
}

func TestHandleAgentDocs_WriteAgentsOptIn(t *testing.T) {
	root := t.TempDir()
	toolbeltDir := filepath.Join(root, DefaultConfigDir)

	without, err := HandleAgentDocs(root, toolbeltDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if without.Seeded || len(without.Sources) != 0 {
		t.Fatalf("without flag should not create AGENTS.md: %+v", without)
	}
	if _, err := os.Stat(filepath.Join(root, AgentsMDName)); !os.IsNotExist(err) {
		t.Fatal("AGENTS.md must stay absent without --write-agents")
	}

	with, err := HandleAgentDocs(root, toolbeltDir, true)
	if err != nil {
		t.Fatal(err)
	}
	if !with.Seeded {
		t.Fatal("expected seed with --write-agents")
	}
	if len(with.Sources) != 1 || with.Sources[0] != AgentsMDName {
		t.Fatalf("sources after seed=%v", with.Sources)
	}
	if _, err := os.Stat(filepath.Join(root, AgentsMDName)); err != nil {
		t.Fatal(err)
	}
}

func TestHandleAgentDocs_WriteAgentsDoesNotClobber(t *testing.T) {
	root := t.TempDir()
	toolbeltDir := filepath.Join(root, DefaultConfigDir)
	original := "# keep\n`pytest -q`\n"
	if err := os.WriteFile(filepath.Join(root, AgentsMDName), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ClaudeMDName), []byte("pointer\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := HandleAgentDocs(root, toolbeltDir, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Seeded {
		t.Fatal("must not report seeded when AGENTS.md already exists")
	}
	data, err := os.ReadFile(filepath.Join(root, AgentsMDName))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("overwrote AGENTS.md: %q", data)
	}
	claude, err := os.ReadFile(filepath.Join(root, ClaudeMDName))
	if err != nil {
		t.Fatal(err)
	}
	if string(claude) != "pointer\n" {
		t.Fatalf("CLAUDE.md changed: %q", claude)
	}
	if len(result.Sources) != 2 {
		t.Fatalf("sources=%v", result.Sources)
	}
}

// Dogfood #18: comentário dentro de fence bash e nome de ferramenta solto
// na prosa não viram dicas.
func TestExtractCommandHints_SkipsCommentsAndBareTools(t *testing.T) {
	content := "## Setup\n\n```bash\n# `make install` sobrescreve o build local\n" +
		"go vet ./...   # roda o vet\ntb mem check\n```\n\n" +
		"- `golangci-lint` não está no PATH; use `go vet ./...`.\n- rode `make` para tudo.\n"
	hints := ExtractCommandHints(content)
	want := []string{"go vet ./...", "make"}
	if strings.Join(hints, "|") != strings.Join(want, "|") {
		t.Fatalf("hints = %q, want %q", hints, want)
	}
}
