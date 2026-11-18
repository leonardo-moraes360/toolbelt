// Package bench is the harness for benchmarking coding agents.
package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Task struct {
	ID string `yaml:"id" json:"id"`

	Name string `yaml:"name" json:"name"`

	Description string `yaml:"description" json:"description"`

	// bugfix, feature, refactor, etc.
	Category string `yaml:"category" json:"category"`

	// easy, medium or hard.
	Difficulty string `yaml:"difficulty" json:"difficulty"`

	Prompt string `yaml:"prompt" json:"prompt"`

	// Directory holding the starting code.
	Fixture string `yaml:"fixture" json:"fixture"`

	Verification Verification `yaml:"verification" json:"verification"`

	Timeout string `yaml:"timeout" json:"timeout"`

	HiddenTests string `yaml:"hidden_tests,omitempty" json:"hidden_tests,omitempty"`

	// Files that must be modified.
	ExpectedChanges []string `yaml:"expected_changes,omitempty" json:"expected_changes,omitempty"`

	MaxTokens int `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty"`
}

type Verification struct {
	// Shell command, e.g. "go test ./...".
	Command string `yaml:"command" json:"command"`

	Timeout string `yaml:"timeout" json:"timeout"`

	ExpectedExitCode int `yaml:"expected_exit_code" json:"expected_exit_code"`
}

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
