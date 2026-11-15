package bench

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// WriteMarkdownReport gera um relatório Markdown dos resultados.
func WriteMarkdownReport(w io.Writer, summary *Summary, results []Result) error {
	fmt.Fprintf(w, "# Relatório de Benchmark\n\n")
	fmt.Fprintf(w, "**ID**: %s\n", summary.BenchmarkID)
	fmt.Fprintf(w, "**Agente**: %s\n", summary.AgentID)
	fmt.Fprintf(w, "**Início**: %s\n", summary.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "**Fim**: %s\n", summary.EndTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "\n---\n\n")

	// Resumo
	fmt.Fprintf(w, "## Resumo\n\n")
	fmt.Fprintf(w, "| Métrica | Valor |\n")
	fmt.Fprintf(w, "|---------|-------|\n")
	fmt.Fprintf(w, "| Tarefas | %d |\n", summary.TotalTasks)
	fmt.Fprintf(w, "| Repetições | %d |\n", summary.TotalRepetitions)
	fmt.Fprintf(w, "| **pass@1** | **%.1f%%** |\n", summary.Metrics.PassAt1*100)

	for _, k := range []int{3, 5, 10} {
		if v, ok := summary.Metrics.PassAtK[k]; ok {
			fmt.Fprintf(w, "| pass@%d | %.1f%% |\n", k, v*100)
		}
	}

	fmt.Fprintf(w, "| Tempo médio | %.1fs |\n", summary.Metrics.AvgDuration)

	if summary.Metrics.TotalTokens > 0 {
		fmt.Fprintf(w, "| Tokens totais | %d |\n", summary.Metrics.TotalTokens)
		fmt.Fprintf(w, "| Tokens médio | %.0f |\n", summary.Metrics.AvgTokens)
	}
	if summary.Metrics.TotalCostUSD > 0 {
		fmt.Fprintf(w, "| Custo total | $%.4f |\n", summary.Metrics.TotalCostUSD)
	}
	if summary.Metrics.AvgToolCalls > 0 {
		fmt.Fprintf(w, "| Tool calls médio | %.1f |\n", summary.Metrics.AvgToolCalls)
	}
	fmt.Fprintf(w, "| Retries médio | %.1f |\n", summary.Metrics.AvgRetries)
	fmt.Fprintf(w, "\n")

	// Resultados por tarefa
	fmt.Fprintf(w, "## Resultados por Tarefa\n\n")
	fmt.Fprintf(w, "| Tarefa | Rep | Passou | Tempo | Tokens | Erro |\n")
	fmt.Fprintf(w, "|--------|-----|--------|-------|--------|------|\n")

	// Ordena por tarefa e repetição
	sorted := make([]Result, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].TaskID != sorted[j].TaskID {
			return sorted[i].TaskID < sorted[j].TaskID
		}
		return sorted[i].Repetition < sorted[j].Repetition
	})

	for _, r := range sorted {
		passed := "❌"
		if r.Passed {
			passed = "✅"
		}

		tokens := "-"
		if r.Tokens != nil {
			tokens = fmt.Sprintf("%d", r.Tokens.Total)
		}

		errMsg := "-"
		if r.Error != "" {
			errMsg = truncateString(r.Error, 50)
			errMsg = strings.ReplaceAll(errMsg, "|", "\\|")
			errMsg = strings.ReplaceAll(errMsg, "\n", " ")
		}

		fmt.Fprintf(w, "| %s | %d | %s | %.1fs | %s | %s |\n",
			r.TaskID, r.Repetition, passed, r.Duration, tokens, errMsg)
	}

	return nil
}

