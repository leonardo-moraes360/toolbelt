package registry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/graph"
)

func TestMCPSchemaTypesAndGraphImpact(t *testing.T) {
	tools := map[string]*Tool{}
	for _, tool := range Default().List() {
		tools[tool.Name] = tool
	}

	impact, ok := tools["tb_graph_impact"]
	if !ok {
		t.Fatal("missing MCP tool tb_graph_impact")
	}
	if impact.InputSchema == nil || impact.InputSchema.Properties["file"].Type != "string" {
		t.Fatalf("tb_graph_impact schema inválido: %+v", impact.InputSchema)
	}
	if got := impact.InputSchema.Properties["depth"].Type; got != "integer" {
		t.Fatalf("tb_graph_impact.depth type=%q want integer", got)
	}

	cases := []struct {
		tool string
		prop string
		want string
	}{
		{"tb_graph_callers", "depth", "integer"},
		{"tb_graph_search", "limit", "integer"},
		{"tb_graph_deps", "transitive", "boolean"},
		{"tb_guard_check", "strict", "boolean"},
		{"tb_review", "reviewers", "integer"},
		{"tb_verify_run", "max_iter", "integer"},
		{"tb_verify_run", "threshold", "number"},
		{"tb_verify_run", "skip_unavailable", "boolean"},
		{"tb_stack", "fail_fast", "boolean"},
		{"tb_stack", "max_iter", "integer"},
		{"tb_stack", "ops", "array"},
		{"tb_run", "env", "array"},
		{"tb_run", "pass_env", "array"},
		{"tb_verify_run", "env", "array"},
		{"tb_verify_run", "pass_env", "array"},
		{"tb_stack", "env", "array"},
		{"tb_stack", "pass_env", "array"},
	}
	for _, tc := range cases {
		tool, ok := tools[tc.tool]
		if !ok {
			t.Errorf("missing tool %s", tc.tool)
			continue
		}
		prop, ok := tool.InputSchema.Properties[tc.prop]
		if !ok {
			t.Errorf("%s missing property %s", tc.tool, tc.prop)
			continue
		}
		if prop.Type != tc.want {
			t.Errorf("%s.%s type=%q want %q", tc.tool, tc.prop, prop.Type, tc.want)
		}
	}

	standards := tools["tb_review"].InputSchema.Properties["standards"]
	if standards.Type != "array" {
		t.Fatalf("tb_review.standards type=%q want array", standards.Type)
	}
	if standards.Items == nil || standards.Items.Type != "string" {
		t.Fatalf("tb_review.standards.items=%+v want string", standards.Items)
	}
}

func TestGraphImpactToolAcceptsTypedAndLegacyArgs(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(src, "a.go"), "package pkg\n\nfunc Helper() {}\n")
	writeFile(t, filepath.Join(src, "b.go"), "package pkg\n\nfunc Caller() { Helper() }\n")

	t.Setenv(graph.EnvEnable, "on")
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	res, err := Default().Call(context.Background(), "tb_graph_index", map[string]any{"path": "."})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_graph_index: %s", res.Content[0].Text)
	}

	res, err = Default().Call(context.Background(), "tb_graph_impact", map[string]any{
		"file":  "pkg/a.go",
		"depth": 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_graph_impact typed: %s", res.Content[0].Text)
	}
	var nodes []graph.Node
	if err := json.Unmarshal([]byte(res.Content[0].Text), &nodes); err != nil {
		t.Fatal(err)
	}
	if len(nodes) == 0 {
		t.Fatalf("esperava impacto tipado, got: %s", res.Content[0].Text)
	}

	res, err = Default().Call(context.Background(), "tb_graph_impact", map[string]any{
		"file":  "pkg/a.go",
		"depth": "2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_graph_impact legacy string depth: %s", res.Content[0].Text)
	}

	res, err = Default().Call(context.Background(), "tb_graph_callers", map[string]any{
		"function": "Helper",
		"depth":    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tb_graph_callers typed depth: %s", res.Content[0].Text)
	}
}

func TestTypedArgHelpersLegacyCompat(t *testing.T) {
	args := map[string]any{
		"depth":      float64(3),
		"limit":      "7",
		"strict":     true,
		"legacy":     "true",
		"reviewers":  float64(2),
		"standards":  []any{"scope", "completeness"},
		"csv":        "a, b, c",
		"threshold":  0.8,
		"thresholdS": "0.7",
	}
	if got := IntArg(args, "depth", 1); got != 3 {
		t.Fatalf("IntArg float64=%d", got)
	}
	if got := IntArg(args, "limit", 20); got != 7 {
		t.Fatalf("IntArg string=%d", got)
	}
	if got := IntArg(args, "reviewers", 3); got != 2 {
		t.Fatalf("IntArg reviewers=%d", got)
	}
	if !BoolArg(args, "strict", false) {
		t.Fatal("BoolArg true")
	}
	if !BoolArg(args, "legacy", false) {
		t.Fatal("BoolArg legacy string")
	}
	if BoolArg(args, "missing", true) != true {
		t.Fatal("BoolArg default")
	}
	if got := FloatArg(args, "threshold", 0.95); got != 0.8 {
		t.Fatalf("FloatArg=%v", got)
	}
	if got := FloatArg(args, "thresholdS", 0.95); got != 0.7 {
		t.Fatalf("FloatArg string=%v", got)
	}
	if got := StringSliceArg(args, "standards", nil); len(got) != 2 || got[0] != "scope" {
		t.Fatalf("StringSliceArg array=%v", got)
	}
	if got := StringSliceArg(args, "csv", nil); len(got) != 3 || got[1] != "b" {
		t.Fatalf("StringSliceArg csv=%v", got)
	}
}
