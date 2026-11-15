package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FakeAgentConfig configuração do agente fake para testes.
type FakeAgentConfig struct {
	// Behaviors define comportamentos por task ID.
	// Se não definido para uma task, usa DefaultBehavior.
	Behaviors       map[string]FakeBehavior `json:"behaviors"`
	DefaultBehavior FakeBehavior            `json:"default_behavior"`
}

// FakeBehavior define como o agente fake responde a uma tarefa.
type FakeBehavior struct {
	// ShouldPass se true, produz o resultado correto.
	ShouldPass bool `json:"should_pass"`

	// DelayMs atraso artificial em milissegundos.
	DelayMs int `json:"delay_ms"`

	// TokensUsed tokens simulados.
	TokensUsed int `json:"tokens_used"`

	// ToolCalls chamadas de ferramentas simuladas.
	ToolCalls int `json:"tool_calls"`

	// FilesToCreate arquivos a criar no workdir.
	FilesToCreate map[string]string `json:"files_to_create"`

	// FilesToModify modificações a fazer em arquivos existentes.
	FilesToModify map[string]string `json:"files_to_modify"`
}

// FakeAgentOutput saída JSON do agente fake.
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

// RunFakeAgent executa o agente fake para uma tarefa.
// Isso é usado pelo script fake-agent para simular um agente.
func RunFakeAgent(workDir, prompt, configPath string) error {
	// Carrega configuração
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

	// Detecta task ID do workdir (último segmento do path temporário)
	taskID := detectTaskID(workDir)

	// Busca comportamento específico ou usa default
	behavior := config.DefaultBehavior
	if b, ok := config.Behaviors[taskID]; ok {
		behavior = b
	}

	// Aplica modificações
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

	// Gera output
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

// detectTaskID tenta extrair o ID da tarefa do diretório de trabalho.
func detectTaskID(workDir string) string {
	base := filepath.Base(workDir)
	// Formato: tb-bench-{taskID}-{random}
	// Tenta extrair taskID
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
