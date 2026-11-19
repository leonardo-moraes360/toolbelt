package bench

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

func WriteMarkdownReport(w io.Writer, summary *Summary, results []Result) error {
	writeReportHeader(w, summary, results)
	writeReportSummaryTable(w, summary)
	return writeReportTaskTable(w, results)
}

func writeReportHeader(w io.Writer, summary *Summary, results []Result) {
	fmt.Fprintf(w, "# Benchmark Report\n\n")
	fmt.Fprintf(w, "**ID**: %s\n", summary.BenchmarkID)
	fmt.Fprintf(w, "**Agent**: %s\n", summary.AgentID)
	if len(results) > 0 && results[0].ConfigLabel != "" {
		fmt.Fprintf(w, "**A/B variant**: %s\n", results[0].ConfigLabel)
	}
	if len(results) > 0 && len(results[0].Flags) > 0 {
		fmt.Fprintf(w, "**Flags**: %s\n", formatFlags(results[0].Flags))
	}
	fmt.Fprintf(w, "**Start**: %s\n", summary.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "**End**: %s\n", summary.EndTime.Format("2006-01-02 15:04:05"))
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
	fmt.Fprintf(w, "## Summary\n\n")
	fmt.Fprintf(w, "| Metric | Value |\n")
	fmt.Fprintf(w, "|---------|-------|\n")
	fmt.Fprintf(w, "| Tasks | %d |\n", summary.TotalTasks)
	fmt.Fprintf(w, "| Repetitions | %d |\n", summary.TotalRepetitions)
	fmt.Fprintf(w, "| **success (pass@1)** | **%.1f%%** |\n", summary.Metrics.PassAt1*100)

	for _, k := range []int{3, 5, 10} {
		if v, ok := summary.Metrics.PassAtK[k]; ok {
			fmt.Fprintf(w, "| pass@%d | %.1f%% |\n", k, v*100)
		}
	}

	fmt.Fprintf(w, "| Avg time | %.1fs |\n", summary.Metrics.AvgDuration)

	if summary.Metrics.TotalTokens > 0 {
		fmt.Fprintf(w, "| Total tokens | %d |\n", summary.Metrics.TotalTokens)
		fmt.Fprintf(w, "| Avg tokens | %.0f |\n", summary.Metrics.AvgTokens)
	}
	if summary.Metrics.TotalCostUSD > 0 {
		fmt.Fprintf(w, "| Total cost | $%.4f |\n", summary.Metrics.TotalCostUSD)
	}
	if summary.Metrics.AvgToolCalls > 0 {
		fmt.Fprintf(w, "| Avg tool calls | %.1f |\n", summary.Metrics.AvgToolCalls)
	}
	fmt.Fprintf(w, "| Avg retries | %.1f |\n", summary.Metrics.AvgRetries)
	fmt.Fprintf(w, "\n")
}

