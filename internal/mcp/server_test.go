package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/registry"
)

func TestInitialize(t *testing.T) {
	reg := registry.New()
	reg.Register(&registry.Tool{
		Name:    "tb_echo",
		Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil },
	})
	srv := NewServer(reg)

	in := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	out := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_ = srv.Run(ctx, strings.NewReader(in), out)

	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("erro: %+v", resp.Error)
	}
}

func TestToolsCall(t *testing.T) {
	reg := registry.New()
	reg.Register(&registry.Tool{
		Name: "tb_echo",
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return args["message"], nil
		},
	})
	srv := NewServer(reg)

	in := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"tb_echo","arguments":{"message":"ok"}}}` + "\n"
	out := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_ = srv.Run(ctx, strings.NewReader(in), out)

	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("erro: %+v", resp.Error)
	}
}

func TestDefaultToolsListHasParity(t *testing.T) {
	srv := NewServer(registry.Default())
	in := `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}` + "\n"
	out := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_ = srv.Run(ctx, strings.NewReader(in), out)

	raw := out.String()
	for _, name := range []string{
		"tb_mem_list", "tb_mem_show", "tb_mem_edit", "tb_mem_retire", "tb_mem_search",
		"tb_verify_run", "tb_review", "tb_drift", "tb_guard_check", "tb_stack",
		"tb_graph_impact",
	} {
		if !strings.Contains(raw, name) {
			t.Errorf("tools/list sem %s: %s", name, raw)
		}
	}
}

func TestToolsListSchemaTypes(t *testing.T) {
	srv := NewServer(registry.Default())
	in := `{"jsonrpc":"2.0","id":4,"method":"tools/list","params":{}}` + "\n"
	out := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_ = srv.Run(ctx, strings.NewReader(in), out)

	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("erro: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result tipo %T", resp.Result)
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools tipo %T", result["tools"])
	}
	byName := map[string]map[string]any{}
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := tool["name"].(string)
		byName[name] = tool
	}
	impact, ok := byName["tb_graph_impact"]
	if !ok {
		t.Fatal("tools/list sem tb_graph_impact")
	}
	want := map[string]map[string]string{
		"tb_graph_impact":  {"depth": "integer"},
		"tb_graph_callers": {"depth": "integer"},
		"tb_graph_search":  {"limit": "integer"},
		"tb_graph_deps":    {"transitive": "boolean"},
		"tb_graph_explain": {"limit": "integer"},
		"tb_graph_path":    {"max_depth": "integer"},
		"tb_guard_check":   {"strict": "boolean"},
		"tb_review":        {"reviewers": "integer", "standards": "array"},
	}
	for toolName, props := range want {
		tool := byName[toolName]
		if tool == nil {
			t.Errorf("missing %s", toolName)
			continue
		}
		schema, _ := tool["inputSchema"].(map[string]any)
		properties, _ := schema["properties"].(map[string]any)
		for prop, typ := range props {
			p, _ := properties[prop].(map[string]any)
			if p["type"] != typ {
				t.Errorf("%s.%s type=%v want %s", toolName, prop, p["type"], typ)
			}
		}
	}
	_ = impact
}
