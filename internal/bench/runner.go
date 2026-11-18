package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/log"
)

type AgentConfig struct {
	ID string `yaml:"id" json:"id"`

	// Placeholders: {prompt}, {workdir}, {timeout} (see ExpandAgentCommand).
	Command string `yaml:"command" json:"command"`

	// "json" or "none".
	OutputParser string `yaml:"output_parser" json:"output_parser"`

	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

type Runner struct {
	agent       AgentConfig
	tasksDir    string
	fixturesDir string
	outputDir   string
	repetitions int
}

func NewRunner(agent AgentConfig, tasksDir, fixturesDir, outputDir string, repetitions int) *Runner {
	return &Runner{
		agent:       agent,
		tasksDir:    tasksDir,
		fixturesDir: fixturesDir,
		outputDir:   outputDir,
		repetitions: repetitions,
	}
}

func (r *Runner) Run(ctx context.Context, taskIDs []string) ([]Result, error) {
	var tasks []*Task
	if len(taskIDs) == 0 {
		var err error
		tasks, err = LoadTasksFromDir(r.tasksDir)
		if err != nil {
			return nil, fmt.Errorf("loading tasks: %w", err)
		}
	} else {
		for _, id := range taskIDs {
			taskPath := filepath.Join(r.tasksDir, id, "task.yaml")
			task, err := LoadTask(taskPath)
			if err != nil {
				return nil, fmt.Errorf("loading task %s: %w", id, err)
			}
			tasks = append(tasks, task)
		}
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks found")
	}

	log.Info("starting benchmark", "agent", r.agent.ID, "tasks", len(tasks), "repetitions", r.repetitions)

	var results []Result

	for _, task := range tasks {
		for rep := 1; rep <= r.repetitions; rep++ {
			select {
			case <-ctx.Done():
				return results, ctx.Err()
			default:
			}

			log.Info("running task", "task", task.ID, "rep", rep, "total", r.repetitions)

			result, err := r.runTask(ctx, task, rep)
			if err != nil {
				log.Error("task failed", "task", task.ID, "rep", rep, "error", err)
				result = &Result{
					TaskID:     task.ID,
					AgentID:    r.agent.ID,
					Repetition: rep,
					Passed:     false,
					Error:      err.Error(),
					StartTime:  time.Now(),
					EndTime:    time.Now(),
				}
			}

			results = append(results, *result)
		}
	}

	return results, nil
}

func (r *Runner) runTask(ctx context.Context, task *Task, rep int) (*Result, error) {
	result := &Result{
		TaskID:     task.ID,
		AgentID:    r.agent.ID,
		Repetition: rep,
		StartTime:  time.Now(),
	}

	workDir, err := r.prepareWorkDir(task)
	if err != nil {
		return nil, fmt.Errorf("preparing work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	taskCtx, cancel := context.WithTimeout(ctx, task.TimeoutDuration())
	defer cancel()

	agentOutput, exitCode, err := r.executeAgent(taskCtx, task, workDir)
	result.ExitCode = exitCode
	result.AgentOutput = truncateString(agentOutput, 10000)

	if err != nil {
		result.Error = err.Error()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime).Seconds()
		return result, nil // agent failure is a failed result, not an error
	}

	// Extract tokens/tool_calls from the agent output.
	if r.agent.OutputParser == "json" {
		r.parseAgentOutput(agentOutput, result)
	}

	verifyCtx, verifyCancel := context.WithTimeout(ctx, task.Verification.TimeoutDuration())
	defer verifyCancel()

	verifyStart := time.Now()
	verifyOutput, verifyExit, verifyErr := r.executeVerification(verifyCtx, task, workDir)
	result.VerificationDuration = time.Since(verifyStart).Seconds()
	result.VerificationExitCode = verifyExit
	result.VerificationOutput = truncateString(verifyOutput, 5000)

	if verifyErr != nil {
		result.Error = fmt.Sprintf("verification error: %v", verifyErr)
	}

	result.Passed = exitCode == 0 && verifyExit == task.Verification.ExpectedExitCode
	if exitCode != 0 && result.Error == "" {
		result.Error = fmt.Sprintf("agent exited with code %d", exitCode)
	}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime).Seconds()

	return result, nil
}

func (r *Runner) prepareWorkDir(task *Task) (string, error) {
	workDir, err := os.MkdirTemp("", fmt.Sprintf("tb-bench-%s-*", task.ID))
	if err != nil {
		return "", err
	}

	if task.Fixture != "" {
		fixtureDir := filepath.Join(r.fixturesDir, task.Fixture)
		if _, err := os.Stat(fixtureDir); err == nil {
			if err := copyDir(fixtureDir, workDir); err != nil {
				os.RemoveAll(workDir)
				return "", fmt.Errorf("copying fixture: %w", err)
			}
		}
	}

	return workDir, nil
}

//nolint:gosec // G204: command is intentionally constructed from user config
func (r *Runner) executeAgent(ctx context.Context, task *Task, workDir string) (output string, exitCode int, err error) {
	cmd := ExpandAgentCommand(r.agent.Command, task.Prompt, workDir, task.Timeout)

	proc := exec.CommandContext(ctx, "sh", "-c", cmd)
	proc.Dir = workDir

	proc.Env = os.Environ()
	proc.Env = append(proc.Env,
		"TB_BENCH_PROMPT="+task.Prompt,
		"TB_BENCH_WORKDIR="+workDir,
		"TB_BENCH_TIMEOUT="+task.Timeout,
	)
	for k, v := range r.agent.Env {
		proc.Env = append(proc.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	proc.Stdout = &stdout
	proc.Stderr = &stderr

	runErr := proc.Run()
	output = stdout.String() + stderr.String()

	exitCode = 0
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
		runErr = nil
	}

	return output, exitCode, runErr
}

// ExpandAgentCommand interpolates {prompt}, {workdir} and {timeout} with POSIX
// single-quoting so the result is safe for sh -c.
func ExpandAgentCommand(template, prompt, workDir, timeout string) string {
	cmd := template
	cmd = strings.ReplaceAll(cmd, "{prompt}", shellSingleQuote(prompt))
	cmd = strings.ReplaceAll(cmd, "{workdir}", shellSingleQuote(workDir))
	cmd = strings.ReplaceAll(cmd, "{timeout}", shellSingleQuote(timeout))
	return cmd
}

//nolint:gosec // G204: command is intentionally from task config
func (r *Runner) executeVerification(ctx context.Context, task *Task, workDir string) (output string, exitCode int, err error) {
	proc := exec.CommandContext(ctx, "sh", "-c", task.Verification.Command)
	proc.Dir = workDir

	var stdout, stderr bytes.Buffer
	proc.Stdout = &stdout
	proc.Stderr = &stderr

	runErr := proc.Run()
	output = stdout.String() + stderr.String()

	exitCode = 0
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
		runErr = nil
	}

	return output, exitCode, runErr
}

func (r *Runner) parseAgentOutput(output string, result *Result) {
	// The JSON may be surrounded by other output.
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start == -1 || end == -1 || end <= start {
		return
	}

	jsonStr := output[start : end+1]
	var data struct {
		Tokens struct {
			Input     int     `json:"input"`
			Output    int     `json:"output"`
			Total     int     `json:"total"`
			CostUSD   float64 `json:"cost_usd"`
			CacheRead int     `json:"cache_read"`
		} `json:"tokens"`
		ToolCalls int `json:"tool_calls"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return
	}

	if data.Tokens.Total > 0 {
		result.Tokens = &TokenInfo{
			Input:     data.Tokens.Input,
			Output:    data.Tokens.Output,
			Total:     data.Tokens.Total,
			CostUSD:   data.Tokens.CostUSD,
			CacheRead: data.Tokens.CacheRead,
		}
	}
	result.ToolCalls = data.ToolCalls
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... [truncated]"
}