// WriteComparisonReport gera um relatório comparando dois benchmarks.
func WriteComparisonReport(w io.Writer, before, after *Summary) error {
	fmt.Fprintf(w, "# Comparação de Benchmarks\n\n")
	fmt.Fprintf(w, "| Métrica | Antes | Depois | Δ |\n")
	fmt.Fprintf(w, "|---------|-------|--------|---|\n")

	// pass@1
	delta := (after.Metrics.PassAt1 - before.Metrics.PassAt1) * 100
	sign := "+"
	if delta < 0 {
		sign = ""
	}
	fmt.Fprintf(w, "| pass@1 | %.1f%% | %.1f%% | %s%.1f%% |\n",
		before.Metrics.PassAt1*100, after.Metrics.PassAt1*100, sign, delta)

	// Tempo médio
	deltaTime := after.Metrics.AvgDuration - before.Metrics.AvgDuration
	sign = "+"
	if deltaTime < 0 {
		sign = ""
	}
	fmt.Fprintf(w, "| Tempo médio | %.1fs | %.1fs | %s%.1fs |\n",
		before.Metrics.AvgDuration, after.Metrics.AvgDuration, sign, deltaTime)

	// Tokens
	if before.Metrics.TotalTokens > 0 && after.Metrics.TotalTokens > 0 {
		deltaTokens := (float64(after.Metrics.AvgTokens) - float64(before.Metrics.AvgTokens)) / float64(before.Metrics.AvgTokens) * 100
		sign = "+"
		if deltaTokens < 0 {
			sign = ""
		}
		fmt.Fprintf(w, "| Tokens médio | %.0f | %.0f | %s%.1f%% |\n",
			before.Metrics.AvgTokens, after.Metrics.AvgTokens, sign, deltaTokens)
	}

	// Custo
	if before.Metrics.TotalCostUSD > 0 && after.Metrics.TotalCostUSD > 0 {
		deltaCost := (after.Metrics.TotalCostUSD - before.Metrics.TotalCostUSD) / before.Metrics.TotalCostUSD * 100
		sign = "+"
		if deltaCost < 0 {
			sign = ""
		}
		fmt.Fprintf(w, "| Custo total | $%.4f | $%.4f | %s%.1f%% |\n",
			before.Metrics.TotalCostUSD, after.Metrics.TotalCostUSD, sign, deltaCost)
	}

	fmt.Fprintf(w, "\n")

	// Interpretação
	fmt.Fprintf(w, "## Interpretação\n\n")

	improvements := []string{}
	regressions := []string{}

	if after.Metrics.PassAt1 > before.Metrics.PassAt1 {
		improvements = append(improvements, fmt.Sprintf("pass@1 melhorou %.1f pontos", (after.Metrics.PassAt1-before.Metrics.PassAt1)*100))
	} else if after.Metrics.PassAt1 < before.Metrics.PassAt1 {
		regressions = append(regressions, fmt.Sprintf("pass@1 piorou %.1f pontos", (before.Metrics.PassAt1-after.Metrics.PassAt1)*100))
	}

	if after.Metrics.AvgDuration < before.Metrics.AvgDuration {
		improvements = append(improvements, fmt.Sprintf("tempo médio reduziu %.1fs", before.Metrics.AvgDuration-after.Metrics.AvgDuration))
	} else if after.Metrics.AvgDuration > before.Metrics.AvgDuration*1.1 {
		regressions = append(regressions, fmt.Sprintf("tempo médio aumentou %.1fs", after.Metrics.AvgDuration-before.Metrics.AvgDuration))
	}

	if len(improvements) > 0 {
		fmt.Fprintf(w, "**Melhorias:**\n")
		for _, imp := range improvements {
			fmt.Fprintf(w, "- %s\n", imp)
		}
		fmt.Fprintf(w, "\n")
	}

	if len(regressions) > 0 {
		fmt.Fprintf(w, "**Regressões:**\n")
		for _, reg := range regressions {
			fmt.Fprintf(w, "- %s\n", reg)
		}
		fmt.Fprintf(w, "\n")
	}

	if len(improvements) == 0 && len(regressions) == 0 {
		fmt.Fprintf(w, "Sem diferenças significativas.\n")
	}

	return nil
}
