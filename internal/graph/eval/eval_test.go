// Package eval measures recall@10 of the lexical, semantic and hybrid graph
// search modes over this repository (docs/specs/vector-index.md,
// Evaluation (V2), acceptance 17).
package eval

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/leonardo-moraes360/toolbelt/internal/embed"
	"github.com/leonardo-moraes360/toolbelt/internal/embed/native"
	"github.com/leonardo-moraes360/toolbelt/internal/graph"
)

// childArg is the subcommand embed.Start passes to its executable. Without
// TB_EMBED_EXE the test binary itself is the child, so it must be built with
// -tags embednative.
const childArg = "__graph-embed"

const (
	topK        = 10
	minQueries  = 25
	requiredGap = 0.10
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == childArg {
		os.Exit(runChild(os.Args[2:]))
	}
	os.Exit(m.Run())
}

// runChild mirrors tb __graph-embed's serve mode.
func runChild(args []string) int {
	fs := flag.NewFlagSet(childArg, flag.ContinueOnError)
	model := fs.String("model", "", "")
	sha := fs.String("sha256", "", "")
	threads := fs.Int("threads", 2, "")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !native.Available {
		fmt.Fprintln(os.Stderr, "eval child:", native.ErrUnavailable)
		return 2
	}
	if err := embed.Serve(context.Background(), os.Stdin, os.Stdout, *model, *sha, *threads); err != nil {
		fmt.Fprintln(os.Stderr, "eval child:", err)
		return 2
	}
	return 0
}

type query struct {
	Query  string   `yaml:"query"`
	Expect []string `yaml:"expect"`
	Note   string   `yaml:"note,omitempty"`
}

type suite struct {
	Queries []query `yaml:"queries"`
}

func loadSuite(t *testing.T) []query {
	t.Helper()
	b, err := os.ReadFile("semantic.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var s suite
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		t.Fatalf("semantic.yaml: %v", err)
	}
	return s.Queries
}

// repoRoot walks up from the package directory to go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

var (
	camelRe = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	wordRe  = regexp.MustCompile(`[a-z0-9]+`)
)

// nameTokens are the words of a file name that a query must not reuse:
// basename without extension, split on _ - . and camelCase, at least 4 chars.
func nameTokens(p string) []string {
	base := strings.TrimSuffix(path.Base(p), path.Ext(p))
	base = strings.ToLower(camelRe.ReplaceAllString(base, "$1 $2"))
	var out []string
	for _, w := range wordRe.FindAllString(base, -1) {
		if len(w) >= 4 {
			out = append(out, w)
		}
	}
	return out
}

// TestSuiteShape keeps the query set honest without a model: enough
// queries, existing targets outside the excluded trees, and no query that
// names its own target file.
func TestSuiteShape(t *testing.T) {
	qs := loadSuite(t)
	if len(qs) < minQueries {
		t.Fatalf("%d queries, want at least %d", len(qs), minQueries)
	}
	root := repoRoot(t)
	seen := map[string]bool{}
	for i, q := range qs {
		if seen[q.Query] {
			t.Errorf("query %d: duplicate %q", i, q.Query)
		}
		seen[q.Query] = true
		checkQuery(t, root, i, q)
	}
}

func checkQuery(t *testing.T, root string, i int, q query) {
	t.Helper()
	if strings.TrimSpace(q.Query) == "" || len(q.Expect) == 0 || len(q.Expect) > 3 {
		t.Errorf("query %d: needs a query and 1-3 expected paths", i)
		return
	}
	words := map[string]bool{}
	for _, w := range wordRe.FindAllString(strings.ToLower(q.Query), -1) {
		words[w] = true
	}
	for _, p := range q.Expect {
		if filepath.IsAbs(p) || path.Clean(p) != p || strings.HasPrefix(p, "third_party/") || strings.HasPrefix(p, "docs/pt-BR/") {
			t.Errorf("query %d: expected path %q must be a clean repo path outside third_party/ and docs/pt-BR/", i, p)
			continue
		}
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil || !st.Mode().IsRegular() {
			t.Errorf("query %d: expected path %q is not a file in the repository", i, p)
		}
		for _, tok := range nameTokens(p) {
			if words[tok] {
				t.Errorf("query %d %q reuses %q from its target %s", i, q.Query, tok, p)
			}
		}
	}
}

type mode struct {
	name   string
	search func(ctx context.Context, q string) ([]graph.SearchHit, error)
	total  float64
}

