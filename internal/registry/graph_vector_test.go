package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/embed"
	"github.com/leonardo-moraes360/toolbelt/internal/envelope"
	"github.com/leonardo-moraes360/toolbelt/internal/vecsearch"
)

var updateGolden = flag.Bool("update-graph-golden", false, "rewrite testdata/graph_*.golden")

// callGraphText returns the exact text an MCP client receives.
func callGraphText(t *testing.T, tool string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := Default().Call(context.Background(), tool, args)
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	if res.IsError {
		return errorBodyText(t, res.Content[0].Text), true
	}
	return res.Content[0].Text, false
}

// errorBodyText checks a tool error is the error/1.0 body and returns its
// message, which is what these tests assert on.
func errorBodyText(t *testing.T, text string) string {
	t.Helper()
	var body envelope.ErrorBody
	if err := json.Unmarshal([]byte(text), &body); err != nil || body.V != envelope.Error || body.ExitCode == 0 {
		t.Fatalf("tool error is not an error/1.0 body: %v %q", err, text)
	}
	return body.Error
}

// testdataDir is resolved before graphRepo chdirs into the temp repo.
func testdataDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "testdata")
}

// checkGolden compares got with testdata/<name>, with the temp repo path
// replaced by $REPO so the file is stable across runs.
func checkGolden(t *testing.T, testdata, dir, name, got string) {
	t.Helper()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		got = strings.ReplaceAll(got, resolved, "$REPO")
	}
	got = strings.ReplaceAll(got, dir, "$REPO")
	path := filepath.Join(testdata, name)
	if *updateGolden {
		if err := os.MkdirAll(testdata, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("%s changed:\n%s\n--- want ---\n%s", name, got, want)
	}
}

// Acceptance 16: without mode/vectors the tools answer byte for byte as
// before V2 (the goldens were captured on the V1 handlers).
func TestGraphMCPDefaultsUnchanged(t *testing.T) {
	testdata := testdataDir(t)
	dir := graphRepo(t)
	idx, isErr := callGraphText(t, "tb_graph_index", map[string]any{})
	if isErr {
		t.Fatalf("tb_graph_index: %s", idx)
	}
	checkGolden(t, testdata, dir, "graph_index.golden", idx)
	search, isErr := callGraphText(t, "tb_graph_search", map[string]any{"query": "Foo"})
	if isErr {
		t.Fatalf("tb_graph_search: %s", search)
	}
	checkGolden(t, testdata, dir, "graph_search.golden", search)
}

func TestGraphSearchInvalidMode(t *testing.T) {
	graphRepo(t)
	text, isErr := callGraphText(t, "tb_graph_search", map[string]any{"query": "Foo", "mode": "fuzzy"})
	if !isErr || !strings.Contains(text, `invalid mode "fuzzy"`) {
		t.Fatalf("want invalid mode error, got %v %s", isErr, text)
	}
}

// graphRepo's user config has no graph.vector: both vector entry points
// refuse with the CLI's reason, and nothing is written.
func TestGraphMCPVectorsDisabled(t *testing.T) {
	dir := graphRepo(t)
	text, isErr := callGraphText(t, "tb_graph_index", map[string]any{"vectors": true})
	if !isErr || !strings.HasPrefix(text, "vectors_disabled") {
		t.Fatalf("index vectors: want vectors_disabled, got %v %s", isErr, text)
	}
	for _, mode := range []string{"semantic", "hybrid"} {
		text, isErr := callGraphText(t, "tb_graph_search", map[string]any{"query": "Foo", "mode": mode})
		if !isErr || !strings.HasPrefix(text, "vectors_disabled") {
			t.Fatalf("%s: want vectors_disabled, got %v %s", mode, isErr, text)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".toolbelt", "graph.vec")); !os.IsNotExist(err) {
		t.Fatalf("graph.vec written while disabled: %v", err)
	}
}

// A repository that turns vectors on is ignored, and the error says so.
func TestGraphMCPVectorsRepoConfigUntrusted(t *testing.T) {
	dir := graphRepo(t)
	writeFile(t, filepath.Join(dir, ".toolbelt", "config.yaml"), "graph:\n  vector:\n    enabled: true\n")
	text, isErr := callGraphText(t, "tb_graph_search", map[string]any{"query": "Foo", "mode": "semantic"})
	if !isErr || !strings.HasPrefix(text, "vectors_disabled") || !strings.Contains(text, "config_untrusted") ||
		!strings.Contains(text, "graph.vector.enabled") {
		t.Fatalf("want vectors_disabled with config_untrusted, got %s", text)
	}
}

