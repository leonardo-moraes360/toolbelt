package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type FakeAgentConfig struct {
	// Keyed by task ID; tasks not listed use DefaultBehavior.
	Behaviors       map[string]FakeBehavior `json:"behaviors"`
	DefaultBehavior FakeBehavior            `json:"default_behavior"`
}

type FakeBehavior struct {
	// ShouldPass produces the correct result.
	ShouldPass bool `json:"should_pass"`

	DelayMs int `json:"delay_ms"`

	TokensUsed int `json:"tokens_used"`

	ToolCalls int `json:"tool_calls"`

	// Paths relative to the workdir.
	FilesToCreate map[string]string `json:"files_to_create"`

	FilesToModify map[string]string `json:"files_to_modify"`
}

type FakeAgentOutput struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Tokens  struct {
		Input   int     `json:"input"`
		Output  int     `json:"output"`
		Total   int     `json:"total"`
		CostUSD float64 `json:"cost_usd"`
	} `json:"tokens"`
	ToolCalls int `json:"tool_calls"`
}

// RunFakeAgent backs the fake-agent binary used to simulate an agent.
func RunFakeAgent(workDir, prompt, configPath string) error {
	config := FakeAgentConfig{
		DefaultBehavior: FakeBehavior{
			ShouldPass: true,
			TokensUsed: 1000,
			ToolCalls:  5,
		},
	}

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err == nil {
			_ = json.Unmarshal(data, &config)
		}
	}

	// Task ID comes from the workdir (last segment of the temp path).
	taskID := detectTaskID(workDir)

	behavior := config.DefaultBehavior
	if b, ok := config.Behaviors[taskID]; ok {
		behavior = b
	}

	for path, content := range behavior.FilesToCreate {
		fullPath := filepath.Join(workDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
			return fmt.Errorf("creating file %s: %w", path, err)
		}
	}

	for path, content := range behavior.FilesToModify {
		fullPath := filepath.Join(workDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
			return fmt.Errorf("modifying file %s: %w", path, err)
		}
	}

	output := FakeAgentOutput{
		Status:  "completed",
		Message: "Task processed by fake agent",
	}
	output.Tokens.Input = behavior.TokensUsed / 4
	output.Tokens.Output = behavior.TokensUsed * 3 / 4
	output.Tokens.Total = behavior.TokensUsed
	output.Tokens.CostUSD = float64(behavior.TokensUsed) * 0.00001
	output.ToolCalls = behavior.ToolCalls

	if !behavior.ShouldPass {
		output.Status = "failed"
		output.Message = "Simulated failure by fake agent"
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func detectTaskID(workDir string) string {
	base := filepath.Base(workDir)
	// Format: tb-bench-{taskID}-{random}
	if len(base) > 9 && base[:9] == "tb-bench-" {
		rest := base[9:]
		for i := len(rest) - 1; i >= 0; i-- {
			if rest[i] == '-' && i > 0 {
				return rest[:i]
			}
		}
	}
	return ""
}
