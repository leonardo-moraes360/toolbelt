package bench

import (
	"os"
	"path/filepath"
	"strings"
)

const FakeAgentSubcommand = "__fake-agent"

func FakeAgentCommand(configPath string) string {
	exe := resolveSelfExecutable()
	quoted := shellSingleQuote(exe)
	parts := []string{quoted, FakeAgentSubcommand}
	if configPath != "" {
		parts = append(parts, "--config", shellSingleQuote(configPath))
	}
	parts = append(parts, "--workdir", "{workdir}", "--prompt", "{prompt}")
	return strings.Join(parts, " ")
}

func resolveSelfExecutable() string {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return "tb"
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" {
		return resolved
	}
	return exe
}
