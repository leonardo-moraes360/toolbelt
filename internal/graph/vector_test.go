package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/leonardo-moraes360/toolbelt/internal/embed"
)

// fakeEmbedder returns hashed bag-of-words vectors and records every text.
type fakeEmbedder struct {
	mu    sync.Mutex
	info  embed.Info
	texts []string
	fail  error
}

func newFake() *fakeEmbedder {
	return &fakeEmbedder{info: embed.Info{Engine: "fake", Commit: "c1", ModelSHA256: "m1", Dims: 32, NCtx: 512}}
}

func (f *fakeEmbedder) Info() embed.Info { return f.info }
func (f *fakeEmbedder) Close() error     { return nil }

func (f *fakeEmbedder) Embed(_ context.Context, kind embed.Kind, texts []string) (vecs [][]float32, trunc []bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, nil, f.fail
	}
	if kind == embed.KindDoc {
		f.texts = append(f.texts, texts...)
	}
	vecs = make([][]float32, len(texts))
	for i, t := range texts {
		v := make([]float32, f.info.Dims)
		for _, tok := range tokenize(t) {
			h := fnv.New32a()
			h.Write([]byte(tok))
			v[h.Sum32()%uint32(f.info.Dims)]++
		}
		var n float64
		for _, x := range v {
			n += float64(x * x)
		}
		if n > 0 {
			for d := range v {
				v[d] /= float32(math.Sqrt(n))
			}
		}
		vecs[i] = v
	}
	return vecs, make([]bool, len(texts)), nil
}

func (f *fakeEmbedder) take() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.texts
	f.texts = nil
	return t
}

func writeRepo(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func buildIdx(t *testing.T, root string) *Index {
	t.Helper()
	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

// optsPtr lets tests pass option literals to BuildVectors.
func optsPtr[T any](v T) *T { return &v }

func smallOpts() VectorOptions {
	return VectorOptions{ChunkLines: 4, ChunkBytes: 256, OverlapLines: 1}
}

func numbered(prefix string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "// %s line %d word%d\n", prefix, i, i)
	}
	return b.String()
}

func baseRepo(t *testing.T) string {
	root := t.TempDir()
	writeRepo(t, root, map[string]string{
		"go.mod":       "module example.com/m\n",
		"alpha.go":     "package m\n\n" + numbered("alpha", 20) + "func Alpha() {}\n",
		"beta.go":      "package m\n\n" + numbered("beta", 10) + "func Beta() {}\n",
		"docs/note.md": "# Giraffe notes\n\nThe giraffe eats acacia leaves.\n",
	})
	return root
}

func TestLexicalHitJSONUnchanged(t *testing.T) {
	s := searchStore(
		Node{ID: "func:a.Scale", Type: NodeFunction, Title: "Scale", Path: "a/x.go", Package: "a", Source: "extracted", Confidence: 0.9},
		Node{ID: "file:a/scale.go", Type: NodeFile, Title: "scale.go", Path: "a/scale.go", Source: "extracted", Confidence: 1},
	)
	// Captured before SearchHit gained the vector fields.
	const golden = `[{"Node":{"id":"func:a.Scale","type":"function","path":"a/x.go","title":"Scale","package":"a","source":"extracted","confidence":0.9},"Score":0.29439077060308266},{"Node":{"id":"file:a/scale.go","type":"file","path":"a/scale.go","title":"scale.go","source":"extracted","confidence":1},"Score":0.30201029643045657}]`
	b, err := json.Marshal(s.SearchRanked("scale", 10))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != golden {
		t.Fatalf("lexical JSON changed:\n%s", b)
	}
}

func TestBuildVectorsIncremental(t *testing.T) {
	root := baseRepo(t)
	fake := newFake()
	o := smallOpts()
	ctx := context.Background()
	st, err := BuildVectors(ctx, root, buildIdx(t, root), fake, &o)
	if err != nil {
		t.Fatal(err)
	}
	first := fake.take()
	if st.Embedded == 0 || len(first) != st.Embedded || !st.Rebuilt {
		t.Fatalf("first run: %+v, %d texts", st, len(first))
	}
	store := filepath.Join(root, ".toolbelt", "graph.vec")
	fi, err := os.Stat(store)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("store mode: %v %v", fi, err)
	}

	st, err = BuildVectors(ctx, root, buildIdx(t, root), fake, &o)
	if err != nil {
		t.Fatal(err)
	}
	if got := fake.take(); len(got) != 0 || st.Embedded != 0 || st.Rebuilt || st.Reused != st.Units {
		t.Fatalf("no-change run sent %d texts: %+v", len(got), st)
	}

	checkEditReembedsOnlyChanged(t, root, store, fake, &o)
	checkVanishedFileDropped(t, root, store, fake, &o)
}