// TestSemanticRecall needs TB_EMBED_MODEL (absolute GGUF path) and
// TB_EMBED_MODEL_SHA256. Optional: TB_EMBED_EVAL_CACHE (store directory, so
// reruns only embed changed chunks), TB_EMBED_ALLOW_UNSANDBOXED=1 and
// TB_EMBED_EXE (a tb binary to use as the child instead of this test binary).
// Recall never fails the test; only infrastructure errors do.
func TestSemanticRecall(t *testing.T) {
	model, sha := os.Getenv("TB_EMBED_MODEL"), os.Getenv("TB_EMBED_MODEL_SHA256")
	if model == "" || sha == "" {
		t.Skip("TB_EMBED_MODEL and TB_EMBED_MODEL_SHA256 not set")
	}
	if os.Getenv("TB_EMBED_EXE") == "" && !native.Available {
		t.Fatal("the test binary is the embedding child: run with -tags embednative or set TB_EMBED_EXE")
	}
	qs := loadSuite(t)
	root := repoRoot(t)

	opts := graph.DefaultBuildOptions()
	// SkipDirs match directory base names; pt-BR is only docs/pt-BR here.
	opts.SkipDirs = append(opts.SkipDirs, "third_party", "pt-BR")
	start := time.Now()
	// The store is the lexical ranker's only entry point over an index.
	lexStore := graph.NewStore(filepath.Join(t.TempDir(), "graph.json"))
	idx, err := lexStore.RebuildOpts(root, &opts)
	if err != nil {
		t.Fatalf("graph index: %v", err)
	}
	t.Logf("graph: %d nodes in %s", len(idx.Nodes), time.Since(start).Round(time.Millisecond))

	ctx := context.Background()
	emb := startEmbedder(ctx, t, root, model, sha)
	info := emb.Info()

	vo := &graph.VectorOptions{Path: filepath.Join(storeDir(t), graph.DefaultVectorFile),
		QueryPrefix: os.Getenv("TB_EMBED_QUERY_PREFIX"), DocPrefix: os.Getenv("TB_EMBED_DOC_PREFIX")}
	start = time.Now()
	st, err := graph.BuildVectors(ctx, root, idx, emb, vo)
	if err != nil {
		t.Fatalf("build vectors: %v", err)
	}
	t.Logf("vectors: %d files, %d units, %d embedded, %d reused in %s",
		st.Files, st.Units, st.Embedded, st.Reused, time.Since(start).Round(time.Second))
	store, err := graph.LoadVectorsOpts(vo.Path, vo)
	if err != nil {
		t.Fatalf("load vectors: %v", err)
	}

	modes := []*mode{
		{name: "lexical", search: func(_ context.Context, q string) ([]graph.SearchHit, error) {
			return lexStore.SearchRanked(q, topK), nil
		}},
		{name: "semantic", search: func(ctx context.Context, q string) ([]graph.SearchHit, error) {
			return graph.SearchSemantic(ctx, idx, store, emb, q, topK)
		}},
		{name: "hybrid", search: func(ctx context.Context, q string) ([]graph.SearchHit, error) {
			return graph.SearchHybrid(ctx, idx, store, emb, q, topK)
		}},
	}
	for _, q := range qs {
		line := make([]string, 0, len(modes))
		for _, m := range modes {
			hits, err := m.search(ctx, q.Query)
			if err != nil {
				t.Fatalf("%s %q: %v", m.name, q.Query, err)
			}
			r := recall(q.Expect, hits)
			m.total += r
			line = append(line, fmt.Sprintf("%s=%.2f", m.name, r))
		}
		t.Logf("%s | %q", strings.Join(line, " "), q.Query)
	}
	n := float64(len(qs))
	lex, sem, hyb := modes[0].total/n, modes[1].total/n, modes[2].total/n
	t.Logf("model sha256=%s engine=%s commit=%s dims=%d", info.ModelSHA256, info.Engine, info.Commit, info.Dims)
	t.Logf("RESULT lexical=%.3f semantic=%.3f hybrid=%.3f gain=%+.3f", lex, sem, hyb, hyb-lex)
	t.Logf("hybrid >= lexical + %.2f: %v", requiredGap, hyb >= lex+requiredGap)
}

// startEmbedder runs the child with the default argv; without TB_EMBED_EXE
// the executable is this test binary, which TestMain turns into the child.
func startEmbedder(ctx context.Context, t *testing.T, root, model, sha string) *embed.Client {
	t.Helper()
	threads, _ := strconv.Atoi(os.Getenv("TB_EMBED_THREADS"))
	emb, err := embed.Start(ctx, &embed.StartOptions{
		Exe: os.Getenv("TB_EMBED_EXE"), Model: model, SHA256: sha, Threads: max(threads, 4),
		RepoRoot:         root,
		AllowUnsandboxed: os.Getenv("TB_EMBED_ALLOW_UNSANDBOXED") == "1",
		// Models such as e5 or nomic need task prefixes; without them their
		// recall here would understate real use.
		QueryPrefix: os.Getenv("TB_EMBED_QUERY_PREFIX"),
		DocPrefix:   os.Getenv("TB_EMBED_DOC_PREFIX"),
	})
	if err != nil {
		t.Fatalf("embed start: %v", err)
	}
	t.Cleanup(func() { _ = emb.Close() })
	if emb.Unsandboxed() {
		t.Log("warning: embedding child ran without the sandbox")
	}
	return emb
}

// storeDir is TB_EMBED_EVAL_CACHE when set, so reruns reuse vectors.
func storeDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("TB_EMBED_EVAL_CACHE")
	if dir == "" {
		return t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// recall is the fraction of expected paths among the hits' node paths.
func recall(expect []string, hits []graph.SearchHit) float64 {
	got := map[string]bool{}
	for i := range hits {
		got[filepath.ToSlash(hits[i].Node.Path)] = true
	}
	found := 0
	for _, p := range expect {
		if got[p] {
			found++
		}
	}
	return float64(found) / float64(len(expect))
}