func writeReportTaskTable(w io.Writer, results []Result) error {
	fmt.Fprintf(w, "## Results by Task\n\n")
	fmt.Fprintf(w, "| Task | Rep | Passed | Time | Tokens | Error |\n")
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

func WriteComparisonReport(w io.Writer, before, after *Summary) error {
	labelA := before.AgentID
	labelB := after.AgentID
	if before.BenchmarkID != "" {
		labelA = before.BenchmarkID
	}
	if after.BenchmarkID != "" {
		labelB = after.BenchmarkID
	}

	fmt.Fprintf(w, "# Benchmark A/B Comparison\n\n")
	fmt.Fprintf(w, "**Variant A**: %s (agent %s)\n", labelA, before.AgentID)
	fmt.Fprintf(w, "**Variant B**: %s (agent %s)\n\n", labelB, after.AgentID)
	fmt.Fprintf(w, "| Metric | A (%s) | B (%s) | Δ |\n", labelA, labelB)
	fmt.Fprintf(w, "|---------|-------|--------|---|\n")
	writeComparisonMetrics(w, before, after)
	fmt.Fprintf(w, "\n")

	cmp := CompareAB(before, after)
	fmt.Fprintf(w, "## Interpretation\n\n")
	improvements, regressions := interpretAB(cmp)
	writeInterpretation(w, improvements, regressions)
	return nil
}

func writeComparisonMetrics(w io.Writer, before, after *Summary) {
	writeDeltaRow(w, "success (pass@1)",
		fmt.Sprintf("%.1f%%", before.Metrics.PassAt1*100),
		fmt.Sprintf("%.1f%%", after.Metrics.PassAt1*100),
		(after.Metrics.PassAt1-before.Metrics.PassAt1)*100, "%")

	writeDeltaRow(w, "avg time",
		fmt.Sprintf("%.1fs", before.Metrics.AvgDuration),
		fmt.Sprintf("%.1fs", after.Metrics.AvgDuration),
		after.Metrics.AvgDuration-before.Metrics.AvgDuration, "s")

	if before.Metrics.AvgTokens > 0 || after.Metrics.AvgTokens > 0 {
		deltaTok := 0.0
		if before.Metrics.AvgTokens > 0 {
			deltaTok = (after.Metrics.AvgTokens - before.Metrics.AvgTokens) / before.Metrics.AvgTokens * 100
		}
		writeDeltaRow(w, "avg tokens",
			fmt.Sprintf("%.0f", before.Metrics.AvgTokens),
			fmt.Sprintf("%.0f", after.Metrics.AvgTokens),
			deltaTok, "%")
	}

	if before.Metrics.TotalCostUSD > 0 || after.Metrics.TotalCostUSD > 0 {
		deltaCost := 0.0
		if before.Metrics.TotalCostUSD > 0 {
			deltaCost = (after.Metrics.TotalCostUSD - before.Metrics.TotalCostUSD) / before.Metrics.TotalCostUSD * 100
		}
		fmt.Fprintf(w, "| total cost | $%.4f | $%.4f | %s%.1f%% |\n",
			before.Metrics.TotalCostUSD, after.Metrics.TotalCostUSD, signed(deltaCost), deltaCost)
	}

	if before.Metrics.AvgToolCalls > 0 || after.Metrics.AvgToolCalls > 0 {
		writeDeltaRow(w, "avg tool calls",
			fmt.Sprintf("%.1f", before.Metrics.AvgToolCalls),
			fmt.Sprintf("%.1f", after.Metrics.AvgToolCalls),
			after.Metrics.AvgToolCalls-before.Metrics.AvgToolCalls, "")
	}
}

func writeInterpretation(w io.Writer, improvements, regressions []string) {
	if len(improvements) > 0 {
		fmt.Fprintf(w, "**Improvements (B vs A):**\n")
		for _, imp := range improvements {
			fmt.Fprintf(w, "- %s\n", imp)
		}
		fmt.Fprintf(w, "\n")
	}
	if len(regressions) > 0 {
		fmt.Fprintf(w, "**Regressions (B vs A):**\n")
		for _, reg := range regressions {
			fmt.Fprintf(w, "- %s\n", reg)
		}
		fmt.Fprintf(w, "\n")
	}
	if len(improvements) == 0 && len(regressions) == 0 {
		fmt.Fprintf(w, "No significant differences in success, time or tokens.\n")
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
		improvements = append(improvements, fmt.Sprintf("success (pass@1) +%.1f points", cmp.DeltaPassAt1*100))
	} else if cmp.DeltaPassAt1 < -0.001 {
		regressions = append(regressions, fmt.Sprintf("success (pass@1) %.1f points", cmp.DeltaPassAt1*100))
	}
	if cmp.DeltaDuration < -0.05 {
		improvements = append(improvements, fmt.Sprintf("avg time %.1fs", cmp.DeltaDuration))
	} else if cmp.DeltaDuration > 0.05 {
		regressions = append(regressions, fmt.Sprintf("avg time +%.1fs", cmp.DeltaDuration))
	}
	if cmp.AvgTokensA > 0 {
		if cmp.DeltaTokensPct < -1 {
			improvements = append(improvements, fmt.Sprintf("avg tokens %.1f%%", cmp.DeltaTokensPct))
		} else if cmp.DeltaTokensPct > 1 {
			regressions = append(regressions, fmt.Sprintf("avg tokens +%.1f%%", cmp.DeltaTokensPct))
		}
	}
	return improvements, regressions
}