// checkEditReembedsOnlyChanged edits one line of alpha.go and expects only
// the chunks whose text changed to reach the embedder.
func checkEditReembedsOnlyChanged(t *testing.T, root, store string, fake *fakeEmbedder, o *VectorOptions) {
	t.Helper()
	before, _ := LoadVectors(store)
	oldSHA := map[[32]byte]bool{}
	for _, r := range before.records {
		oldSHA[r.SHA] = true
	}
	writeRepo(t, root, map[string]string{
		"alpha.go": strings.Replace("package m\n\n"+numbered("alpha", 20)+"func Alpha() {}\n", "word15", "zebra15", 1),
	})
	idx := buildIdx(t, root)
	n := o.normalized()
	units, _, _, err := collectUnits(root, idx, &n)
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	for _, u := range units {
		if !oldSHA[u.sha] {
			want++
		}
	}
	if _, err := BuildVectors(context.Background(), root, idx, fake, o); err != nil {
		t.Fatal(err)
	}
	got := fake.take()
	if want == 0 || len(got) != want {
		t.Fatalf("edit re-embedded %d texts, want %d", len(got), want)
	}
	for _, txt := range got {
		if !strings.Contains(txt, "zebra15") {
			t.Fatalf("unchanged chunk re-embedded: %q", txt)
		}
	}
}

// checkVanishedFileDropped removes beta.go and expects its records dropped
// without embedding anything.
func checkVanishedFileDropped(t *testing.T, root, store string, fake *fakeEmbedder, o *VectorOptions) {
	t.Helper()
	if err := os.Remove(filepath.Join(root, "beta.go")); err != nil {
		t.Fatal(err)
	}
	st, err := BuildVectors(context.Background(), root, buildIdx(t, root), fake, o)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := LoadVectors(store)
	for _, r := range after.records {
		if r.Path == "beta.go" {
			t.Fatal("vanished file kept in store")
		}
	}
	if st.Dropped == 0 || len(fake.take()) != 0 {
		t.Fatalf("vanish run: %+v", st)
	}
}

func TestBuildVectorsDenyAndRedact(t *testing.T) {
	root := t.TempDir()
	pem := "-----BEGIN RSA PRIVATE KEY-----\nMIIEowSECRETBODY1\nMIIEowSECRETBODY2\nMIIEowSECRETBODY3\nMIIEowSECRETBODY4\n-----END RSA PRIVATE KEY-----\n"
	writeRepo(t, root, map[string]string{
		"go.mod":     "module example.com/m\n",
		"secrets.go": "package m\n\n// deniedmarkerword\nfunc S() {}\n",
		"keys.md":    "# Keys\n\nline a\n" + pem + "line b\n",
	})
	idx := buildIdx(t, root)
	if _, ok := unitNodes(idx)["secrets.go"]; !ok {
		t.Fatal("fixture: secrets.go not indexed")
	}
	fake := newFake()
	st, err := BuildVectors(context.Background(), root, idx, fake, optsPtr(smallOpts()))
	if err != nil {
		t.Fatal(err)
	}
	if st.Denied != 1 {
		t.Fatalf("denied = %d", st.Denied)
	}
	all := strings.Join(fake.take(), "\n")
	if strings.Contains(all, "deniedmarkerword") {
		t.Fatal("deny-listed file sent to embedder")
	}
	if strings.Contains(all, "SECRETBODY") || strings.Contains(all, "PRIVATE KEY") {
		t.Fatalf("secret reached embedder: %q", all)
	}
	if !strings.Contains(all, "[REDACTED]") || !strings.Contains(all, "line b") {
		t.Fatalf("redacted file not embedded: %q", all)
	}
}

