package bench

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// WriteMarkdownReport gera um relatório Markdown dos resultados.
func WriteMarkdownReport(w io.Writer, summary *Summary, results []Result) error {
	writeReportHeader(w, summary, results)
	writeReportSummaryTable(w, summary)
	return writeReportTaskTable(w, results)
}

func writeReportHeader(w io.Writer, summary *Summary, results []Result) {
	fmt.Fprintf(w, "# Relatório de Benchmark\n\n")
	fmt.Fprintf(w, "**ID**: %s\n", summary.BenchmarkID)
	fmt.Fprintf(w, "**Agente**: %s\n", summary.AgentID)
	if len(results) > 0 && results[0].ConfigLabel != "" {
		fmt.Fprintf(w, "**Variante A/B**: %s\n", results[0].ConfigLabel)
	}
	if len(results) > 0 && len(results[0].Flags) > 0 {
		fmt.Fprintf(w, "**Flags**: %s\n", formatFlags(results[0].Flags))
	}
	fmt.Fprintf(w, "**Início**: %s\n", summary.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "**Fim**: %s\n", summary.EndTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "\n---\n\n")
}

func formatFlags(flags map[string]string) string {
	parts := make([]string, 0, len(flags))
	for k, v := range flags {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func writeReportSummaryTable(w io.Writer, summary *Summary) {
	fmt.Fprintf(w, "## Resumo\n\n")
	fmt.Fprintf(w, "| Métrica | Valor |\n")
	fmt.Fprintf(w, "|---------|-------|\n")
	fmt.Fprintf(w, "| Tarefas | %d |\n", summary.TotalTasks)
	fmt.Fprintf(w, "| Repetições | %d |\n", summary.TotalRepetitions)
	fmt.Fprintf(w, "| **sucesso (pass@1)** | **%.1f%%** |\n", summary.Metrics.PassAt1*100)

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
}

func writeReportTaskTable(w io.Writer, results []Result) error {
	fmt.Fprintf(w, "## Resultados por Tarefa\n\n")
	fmt.Fprintf(w, "| Tarefa | Rep | Passou | Tempo | Tokens | Erro |\n")
	fmt.Fprintf(w, "|--------|-----|--------|-------|--------|------|\n")

	sorted := make([]Result, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].TaskID != sorted[j].TaskID {
			return sorted[i].TaskID < sorted[j].TaskID
		}
		return sorted[i].Repetition < sorted[j].Repetition
	})

	for i := range sorted {
		r := &sorted[i]
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

// WriteComparisonReport gera um relatório comparando dois benchmarks (A/B).
func WriteComparisonReport(w io.Writer, before, after *Summary) error {
	labelA := before.AgentID
	labelB := after.AgentID
	if before.BenchmarkID != "" {
		labelA = before.BenchmarkID
	}
	if after.BenchmarkID != "" {
		labelB = after.BenchmarkID
	}

	fmt.Fprintf(w, "# Comparação A/B de Benchmarks\n\n")
	fmt.Fprintf(w, "**Variante A**: %s (agente %s)\n", labelA, before.AgentID)
	fmt.Fprintf(w, "**Variante B**: %s (agente %s)\n\n", labelB, after.AgentID)
	fmt.Fprintf(w, "| Métrica | A (%s) | B (%s) | Δ |\n", labelA, labelB)
	fmt.Fprintf(w, "|---------|-------|--------|---|\n")
	writeComparisonMetrics(w, before, after)
	fmt.Fprintf(w, "\n")

	cmp := CompareAB(before, after)
	fmt.Fprintf(w, "## Interpretação\n\n")
	improvements, regressions := interpretAB(cmp)
	writeInterpretation(w, improvements, regressions)
	return nil
}

func writeComparisonMetrics(w io.Writer, before, after *Summary) {
	writeDeltaRow(w, "sucesso (pass@1)",
		fmt.Sprintf("%.1f%%", before.Metrics.PassAt1*100),
		fmt.Sprintf("%.1f%%", after.Metrics.PassAt1*100),
		(after.Metrics.PassAt1-before.Metrics.PassAt1)*100, "%")

	writeDeltaRow(w, "tempo médio",
		fmt.Sprintf("%.1fs", before.Metrics.AvgDuration),
		fmt.Sprintf("%.1fs", after.Metrics.AvgDuration),
		after.Metrics.AvgDuration-before.Metrics.AvgDuration, "s")

	if before.Metrics.AvgTokens > 0 || after.Metrics.AvgTokens > 0 {
		deltaTok := 0.0
		if before.Metrics.AvgTokens > 0 {
			deltaTok = (after.Metrics.AvgTokens - before.Metrics.AvgTokens) / before.Metrics.AvgTokens * 100
		}
		writeDeltaRow(w, "tokens médio",
			fmt.Sprintf("%.0f", before.Metrics.AvgTokens),
			fmt.Sprintf("%.0f", after.Metrics.AvgTokens),
			deltaTok, "%")
	}

	if before.Metrics.TotalCostUSD > 0 || after.Metrics.TotalCostUSD > 0 {
		deltaCost := 0.0
		if before.Metrics.TotalCostUSD > 0 {
			deltaCost = (after.Metrics.TotalCostUSD - before.Metrics.TotalCostUSD) / before.Metrics.TotalCostUSD * 100
		}
		fmt.Fprintf(w, "| custo total | $%.4f | $%.4f | %s%.1f%% |\n",
			before.Metrics.TotalCostUSD, after.Metrics.TotalCostUSD, signed(deltaCost), deltaCost)
	}

	if before.Metrics.AvgToolCalls > 0 || after.Metrics.AvgToolCalls > 0 {
		writeDeltaRow(w, "tool calls médio",
			fmt.Sprintf("%.1f", before.Metrics.AvgToolCalls),
			fmt.Sprintf("%.1f", after.Metrics.AvgToolCalls),
			after.Metrics.AvgToolCalls-before.Metrics.AvgToolCalls, "")
	}
}

func writeInterpretation(w io.Writer, improvements, regressions []string) {
	if len(improvements) > 0 {
		fmt.Fprintf(w, "**Melhorias (B vs A):**\n")
		for _, imp := range improvements {
			fmt.Fprintf(w, "- %s\n", imp)
		}
		fmt.Fprintf(w, "\n")
	}
	if len(regressions) > 0 {
		fmt.Fprintf(w, "**Regressões (B vs A):**\n")
		for _, reg := range regressions {
			fmt.Fprintf(w, "- %s\n", reg)
		}
		fmt.Fprintf(w, "\n")
	}
	if len(improvements) == 0 && len(regressions) == 0 {
		fmt.Fprintf(w, "Sem diferenças significativas em sucesso, tempo ou tokens.\n")
	}
}

func writeDeltaRow(w io.Writer, name, a, b string, delta float64, unit string) {
	fmt.Fprintf(w, "| %s | %s | %s | %s%.1f%s |\n", name, a, b, signed(delta), delta, unit)
}

func signed(v float64) string {
	if v >= 0 {
		return "+"
	}
	return ""
}

func interpretAB(cmp *ABComparison) (improvements, regressions []string) {
	if cmp == nil {
		return nil, nil
	}
	if cmp.DeltaPassAt1 > 0.001 {
		improvements = append(improvements, fmt.Sprintf("sucesso (pass@1) +%.1f pontos", cmp.DeltaPassAt1*100))
	} else if cmp.DeltaPassAt1 < -0.001 {
		regressions = append(regressions, fmt.Sprintf("sucesso (pass@1) %.1f pontos", cmp.DeltaPassAt1*100))
	}
	if cmp.DeltaDuration < -0.05 {
		improvements = append(improvements, fmt.Sprintf("tempo médio %.1fs", cmp.DeltaDuration))
	} else if cmp.DeltaDuration > 0.05 {
		regressions = append(regressions, fmt.Sprintf("tempo médio +%.1fs", cmp.DeltaDuration))
	}
	if cmp.AvgTokensA > 0 {
		if cmp.DeltaTokensPct < -1 {
			improvements = append(improvements, fmt.Sprintf("tokens médio %.1f%%", cmp.DeltaTokensPct))
		} else if cmp.DeltaTokensPct > 1 {
			regressions = append(regressions, fmt.Sprintf("tokens médio +%.1f%%", cmp.DeltaTokensPct))
		}
	}
	return improvements, regressions
}