// hashEmbedder gives each text a deterministic unit vector, so semantic
// ranking is stable without a model.
type hashEmbedder struct{ closed bool }

func (h *hashEmbedder) Info() embed.Info {
	return embed.Info{Engine: "fake", Commit: "c", ModelSHA256: "sha", Dims: 4, NCtx: 512}
}

func (h *hashEmbedder) Embed(_ context.Context, _ embed.Kind, texts []string) (vecs [][]float32, truncated []bool, err error) {
	for _, s := range texts {
		sum := sha256.Sum256([]byte(s))
		v := make([]float32, 4)
		var n float64
		for i := range v {
			v[i] = float32(sum[i]) + 1
			n += float64(v[i]) * float64(v[i])
		}
		for i := range v {
			v[i] /= float32(math.Sqrt(n))
		}
		vecs = append(vecs, v)
		truncated = append(truncated, false)
	}
	return vecs, truncated, nil
}

func (h *hashEmbedder) Close() error { h.closed = true; return nil }

// fakeVectorPool routes the tools to an in-memory embedder and counts
// child starts.
func fakeVectorPool(t *testing.T) (starts *int, children *[]*hashEmbedder) {
	t.Helper()
	var n int
	var kids []*hashEmbedder
	Shutdown()
	prev := newVectorPool
	newVectorPool = func() *vecsearch.Pool {
		return vecsearch.NewPool(func(context.Context, *embed.StartOptions) (embed.Embedder, bool, error) {
			n++
			h := &hashEmbedder{}
			kids = append(kids, h)
			return h, true, nil
		}, time.Hour)
	}
	t.Cleanup(func() { Shutdown(); newVectorPool = prev })
	return &n, &kids
}

// With vectors on, the tools match the CLI JSON, put warnings in the
// result and reuse one warm child; Shutdown closes it.
func TestGraphMCPVectorsWarmChild(t *testing.T) {
	dir := graphRepo(t)
	useConfig(t, "graph:\n  index_on_change: true\n  vector:\n    enabled: true\n  embed:\n    model: /models/m.gguf\n    model_sha256: "+strings.Repeat("a", 64)+"\n    allow_unsandboxed: true\n")
	starts, kids := fakeVectorPool(t)
	if text, isErr := callGraphText(t, "tb_graph_index", map[string]any{"vectors": true}); isErr {
		t.Fatalf("tb_graph_index vectors: %s", text)
	}
	idx, _ := callGraph(t, "tb_graph_index", map[string]any{"vectors": true})
	vst, ok := idx["vectors"].(map[string]any)
	if !ok || vst["units"].(float64) < 1 {
		t.Fatalf("index payload lacks vector stats: %v", idx)
	}
	if _, err := os.Stat(filepath.Join(dir, ".toolbelt", "graph.vec")); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"semantic", "hybrid"} {
		checkVectorSearch(t, mode)
	}
	if *starts != 1 {
		t.Fatalf("child starts = %d, want 1 across index and two searches", *starts)
	}
	Shutdown()
	if !(*kids)[0].closed {
		t.Fatal("Shutdown left the warm child running")
	}
}

// checkVectorSearch asserts the CLI's semantic/hybrid JSON shape plus the
// warnings array.
func checkVectorSearch(t *testing.T, mode string) {
	t.Helper()
	res, isErr := callGraph(t, "tb_graph_search", map[string]any{"query": "Foo", "mode": mode})
	if isErr {
		t.Fatalf("%s failed", mode)
	}
	if res["mode"] != mode || res["stale_vectors"] != false || res["stale"] != false {
		t.Fatalf("%s payload: %v", mode, res)
	}
	nodes, _ := res["nodes"].([]any)
	if len(nodes) == 0 {
		t.Fatalf("%s: no hits", mode)
	}
	hit := nodes[0].(map[string]any)
	for _, k := range []string{"id", "lines", "score_semantic"} {
		if _, ok := hit[k]; !ok {
			t.Fatalf("%s hit lacks %s: %v", mode, k, hit)
		}
	}
	if _, ok := hit["score_fused"]; ok != (mode == "hybrid") {
		t.Fatalf("%s hit score_fused presence wrong: %v", mode, hit)
	}
	warnings, _ := res["warnings"].([]any)
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0].(string), "embed_unsandboxed") {
		t.Fatalf("%s warnings: %v", mode, res["warnings"])
	}
}