// A tracked symlink with an innocent name must not smuggle a denied file
// (or a denied directory) past the name-based deny list.
func TestBuildVectorsSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	writeRepo(t, root, map[string]string{
		"go.mod":                  "module example.com/m\n",
		".env":                    "symlinkedsecretmarker=1\n",
		".toolbelt/vault/prod.md": "# prod\n\nsymlinkeddirmarker\n",
		"real.md":                 "# Real\n\nplain text\n",
	})
	if err := os.Symlink(".env", filepath.Join(root, "notes.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(".toolbelt", filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	idx := buildIdx(t, root)
	fake := newFake()
	if _, err := BuildVectors(context.Background(), root, idx, fake, optsPtr(smallOpts())); err != nil {
		t.Fatal(err)
	}
	all := strings.Join(fake.take(), "\n")
	if strings.Contains(all, "symlinkedsecretmarker") || strings.Contains(all, "symlinkeddirmarker") {
		t.Fatalf("a symlinked path reached the embedder: %q", all)
	}
	if !strings.Contains(all, "plain text") {
		t.Fatalf("regular file not embedded: %q", all)
	}
}

func TestVectorsStale(t *testing.T) {
	root := baseRepo(t)
	fake := newFake()
	o := smallOpts()
	if _, err := BuildVectors(context.Background(), root, buildIdx(t, root), fake, &o); err != nil {
		t.Fatal(err)
	}
	s, err := LoadVectors(filepath.Join(root, ".toolbelt", "graph.vec"))
	if err != nil {
		t.Fatal(err)
	}
	info := fake.Info()
	if s.Stale(&o, info) {
		t.Fatal("fresh store reported stale")
	}
	cases := map[string]func(*VectorOptions, *embed.Info){
		"model":   func(_ *VectorOptions, i *embed.Info) { i.ModelSHA256 = "m2" },
		"dims":    func(_ *VectorOptions, i *embed.Info) { i.Dims = 16 },
		"lines":   func(o *VectorOptions, _ *embed.Info) { o.ChunkLines = 5 },
		"bytes":   func(o *VectorOptions, _ *embed.Info) { o.ChunkBytes = 300 },
		"overlap": func(o *VectorOptions, _ *embed.Info) { o.OverlapLines = 2 },
		"qprefix": func(o *VectorOptions, _ *embed.Info) { o.QueryPrefix = "search_query: " },
		"dprefix": func(o *VectorOptions, _ *embed.Info) { o.DocPrefix = "search_document: " },
	}
	for name, mut := range cases {
		o2, i2 := o, info
		mut(&o2, &i2)
		if !s.Stale(&o2, i2) {
			t.Errorf("%s change not stale", name)
		}
	}
	o2 := o
	o2.DocPrefix = "x"
	fake.take()
	st, err := BuildVectors(context.Background(), root, buildIdx(t, root), fake, optsPtr(o2))
	if err != nil {
		t.Fatal(err)
	}
	if !st.Rebuilt || st.Embedded != st.Units {
		t.Fatalf("stale store not rebuilt: %+v", st)
	}
}

func TestEmbedderErrorLeavesStore(t *testing.T) {
	root := baseRepo(t)
	fake := newFake()
	o := smallOpts()
	if _, err := BuildVectors(context.Background(), root, buildIdx(t, root), fake, &o); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".toolbelt", "graph.vec")
	orig, _ := os.ReadFile(path)
	writeRepo(t, root, map[string]string{"beta.go": "package m\n\n// changed body\nfunc Beta() {}\n"})
	fake.fail = fmt.Errorf("%w: boom", embed.ErrEmbedCrashed)
	_, err := BuildVectors(context.Background(), root, buildIdx(t, root), fake, &o)
	if !errors.Is(err, embed.ErrEmbedCrashed) {
		t.Fatalf("err = %v", err)
	}
	now, _ := os.ReadFile(path)
	if !bytes.Equal(orig, now) {
		t.Fatal("graph.vec changed after embedder failure")
	}
	ents, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".graph.vec.tmp") {
			t.Fatalf("temp file left: %s", e.Name())
		}
	}
}

