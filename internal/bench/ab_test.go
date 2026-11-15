package bench

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadABVariant(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph-off.yaml")
	content := []byte("label: graph-off\nflags:\n  graph: \"off\"\nenv:\n  TB_GRAPH: \"0\"\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := LoadABVariant(path)
	if err != nil {
		t.Fatalf("LoadABVariant: %v", err)
	}
	if v.Label != "graph-off" {
		t.Errorf("Label = %q", v.Label)
	}
	if v.Flags["graph"] != "off" {
		t.Errorf("Flags[graph] = %q", v.Flags["graph"])
	}
}

func TestApplyVariantAndTagResults(t *testing.T) {
	agent := AgentConfig{ID: "fake", Env: map[string]string{}}
	variant := &ABVariant{
		Label: "graph-on",
		Flags: map[string]string{"graph": "on"},
		Env:   map[string]string{"EXTRA": "1"},
	}
	ApplyVariant(&agent, variant)
	if agent.Env["TB_AB_LABEL"] != "graph-on" {
		t.Errorf("TB_AB_LABEL = %q", agent.Env["TB_AB_LABEL"])
	}
	if agent.Env["TB_FLAG_GRAPH"] != "on" {
		t.Errorf("TB_FLAG_GRAPH = %q", agent.Env["TB_FLAG_GRAPH"])
	}
	results := []Result{{TaskID: "t1"}}
	TagResults(results, variant)
	if results[0].ConfigLabel != "graph-on" {
		t.Errorf("ConfigLabel = %q", results[0].ConfigLabel)
	}
	if results[0].Flags["graph"] != "on" {
		t.Errorf("Flags = %#v", results[0].Flags)
	}
}

func TestCompareAB(t *testing.T) {
	a := &Summary{AgentID: "a", Metrics: Metrics{PassAt1: 0.5, AvgDuration: 2.0, AvgTokens: 1000}}
	b := &Summary{AgentID: "b", Metrics: Metrics{PassAt1: 0.7, AvgDuration: 1.5, AvgTokens: 800}}
	cmp := CompareAB(a, b)
	if cmp.DeltaPassAt1 < 0.19 || cmp.DeltaPassAt1 > 0.21 {
		t.Errorf("DeltaPassAt1 = %f", cmp.DeltaPassAt1)
	}
	if cmp.DeltaTokensPct > -19 || cmp.DeltaTokensPct < -21 {
		t.Errorf("DeltaTokensPct = %f", cmp.DeltaTokensPct)
	}
}

func TestWriteComparisonReportAB(t *testing.T) {
	before := &Summary{
		BenchmarkID: "graph-off",
		AgentID:     "fake",
		Metrics:     Metrics{PassAt1: 0.5, AvgDuration: 2, AvgTokens: 1000, TotalTokens: 1000},
	}
	after := &Summary{
		BenchmarkID: "graph-on",
		AgentID:     "fake",
		Metrics:     Metrics{PassAt1: 0.8, AvgDuration: 1.5, AvgTokens: 700, TotalTokens: 700},
	}
	var buf bytes.Buffer
	if err := WriteComparisonReport(&buf, before, after); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Comparação A/B")) {
		t.Errorf("relatório sem título A/B: %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte("tokens médio")) {
		t.Errorf("relatório sem tokens: %s", out)
	}
	if !bytes.Contains(buf.Bytes(), []byte("sucesso (pass@1)")) {
		t.Errorf("relatório sem sucesso: %s", out)
	}
}
