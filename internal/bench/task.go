// Package bench fornece o harness de benchmark para medir performance de agentes.
package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Task representa uma tarefa de benchmark.
type Task struct {
	// ID identificador único da tarefa.
	ID string `yaml:"id" json:"id"`

	// Name nome legível da tarefa.
	Name string `yaml:"name" json:"name"`

	// Description descrição detalhada.
	Description string `yaml:"description" json:"description"`

	// Category categoria da tarefa (bugfix, feature, refactor, etc).
	Category string `yaml:"category" json:"category"`

	// Difficulty nível de dificuldade (easy, medium, hard).
	Difficulty string `yaml:"difficulty" json:"difficulty"`

	// Prompt é a instrução para o agente.
	Prompt string `yaml:"prompt" json:"prompt"`

	// Fixture é o caminho para o diretório de fixture (código inicial).
	Fixture string `yaml:"fixture" json:"fixture"`

	// Verification define como verificar o resultado.
	Verification Verification `yaml:"verification" json:"verification"`

	// Timeout tempo máximo para completar a tarefa.
	Timeout string `yaml:"timeout" json:"timeout"`

	// HiddenTests caminho para testes ocultos (opcional).
	HiddenTests string `yaml:"hidden_tests,omitempty" json:"hidden_tests,omitempty"`

	// ExpectedChanges arquivos que devem ser modificados (opcional).
	ExpectedChanges []string `yaml:"expected_changes,omitempty" json:"expected_changes,omitempty"`

	// MaxTokens limite de tokens (opcional).
	MaxTokens int `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty"`
}

// Verification define como verificar o resultado de uma tarefa.
type Verification struct {
	// Command command shell para verificar (ex: "go test ./...")
	Command string `yaml:"command" json:"command"`

	// Timeout tempo máximo para verificação.
	Timeout string `yaml:"timeout" json:"timeout"`

	// ExpectedExitCode código de saída esperado (default: 0).
	ExpectedExitCode int `yaml:"expected_exit_code" json:"expected_exit_code"`
}

// LoadTask carrega uma tarefa de um arquivo YAML.
func LoadTask(path string) (*Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading task file: %w", err)
	}

	var task Task
	if err := yaml.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("parsing task YAML: %w", err)
	}

	if err := task.Validate(); err != nil {
		return nil, fmt.Errorf("validating task: %w", err)
	}

	return &task, nil
}

// LoadTasksFromDir carrega todas as tarefas de um diretório.
func LoadTasksFromDir(dir string) ([]*Task, error) {
	var tasks []*Task

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading tasks dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		taskPath := filepath.Join(dir, entry.Name(), "task.yaml")
		if _, err := os.Stat(taskPath); os.IsNotExist(err) {
			continue
		}

		task, err := LoadTask(taskPath)
		if err != nil {
			return nil, fmt.Errorf("loading task %s: %w", entry.Name(), err)
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// Validate verifica se a tarefa está bem formada.
func (t *Task) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("task ID is required")
	}
	if t.Prompt == "" {
		return fmt.Errorf("task prompt is required")
	}
	if t.Verification.Command == "" {
		return fmt.Errorf("verification command is required")
	}
	return nil
}

// TimeoutDuration retorna o timeout como time.Duration.
func (t *Task) TimeoutDuration() time.Duration {
	if t.Timeout == "" {
		return 10 * time.Minute
	}
	d, err := time.ParseDuration(t.Timeout)
	if err != nil {
		return 10 * time.Minute
	}
	return d
}

// VerificationTimeoutDuration retorna o timeout de verificação como time.Duration.
func (v *Verification) TimeoutDuration() time.Duration {
	if v.Timeout == "" {
		return 2 * time.Minute
	}
	d, err := time.ParseDuration(v.Timeout)
	if err != nil {
		return 2 * time.Minute
	}
	return d
}