func TestBuildVectorsTooMany(t *testing.T) {
	root := baseRepo(t)
	o := smallOpts()
	o.MaxChunks = 2
	fake := newFake()
	_, err := BuildVectors(context.Background(), root, buildIdx(t, root), fake, &o)
	if !errors.Is(err, ErrVectorsTooMany) || len(fake.take()) != 0 {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadVectorsCorrupt(t *testing.T) {
	s := &VectorStore{
		Header: VectorHeader{V: 1, ModelSHA256: "m", Dims: 3, ChunkLines: 4, ChunkBytes: 64, Count: 2},
		records: []vectorRecord{
			{Path: "a.go", Start: 1, End: 4, Vec: []float32{1, 0, 0}},
			{Path: "b/c.md", Start: 3, End: 3, Truncated: true, Vec: []float32{0, 1, 0}},
		},
	}
	data, err := s.encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeVectors(data, 0)
	if err != nil || !reflect.DeepEqual(got.records, s.records) {
		t.Fatalf("round trip: %v", err)
	}
	if _, err := decodeVectors(data, 1); !errors.Is(err, ErrVectorsInvalid) {
		t.Fatalf("count > max_chunks: %v", err)
	}
	for i := 0; i < len(data); i++ {
		if _, err := decodeVectors(data[:i], 0); !errors.Is(err, ErrVectorsInvalid) {
			t.Fatalf("truncated at %d: %v", i, err)
		}
	}
	if _, err := decodeVectors(append(append([]byte{}, data...), 0), 0); !errors.Is(err, ErrVectorsInvalid) {
		t.Fatal("trailing byte accepted")
	}
	for i := 0; i < len(data); i++ {
		for _, x := range []byte{0xff, 0x01} {
			c := append([]byte{}, data...)
			c[i] ^= x
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic flipping byte %d: %v", i, r)
					}
				}()
				_, _ = decodeVectors(c, 0)
			}()
		}
	}
	// A header claiming a huge count must fail before allocating.
	big := strings.Replace(string(data), `"count":2`, `"count":2000000000`, 1)
	if _, err := decodeVectors([]byte(big), 0); !errors.Is(err, ErrVectorsInvalid) {
		t.Fatalf("huge count: %v", err)
	}
	p := filepath.Join(t.TempDir(), "graph.vec")
	_ = os.WriteFile(p, data[:len(data)-3], 0o600)
	if _, err := LoadVectors(p); !errors.Is(err, ErrVectorsInvalid) {
		t.Fatalf("LoadVectors truncated: %v", err)
	}
}

func TestChunkTextLongLineUTF8(t *testing.T) {
	line := strings.Repeat("aé€😀", 200) // 1, 2, 3, 4-byte runes
	text := "short\n" + line + "\nend\n"
	cs := chunkText(text, 40, 100, 8)
	var rebuilt strings.Builder
	for _, c := range cs {
		if len(c.text) > 100 || !utf8.ValidString(c.text) {
			t.Fatalf("bad chunk %d-%d len %d valid %v", c.start, c.end, len(c.text), utf8.ValidString(c.text))
		}
	}
	// Without overlap the pieces concatenate back to the input.
	for _, c := range chunkText(text, 40, 100, 0) {
		rebuilt.WriteString(c.text)
	}
	if rebuilt.String() != text {
		t.Fatal("chunks lost text")
	}
	mid := 0
	for _, c := range chunkText(text, 40, 100, 0) {
		if c.start == 2 && c.end == 2 {
			mid++
		}
	}
	if mid < 2 {
		t.Fatalf("long line not split: %d pieces", mid)
	}
}

func TestChunkTextShape(t *testing.T) {
	text := numbered("x", 10)
	cs := chunkText(text, 4, 1<<10, 1)
	want := [][2]int{{1, 4}, {4, 7}, {7, 10}}
	if len(cs) != len(want) {
		t.Fatalf("chunks %v", cs)
	}
	for i, c := range cs {
		if c.start != want[i][0] || c.end != want[i][1] {
			t.Fatalf("chunk %d = %d-%d, want %v", i, c.start, c.end, want[i])
		}
	}
}

