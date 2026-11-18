package bench

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteReadJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "results.jsonl")

	results := []Result{
		{
			TaskID:     "task-1",
			AgentID:    "fake",
			Repetition: 1,
			Passed:     true,
			Duration:   1.5,
			StartTime:  time.Now(),
			EndTime:    time.Now(),
		},
		{
			TaskID:     "task-2",
			AgentID:    "fake",
			Repetition: 1,
			Passed:     false,
			Duration:   2.0,
			Error:      "test failed",
			StartTime:  time.Now(),
			EndTime:    time.Now(),
		},
	}

	if err := WriteJSONL(results, filePath); err != nil {
		t.Fatalf("WriteJSONL failed: %v", err)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("output file not created")
	}

	readResults, err := ReadJSONL(filePath)
	if err != nil {
		t.Fatalf("ReadJSONL failed: %v", err)
	}

	if len(readResults) != len(results) {
		t.Errorf("len(readResults) = %d, want %d", len(readResults), len(results))
	}

	if readResults[0].TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", readResults[0].TaskID, "task-1")
	}
}

func TestCalculateSummary(t *testing.T) {
	results := []Result{
		{TaskID: "task-1", Passed: true, Duration: 1.0, Tokens: &TokenInfo{Total: 100, CostUSD: 0.001}},
		{TaskID: "task-1", Passed: true, Duration: 1.5, Tokens: &TokenInfo{Total: 120, CostUSD: 0.0012}},
		{TaskID: "task-1", Passed: false, Duration: 2.0, Tokens: &TokenInfo{Total: 150, CostUSD: 0.0015}},
		{TaskID: "task-2", Passed: false, Duration: 3.0, Tokens: &TokenInfo{Total: 200, CostUSD: 0.002}},
		{TaskID: "task-2", Passed: false, Duration: 2.5, Tokens: &TokenInfo{Total: 180, CostUSD: 0.0018}},
		{TaskID: "task-2", Passed: true, Duration: 2.0, Tokens: &TokenInfo{Total: 160, CostUSD: 0.0016}},
	}

	summary := CalculateSummary("test-bench", "test-agent", results)

	if summary.TotalTasks != 2 {
		t.Errorf("TotalTasks = %d, want %d", summary.TotalTasks, 2)
	}

	if summary.TotalRepetitions != 3 {
		t.Errorf("TotalRepetitions = %d, want %d", summary.TotalRepetitions, 3)
	}

	// task-1: 2/3 passed, task-2: 1/3 passed
	// pass@1 = mean of per-task rates = (0.666... + 0.333...) / 2 ≈ 0.5
	expectedPassAt1 := 0.5
	if summary.Metrics.PassAt1 < expectedPassAt1-0.01 || summary.Metrics.PassAt1 > expectedPassAt1+0.01 {
		t.Errorf("PassAt1 = %f, want ~%f", summary.Metrics.PassAt1, expectedPassAt1)
	}

	// Total tokens = 100+120+150+200+180+160 = 910
	if summary.Metrics.TotalTokens != 910 {
		t.Errorf("TotalTokens = %d, want %d", summary.Metrics.TotalTokens, 910)
	}

	// Total cost = 0.001+0.0012+0.0015+0.002+0.0018+0.0016 = 0.0091
	expectedCost := 0.0091
	if summary.Metrics.TotalCostUSD < expectedCost-0.0001 || summary.Metrics.TotalCostUSD > expectedCost+0.0001 {
		t.Errorf("TotalCostUSD = %f, want %f", summary.Metrics.TotalCostUSD, expectedCost)
	}
}

func TestCalculatePassAtK(t *testing.T) {
	tests := []struct {
		name    string
		passed  int
		total   int
		k       int
		wantMin float64
		wantMax float64
	}{
		{
			name:    "all passed k=1",
			passed:  3,
			total:   3,
			k:       1,
			wantMin: 1.0,
			wantMax: 1.0,
		},
		{
			name:    "none passed k=1",
			passed:  0,
			total:   3,
			k:       1,
			wantMin: 0.0,
			wantMax: 0.0,
		},
		{
			name:    "1 of 3 passed k=3",
			passed:  1,
			total:   3,
			k:       3,
			wantMin: 1.0,
			wantMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculatePassAtK(tt.passed, tt.total, tt.k)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CalculatePassAtK(%d, %d, %d) = %f, want [%f, %f]",
					tt.passed, tt.total, tt.k, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}
