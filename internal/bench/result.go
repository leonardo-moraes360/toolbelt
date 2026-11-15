package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Result representa o resultado de uma execução de benchmark.
type Result struct {
	// TaskID identificador da tarefa.
	TaskID string `json:"task_id"`

	// AgentID identificador do agente usado.
	AgentID string `json:"agent_id"`

	// Repetition número da repetição (1-based).
	Repetition int `json:"repetition"`

	// Passed indica se a verificação passou.
	Passed bool `json:"passed"`

	// StartTime quando a execução começou.
	StartTime time.Time `json:"start_time"`

	// EndTime quando a execução terminou.
	EndTime time.Time `json:"end_time"`

	// Duration duração total em segundos.
	Duration float64 `json:"duration_seconds"`

	// VerificationDuration duração da verificação em segundos.
	VerificationDuration float64 `json:"verification_duration_seconds"`

	// ExitCode código de saída do agente.
	ExitCode int `json:"exit_code"`

	// VerificationExitCode código de saída da verificação.
	VerificationExitCode int `json:"verification_exit_code"`

	// Tokens informações de tokens (se disponível).
	Tokens *TokenInfo `json:"tokens,omitempty"`

	// ToolCalls número de chamadas de ferramentas (se disponível).
	ToolCalls int `json:"tool_calls,omitempty"`

	// Retries número de tentativas.
	Retries int `json:"retries"`

	// Error mensagem de erro, se houver.
	Error string `json:"error,omitempty"`

	// AgentOutput saída do agente (truncada).
	AgentOutput string `json:"agent_output,omitempty"`

	// VerificationOutput saída da verificação (truncada).
	VerificationOutput string `json:"verification_output,omitempty"`
}

// TokenInfo contém informações sobre uso de tokens.
type TokenInfo struct {
	Input     int     `json:"input"`
	Output    int     `json:"output"`
	Total     int     `json:"total"`
	CostUSD   float64 `json:"cost_usd,omitempty"`
	CacheRead int     `json:"cache_read,omitempty"`
}

// Summary contém estatísticas agregadas de uma suite de benchmark.
type Summary struct {
	// BenchmarkID identificador único do benchmark.
	BenchmarkID string `json:"benchmark_id"`

	// AgentID identificador do agente.
	AgentID string `json:"agent_id"`

	// StartTime quando o benchmark começou.
	StartTime time.Time `json:"start_time"`

	// EndTime quando o benchmark terminou.
	EndTime time.Time `json:"end_time"`

	// TotalTasks número total de tarefas.
	TotalTasks int `json:"total_tasks"`

	// TotalRepetitions número total de repetições.
	TotalRepetitions int `json:"total_repetitions"`

	// Metrics métricas agregadas.
	Metrics Metrics `json:"metrics"`
}

// Metrics contém as métricas de benchmark.
type Metrics struct {
	// PassAt1 taxa de sucesso na primeira tentativa.
	PassAt1 float64 `json:"pass_at_1"`

	// PassAtK taxa de sucesso em k tentativas.
	PassAtK map[int]float64 `json:"pass_at_k,omitempty"`

	// AvgDuration duração média em segundos.
	AvgDuration float64 `json:"avg_duration_seconds"`

	// AvgTokens média de tokens por tarefa.
	AvgTokens float64 `json:"avg_tokens,omitempty"`

	// TotalTokens total de tokens consumidos.
	TotalTokens int `json:"total_tokens,omitempty"`

	// TotalCostUSD custo total em USD.
	TotalCostUSD float64 `json:"total_cost_usd,omitempty"`

	// AvgToolCalls média de chamadas de ferramentas.
	AvgToolCalls float64 `json:"avg_tool_calls,omitempty"`

	// AvgRetries média de tentativas.
	AvgRetries float64 `json:"avg_retries"`
}

// WriteJSONL escreve resultados em formato JSONL (JSON Lines).
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

// ReadJSONL lê resultados de um arquivo JSONL.
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

// CalculateSummary calcula estatísticas agregadas dos resultados.
//
//nolint:gocyclo // complexity is acceptable for this aggregation function
func CalculateSummary(benchmarkID, agentID string, results []Result) *Summary {
	if len(results) == 0 {
		return nil
	}

	summary := &Summary{
		BenchmarkID: benchmarkID,
		AgentID:     agentID,
	}

	// Conta tarefas únicas e repetições por tarefa
	taskCounts := make(map[string]int)
	for i := range results {
		taskCounts[results[i].TaskID]++
	}
	summary.TotalTasks = len(taskCounts)

	// TotalRepetitions é o número de repetições por tarefa (assumindo igual para todas)
	if summary.TotalTasks > 0 {
		summary.TotalRepetitions = len(results) / summary.TotalTasks
	}

	// Calcula métricas
	var passed, totalDuration, totalTokens, totalToolCalls, totalRetries int
	var totalCost float64

	// Agrupa por tarefa para pass@1
	taskResults := make(map[string][]Result)
	for i := range results {
		r := &results[i]
		taskResults[r.TaskID] = append(taskResults[r.TaskID], *r)

		totalDuration += int(r.Duration * 1000) // ms para evitar float errors
		totalRetries += r.Retries
		if r.Passed {
			passed++
		}
		if r.Tokens != nil {
			totalTokens += r.Tokens.Total
			totalCost += r.Tokens.CostUSD
		}
		totalToolCalls += r.ToolCalls

		// Track start/end times
		if summary.StartTime.IsZero() || r.StartTime.Before(summary.StartTime) {
			summary.StartTime = r.StartTime
		}
		if r.EndTime.After(summary.EndTime) {
			summary.EndTime = r.EndTime
		}
	}

	// pass@1: proporção de tarefas onde a primeira tentativa passou
	pass1Count := 0
	for _, reps := range taskResults {
		if len(reps) > 0 && reps[0].Passed {
			pass1Count++
		}
	}

	n := float64(len(results))
	summary.Metrics = Metrics{
		PassAt1:      float64(pass1Count) / float64(summary.TotalTasks),
		AvgDuration:  float64(totalDuration) / n / 1000, // volta para segundos
		AvgTokens:    float64(totalTokens) / n,
		TotalTokens:  totalTokens,
		TotalCostUSD: totalCost,
		AvgToolCalls: float64(totalToolCalls) / n,
		AvgRetries:   float64(totalRetries) / n,
	}

	// pass@k para k > 1
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

// CalculatePassAtK calcula a probabilidade de passar com k tentativas.
// Uses the estimator: pass@k = 1 - C(n-c, k) / C(n, k)
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
