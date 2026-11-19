package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/graph"
)

// graphQueries are the MCP graph tools annotated readOnlyHint, with args
// that resolve against graphRepo.
var graphQueries = map[string]map[string]any{
	"tb_graph_search":  {"query": "Foo"},
	"tb_graph_callers": {"function": "Foo"},
	"tb_graph_deps":    {"file": "a.go"},
	"tb_graph_impact":  {"file": "a.go"},
	"tb_graph_explain": {"node": "Foo"},
	"tb_graph_path":    {"from": "Bar", "to": "Foo"},
}

// graphRepo creates a repo with .toolbelt/ and chdirs into it (openGraphStore
// resolves the index from the working directory).
func graphRepo(t *testing.T) string {
	t.Helper()
	t.Setenv(graph.EnvEnable, "on")
	useConfig(t, "graph:\n  index_on_change: true\n")
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n\nfunc Foo() {}\n\nfunc Bar() { Foo() }\n")
	t.Chdir(dir)
	return dir
}

func callGraph(t *testing.T, tool string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	res, err := Default().Call(context.Background(), tool, args)
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	if res.IsError {
		return nil, true
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
		t.Fatalf("%s: %v (%s)", tool, err, res.Content[0].Text)
	}
	return payload, false
}

// Even with graph.index_on_change on, MCP queries must not write the index:
// they are annotated readOnlyHint (M8ven review, finding 3).
func TestGraphQueriesNeverWriteIndex(t *testing.T) {
	dir := graphRepo(t)
	if _, isErr := callGraph(t, "tb_graph_index", map[string]any{}); isErr {
		t.Fatal("tb_graph_index failed")
	}
	indexPath := graph.DefaultPath(filepath.Join(dir, ".toolbelt"))
	before, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}

	// Change the tree so the fingerprint diverges.
	writeFile(t, filepath.Join(dir, "b.go"), "package a\n\nfunc Baz() { Foo() }\n")

	for tool, args := range graphQueries {
		payload, isErr := callGraph(t, tool, args)
		after, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("%s rewrote the index", tool)
		}
		if isErr {
			continue // e.g. no path: the point is that nothing was written
		}
		if payload["stale"] != true || payload["hint"] == nil {
			t.Errorf("%s: want stale:true with a hint, got %v", tool, payload)
		}
	}

	// tb_graph_index is the one tool that refreshes; queries are then fresh.
	if _, isErr := callGraph(t, "tb_graph_index", map[string]any{}); isErr {
		t.Fatal("tb_graph_index failed")
	}
	payload, isErr := callGraph(t, "tb_graph_search", map[string]any{"query": "Baz"})
	if isErr || payload["stale"] != false || payload["hint"] != nil {
		t.Fatalf("after reindex want stale:false and no hint, got %v (isError=%v)", payload, isErr)
	}
}

func TestGraphQueryWithoutIndexCreatesNothing(t *testing.T) {
	dir := graphRepo(t)
	payload, isErr := callGraph(t, "tb_graph_search", map[string]any{"query": "Foo"})
	if isErr || payload["stale"] != true {
		t.Fatalf("want stale:true without an index, got %v (isError=%v)", payload, isErr)
	}
	if _, err := os.Stat(graph.DefaultPath(filepath.Join(dir, ".toolbelt"))); !os.IsNotExist(err) {
		t.Fatalf("query created the index file (stat err=%v)", err)
	}
}
