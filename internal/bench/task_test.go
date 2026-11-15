package bench

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadTask(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "test-task")
	os.MkdirAll(taskDir, 0o755)

	taskYAML := `
id: test-task
name: Test Task
description: A test task
category: test
difficulty: easy
prompt: "Fix the bug in main.go"
fixture: bugfix-go
verification:
  command: "go test ./..."
  timeout: "1m"
  expected_exit_code: 0
timeout: "5m"
`
	taskPath := filepath.Join(taskDir, "task.yaml")
	if err := os.WriteFile(taskPath, []byte(taskYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	task, err := LoadTask(taskPath)
	if err != nil {
		t.Fatalf("LoadTask failed: %v", err)
	}

	if task.ID != "test-task" {
		t.Errorf("ID = %q, want %q", task.ID, "test-task")
	}
	if task.Category != "test" {
		t.Errorf("Category = %q, want %q", task.Category, "test")
	}
	if task.Verification.Command != "go test ./..." {
		t.Errorf("Verification.Command = %q, want %q", task.Verification.Command, "go test ./...")
	}
}

func TestTask_Validate(t *testing.T) {
	tests := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{
			name: "valid task",
			task: Task{
				ID:     "test",
				Prompt: "do something",
				Verification: Verification{
					Command: "test -f output.txt",
				},
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			task: Task{
				Prompt: "do something",
				Verification: Verification{
					Command: "test -f output.txt",
				},
			},
			wantErr: true,
		},
		{
			name: "missing prompt",
			task: Task{
				ID: "test",
				Verification: Verification{
					Command: "test -f output.txt",
				},
			},
			wantErr: true,
		},
		{
			name: "missing verification",
			task: Task{
				ID:     "test",
				Prompt: "do something",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTask_TimeoutDuration(t *testing.T) {
	tests := []struct {
		timeout string
		want    time.Duration
	}{
		{"5m", 5 * time.Minute},
		{"30s", 30 * time.Second},
		{"", 10 * time.Minute},        // default
		{"invalid", 10 * time.Minute}, // fallback
	}

	for _, tt := range tests {
		t.Run(tt.timeout, func(t *testing.T) {
			task := Task{Timeout: tt.timeout}
			got := task.TimeoutDuration()
			if got != tt.want {
				t.Errorf("TimeoutDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadTasksFromDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Cria duas tarefas
	for _, id := range []string{"task-1", "task-2"} {
		taskDir := filepath.Join(tmpDir, id)
		os.MkdirAll(taskDir, 0o755)

		taskYAML := `
id: ` + id + `
prompt: "Do task"
verification:
  command: "echo ok"
`
		os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0o644)
	}

	tasks, err := LoadTasksFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadTasksFromDir failed: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("len(tasks) = %d, want %d", len(tasks), 2)
	}
}