func TestSearchSemanticAndHybrid(t *testing.T) {
	root := baseRepo(t)
	fake := newFake()
	o := smallOpts()
	idx := buildIdx(t, root)
	if _, err := BuildVectors(context.Background(), root, idx, fake, &o); err != nil {
		t.Fatal(err)
	}
	store, err := LoadVectorsOpts(filepath.Join(root, ".toolbelt", "graph.vec"), &o)
	if err != nil {
		t.Fatal(err)
	}
	if store.StaleGraph(idx) {
		t.Fatal("fresh graph reported stale")
	}
	checkSemanticTopHit(t, idx, store, fake)
	checkHybridDeterministic(t, idx, store, fake)
	checkPrunedIndex(t, idx, store, fake)

	other := newFake()
	other.info.ModelSHA256 = "m2"
	if _, err := SearchSemantic(context.Background(), idx, store, other, "x", 5); !errors.Is(err, ErrVectorsStale) {
		t.Fatalf("other model: %v", err)
	}
}

func checkSemanticTopHit(t *testing.T, idx *Index, store *VectorStore, fake *fakeEmbedder) {
	t.Helper()
	hits, err := SearchSemantic(context.Background(), idx, store, fake, "giraffe acacia leaves", 10)
	if err != nil || len(hits) == 0 {
		t.Fatalf("semantic: %v %v", hits, err)
	}
	if hits[0].Node.ID != "file:docs/note.md" || hits[0].Lines == "" || hits[0].ScoreSemantic == nil {
		t.Fatalf("top hit %+v", hits[0])
	}
	b, _ := json.Marshal(hits[0])
	if !strings.Contains(string(b), `"lines":"1-3"`) || !strings.Contains(string(b), `"score_semantic"`) {
		t.Fatalf("semantic JSON %s", b)
	}
}

func checkHybridDeterministic(t *testing.T, idx *Index, store *VectorStore, fake *fakeEmbedder) {
	t.Helper()
	ctx := context.Background()
	h1, err := SearchHybrid(ctx, idx, store, fake, "Alpha", 10)
	if err != nil {
		t.Fatal(err)
	}
	h2, _ := SearchHybrid(ctx, idx, store, fake, "Alpha", 10)
	if !reflect.DeepEqual(h1, h2) || len(h1) == 0 {
		t.Fatal("hybrid not deterministic")
	}
	for i := range h1 {
		h := &h1[i]
		if h.ScoreFused == nil || (h.RankLexical == nil && h.RankSemantic == nil) {
			t.Fatalf("hybrid hit missing fields: %+v", h)
		}
	}
}

// checkPrunedIndex expects a node gone from the index to be dropped, and the
// changed fingerprint to flag the vectors without refusing the search.
func checkPrunedIndex(t *testing.T, idx *Index, store *VectorStore, fake *fakeEmbedder) {
	t.Helper()
	pruned := &Index{Root: idx.Root, Fingerprint: "other"}
	for _, n := range idx.Nodes {
		if n.Path != "docs/note.md" {
			pruned.Nodes = append(pruned.Nodes, n)
		}
	}
	hits, err := SearchSemantic(context.Background(), pruned, store, fake, "giraffe acacia leaves", 10)
	if err != nil {
		t.Fatal(err)
	}
	for i := range hits {
		if hits[i].Node.Path == "docs/note.md" {
			t.Fatal("hit for removed node")
		}
	}
	if !store.StaleGraph(pruned) {
		t.Fatal("changed fingerprint not reported")
	}
}

func TestFuseRRFTieBreak(t *testing.T) {
	a := Node{ID: "file:a.go"}
	b := Node{ID: "file:b.go"}
	c := Node{ID: "file:c.go"}
	s := 0.5
	lex := []SearchHit{{Node: b}, {Node: a}, {Node: c}}
	sem := []SearchHit{{Node: a, ScoreSemantic: &s}, {Node: b, ScoreSemantic: &s}}
	for i := 0; i < 20; i++ {
		got := fuseRRF(lex, sem)
		if len(got) != 3 || got[0].Node.ID != "file:a.go" || got[1].Node.ID != "file:b.go" || got[2].Node.ID != "file:c.go" {
			t.Fatalf("order %v", []string{got[0].Node.ID, got[1].Node.ID, got[2].Node.ID})
		}
		r1, r2 := 61.0, 62.0
		want := 1/r2 + 1/r1
		if *got[0].ScoreFused != want || *got[0].RankLexical != 2 || *got[0].RankSemantic != 1 {
			t.Fatalf("fused %+v", got[0])
		}
		if got[2].RankSemantic != nil || got[2].Lines != "" {
			t.Fatalf("lexical-only hit gained semantic fields: %+v", got[2])
		}
	}
}
