package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Result struct {
	TaskID string `json:"task_id"`

	AgentID string `json:"agent_id"`

	// A/B variant label, e.g. graph-on, graph-off.
	ConfigLabel string `json:"config_label,omitempty"`

	Flags map[string]string `json:"flags,omitempty"`

	// 1-based.
	Repetition int `json:"repetition"`

	// Agent exited 0 and verification exited with expected_exit_code.
	Passed bool `json:"passed"`

	StartTime time.Time `json:"start_time"`

	EndTime time.Time `json:"end_time"`

	Duration float64 `json:"duration_seconds"`

	VerificationDuration float64 `json:"verification_duration_seconds"`

	ExitCode int `json:"exit_code"`

	VerificationExitCode int `json:"verification_exit_code"`

	// nil when the agent output reports none.
	Tokens *TokenInfo `json:"tokens,omitempty"`

	ToolCalls int `json:"tool_calls,omitempty"`

	Retries int `json:"retries"`

	Error string `json:"error,omitempty"`

	// Truncated.
	AgentOutput string `json:"agent_output,omitempty"`

	// Truncated.
	VerificationOutput string `json:"verification_output,omitempty"`
}

type TokenInfo struct {
	Input     int     `json:"input"`
	Output    int     `json:"output"`
	Total     int     `json:"total"`
	CostUSD   float64 `json:"cost_usd,omitempty"`
	CacheRead int     `json:"cache_read,omitempty"`
}

type Summary struct {
	BenchmarkID string `json:"benchmark_id"`

	AgentID string `json:"agent_id"`

	StartTime time.Time `json:"start_time"`

	EndTime time.Time `json:"end_time"`

	TotalTasks int `json:"total_tasks"`

	// Repetitions per task.
	TotalRepetitions int `json:"total_repetitions"`

	Metrics Metrics `json:"metrics"`
}

type Metrics struct {
	// Success rate on the first attempt.
	PassAt1 float64 `json:"pass_at_1"`

	// Success rate within k attempts.
	PassAtK map[int]float64 `json:"pass_at_k,omitempty"`

	AvgDuration float64 `json:"avg_duration_seconds"`

	// Per task.
	AvgTokens float64 `json:"avg_tokens,omitempty"`

	TotalTokens int `json:"total_tokens,omitempty"`

	TotalCostUSD float64 `json:"total_cost_usd,omitempty"`

	AvgToolCalls float64 `json:"avg_tool_calls,omitempty"`

	AvgRetries float64 `json:"avg_retries"`
}

func WriteJSONL(results []Result, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for i := range results {
		if err := encoder.Encode(&results[i]); err != nil {
			return fmt.Errorf("encoding result: %w", err)
		}
	}

	return nil
}

func ReadJSONL(path string) ([]Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	var results []Result
	decoder := json.NewDecoder(f)
	for decoder.More() {
		var r Result
		if err := decoder.Decode(&r); err != nil {
			return nil, fmt.Errorf("decoding result: %w", err)
		}
		results = append(results, r)
	}

	return results, nil
}

//nolint:gocyclo // complexity is acceptable for this aggregation function
func CalculateSummary(benchmarkID, agentID string, results []Result) *Summary {
	if len(results) == 0 {
		return nil
	}

	summary := &Summary{
		BenchmarkID: benchmarkID,
		AgentID:     agentID,
	}

	taskCounts := make(map[string]int)
	for i := range results {
		taskCounts[results[i].TaskID]++
	}
	summary.TotalTasks = len(taskCounts)

	// Assumes every task has the same number of repetitions.
	if summary.TotalTasks > 0 {
		summary.TotalRepetitions = len(results) / summary.TotalTasks
	}

	var passed, totalDuration, totalTokens, totalToolCalls, totalRetries int
	var totalCost float64

	taskResults := make(map[string][]Result)
	for i := range results {
		r := &results[i]
		taskResults[r.TaskID] = append(taskResults[r.TaskID], *r)

		totalDuration += int(r.Duration * 1000) // ms, to avoid float error
		totalRetries += r.Retries
		if r.Passed {
			passed++
		}
		if r.Tokens != nil {
			totalTokens += r.Tokens.Total
			totalCost += r.Tokens.CostUSD
		}
		totalToolCalls += r.ToolCalls

		if summary.StartTime.IsZero() || r.StartTime.Before(summary.StartTime) {
			summary.StartTime = r.StartTime
		}
		if r.EndTime.After(summary.EndTime) {
			summary.EndTime = r.EndTime
		}
	}

	// pass@1: fraction of tasks whose first attempt passed.
	pass1Count := 0
	for _, reps := range taskResults {
		if len(reps) > 0 && reps[0].Passed {
			pass1Count++
		}
	}

	n := float64(len(results))
	summary.Metrics = Metrics{
		PassAt1:      float64(pass1Count) / float64(summary.TotalTasks),
		AvgDuration:  float64(totalDuration) / n / 1000, // back to seconds
		AvgTokens:    float64(totalTokens) / n,
		TotalTokens:  totalTokens,
		TotalCostUSD: totalCost,
		AvgToolCalls: float64(totalToolCalls) / n,
		AvgRetries:   float64(totalRetries) / n,
	}

	// pass@k for k > 1
	summary.Metrics.PassAtK = make(map[int]float64)
	for _, k := range []int{3, 5, 10} {
		passK := 0
		for _, reps := range taskResults {
			for i := 0; i < k && i < len(reps); i++ {
				if reps[i].Passed {
					passK++
					break
				}
			}
		}
		if k <= len(results)/summary.TotalTasks {
			summary.Metrics.PassAtK[k] = float64(passK) / float64(summary.TotalTasks)
		}
	}

	return summary
}

// CalculatePassAtK uses the estimator: pass@k = 1 - C(n-c, k) / C(n, k)
// where n = total runs, c = passed runs, k = number of attempts
func CalculatePassAtK(passed, total, k int) float64 {
	if total == 0 || k <= 0 {
		return 0.0
	}
	if passed >= total {
		return 1.0
	}
	if k >= total {
		if passed > 0 {
			return 1.0
		}
		return 0.0
	}

	// Simple case: if any passed and k >= total, guaranteed to find at least one
	if passed > 0 && k >= total {
		return 1.0
	}

	// 1 - C(n-c, k) / C(n, k)
	// = 1 - product for i in 0..k-1 of (n-c-i)/(n-i)
	notPass := 1.0
	for i := 0; i < k; i++ {
		notPass *= float64(total-passed-i) / float64(total-i)
	}
	return 1.0 - notPass
}
