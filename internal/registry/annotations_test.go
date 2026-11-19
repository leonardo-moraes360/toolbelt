package registry

import (
	"context"
	"sort"
	"testing"
)

func TestAllToolsAnnotated(t *testing.T) {
	tools := Default().List()
	if len(tools) == 0 {
		t.Fatal("default registry is empty")
	}
	registered := map[string]bool{}
	for _, tool := range tools {
		registered[tool.Name] = true
		if tool.Annotations == nil {
			t.Errorf("%s: no annotations (add it to toolAnnotations in annotations.go)", tool.Name)
			continue
		}
		a := tool.Annotations
		if a.ReadOnlyHint && (a.DestructiveHint || a.OpenWorldHint) {
			t.Errorf("%s: readOnlyHint contradicts destructive/openWorld: %+v", tool.Name, *a)
		}
	}
	for name := range toolAnnotations {
		if !registered[name] {
			t.Errorf("toolAnnotations has %q, but no such tool is registered", name)
		}
	}
}

func TestToolAnnotationsMatchEffects(t *testing.T) {
	cases := map[string]ToolAnnotations{
		"tb_run":          {DestructiveHint: true, OpenWorldHint: true},
		"tb_stack":        {DestructiveHint: true, OpenWorldHint: true},
		"tb_ticket_list":  {ReadOnlyHint: true, IdempotentHint: true},
		"tb_graph_search": {ReadOnlyHint: true, IdempotentHint: true}, // never reindexes over MCP
		"tb_bus_recv":     {OpenWorldHint: true},                      // advances the cursor
		"tb_merge":        {DestructiveHint: true},
	}
	for name, want := range cases {
		tool, ok := Default().Get(name)
		if !ok {
			t.Fatalf("%s not registered", name)
		}
		if tool.Annotations == nil || *tool.Annotations != want {
			t.Errorf("%s: annotations = %+v, want %+v", name, tool.Annotations, want)
		}
	}
}

func TestRegisterKeepsExplicitAnnotations(t *testing.T) {
	r := New()
	own := &ToolAnnotations{ReadOnlyHint: true}
	r.Register(&Tool{Name: "tb_run", Annotations: own, Handler: func(context.Context, map[string]any) (any, error) { return nil, nil }})
	got, _ := r.Get("tb_run")
	if got.Annotations != own {
		t.Fatalf("Register replaced explicit annotations: %+v", got.Annotations)
	}
}

func TestListSortedByName(t *testing.T) {
	tools := Default().List()
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("List is not sorted: %v", names)
	}
}
