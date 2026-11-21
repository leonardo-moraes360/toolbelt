package graph

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/leonardo-moraes360/toolbelt/internal/embed"
	"github.com/leonardo-moraes360/toolbelt/internal/secrets"
)

// Vector index errors (spec reason codes, used as the error text prefix).
var (
	ErrVectorsDisabled = errors.New("vectors_disabled")
	ErrVectorsStale    = errors.New("vectors_stale")
	ErrVectorsInvalid  = errors.New("vectors_invalid")
	ErrVectorsTooMany  = errors.New("vectors_too_many")
)

const (
	DefaultVectorFile   = "graph.vec"
	DefaultMaxChunks    = 50000
	DefaultChunkLines   = 40
	DefaultChunkBytes   = 1536
	DefaultOverlapLines = 8

	vectorMagic   = "TBVEC1"
	vectorVersion = 1
	rrfK          = 60

	// Read-side caps, so a hostile or corrupt header cannot make LoadVectors
	// allocate unbounded memory before the exact-size check.
	maxHeaderBytes = 1 << 20
	maxVectorDims  = 1 << 16
	maxUnitPath    = 4096
)

// VectorOptions is the chunk shape and limits taken from the user-only
// config (graph.vector / graph.embed). Zero values get the spec defaults.
type VectorOptions struct {
	MaxChunks    int
	ChunkLines   int
	ChunkBytes   int
	OverlapLines int
	// MaxFileBytes mirrors graph.max_file_bytes; 0 means DefaultMaxFileBytes.
	MaxFileBytes int64
	// The prefixes are applied by the embedder; graph only hashes them so a
	// change makes the store stale.
	QueryPrefix string
	DocPrefix   string
	// Path of the store; empty means <repoRoot>/.toolbelt/graph.vec.
	Path string
}

func (o VectorOptions) normalized() VectorOptions {
	if o.MaxChunks <= 0 {
		o.MaxChunks = DefaultMaxChunks
	}
	if o.ChunkLines <= 0 {
		o.ChunkLines = DefaultChunkLines
	}
	if o.ChunkBytes <= 0 {
		o.ChunkBytes = DefaultChunkBytes
	}
	if o.OverlapLines < 0 {
		o.OverlapLines = 0
	}
	if o.OverlapLines >= o.ChunkLines {
		o.OverlapLines = o.ChunkLines - 1
	}
	if o.MaxFileBytes <= 0 {
		o.MaxFileBytes = DefaultMaxFileBytes
	}
	return o
}

func (o VectorOptions) prefixesSHA() string {
	sum := sha256.Sum256([]byte(o.QueryPrefix + "\x00" + o.DocPrefix))
	return hex.EncodeToString(sum[:])
}

// VectorPath is the store path next to graph.json.
func VectorPath(toolbeltDir string) string {
	return filepath.Join(toolbeltDir, DefaultVectorFile)
}

// VectorHeader is the JSON header of graph.vec.
type VectorHeader struct {
	V                int    `json:"v"`
	EngineCommit     string `json:"engine_commit"`
	ModelSHA256      string `json:"model_sha256"`
	Dims             int    `json:"dims"`
	ChunkLines       int    `json:"chunk_lines"`
	ChunkBytes       int    `json:"chunk_bytes"`
	OverlapLines     int    `json:"overlap_lines"`
	PrefixesSHA256   string `json:"prefixes_sha256"`
	GraphFingerprint string `json:"graph_fingerprint"`
	Count            int    `json:"count"`
}

type vectorRecord struct {
	Path      string
	Start     int
	End       int
	SHA       [32]byte
	Truncated bool
	Vec       []float32
}

// VectorStore is a loaded graph.vec.
type VectorStore struct {
	Header  VectorHeader
	records []vectorRecord
}

// Stale reports whether the store was built with another model, engine,
// dimension, chunk shape or prefixes; a stale store is never queried
// (vectors_stale) and the next build re-embeds everything.
func (s *VectorStore) Stale(o VectorOptions, info embed.Info) bool {
	o = o.normalized()
	h := s.Header
	return h.V != vectorVersion ||
		h.ModelSHA256 != info.ModelSHA256 ||
		h.EngineCommit != info.Commit ||
		h.Dims != info.Dims ||
		h.ChunkLines != o.ChunkLines ||
		h.ChunkBytes != o.ChunkBytes ||
		h.OverlapLines != o.OverlapLines ||
		h.PrefixesSHA256 != o.prefixesSHA() ||
		h.Count > o.MaxChunks
}

// StaleGraph reports that graph.json changed since the store was built:
// searches still answer and the caller reports "stale_vectors": true.
func (s *VectorStore) StaleGraph(idx *Index) bool {
	return idx == nil || s.Header.GraphFingerprint != idx.Fingerprint
}

// VectorStats summarizes one BuildVectors run.
type VectorStats struct {
	Path      string `json:"path"`
	Files     int    `json:"files"`
	Units     int    `json:"units"`
	Embedded  int    `json:"embedded"`
	Reused    int    `json:"reused"`
	Dropped   int    `json:"dropped"`
	Denied    int    `json:"denied"`
	Truncated int    `json:"truncated"`
	// Rebuilt is set when no usable previous store existed (missing,
	// invalid or stale), so nothing could be reused.
	Rebuilt bool `json:"rebuilt"`
}

type unit struct {
	path       string
	start, end int
	text       string
	sha        [32]byte
}

// BuildVectors embeds the units of idx's files, reusing vectors from the
// existing store whose text sha256 is unchanged, and atomically replaces
// graph.vec (0600). Any embedder error returns before the store is touched.
func BuildVectors(ctx context.Context, repoRoot string, idx *Index, emb embed.Embedder, o VectorOptions) (VectorStats, error) {
	o = o.normalized()
	if o.Path == "" {
		o.Path = filepath.Join(repoRoot, ".toolbelt", DefaultVectorFile)
	}
	st := VectorStats{Path: o.Path}
	if idx == nil || emb == nil {
		return st, errors.New("graph: BuildVectors needs an index and an embedder")
	}
	info := emb.Info()
	if info.Dims <= 0 || info.Dims > maxVectorDims {
		return st, fmt.Errorf("%w: embedder reports %d dims", embed.ErrEmbedProtocol, info.Dims)
	}

	units, files, denied, err := collectUnits(repoRoot, idx, o)
	if err != nil {
		return st, err
	}
	st.Files, st.Units, st.Denied = files, len(units), denied
	if len(units) > o.MaxChunks {
		return st, fmt.Errorf("%w: %d chunks > max_chunks %d", ErrVectorsTooMany, len(units), o.MaxChunks)
	}

	type cached struct {
		vec       []float32
		truncated bool
	}
	reuse := map[[32]byte]cached{}
	oldKeys := map[string]bool{}
	if old, lerr := LoadVectors(o.Path); lerr == nil && !old.Stale(o, info) {
		for _, r := range old.records {
			reuse[r.SHA] = cached{r.Vec, r.Truncated}
			oldKeys[recordKey(r.Path, r.Start, r.End, r.SHA)] = true
		}
	} else {
		st.Rebuilt = true
	}

	var pending []int
	queued := map[[32]byte]bool{}
	for i := range units {
		sha := units[i].sha
		if _, ok := reuse[sha]; ok || queued[sha] {
			continue
		}
		queued[sha] = true
		pending = append(pending, i)
	}
	for b := 0; b < len(pending); b += embed.MaxTexts {
		if err := ctx.Err(); err != nil {
			return st, err
		}
		batch := pending[b:min(b+embed.MaxTexts, len(pending))]
		texts := make([]string, len(batch))
		for j, ui := range batch {
			texts[j] = units[ui].text
		}
		vecs, trunc, err := emb.Embed(ctx, embed.KindDoc, texts)
		if err != nil {
			return st, err
		}
		if len(vecs) != len(texts) || (trunc != nil && len(trunc) != len(texts)) {
			return st, fmt.Errorf("%w: %d vectors for %d texts", embed.ErrEmbedProtocol, len(vecs), len(texts))
		}
		for j, ui := range batch {
			if len(vecs[j]) != info.Dims || !finite(vecs[j]) {
				return st, fmt.Errorf("%w: vector %d has %d dims, want %d", embed.ErrEmbedProtocol, j, len(vecs[j]), info.Dims)
			}
			t := trunc != nil && trunc[j]
			reuse[units[ui].sha] = cached{vecs[j], t}
		}
		st.Embedded += len(batch)
	}

	recs := make([]vectorRecord, len(units))
	newKeys := map[string]bool{}
	for i, u := range units {
		c := reuse[u.sha]
		recs[i] = vectorRecord{Path: u.path, Start: u.start, End: u.end, SHA: u.sha, Truncated: c.truncated, Vec: c.vec}
		if c.truncated {
			st.Truncated++
		}
		k := recordKey(u.path, u.start, u.end, u.sha)
		newKeys[k] = true
		if oldKeys[k] {
			st.Reused++
		}
	}
	for k := range oldKeys {
		if !newKeys[k] {
			st.Dropped++
		}
	}
	store := &VectorStore{
		Header: VectorHeader{
			V: vectorVersion, EngineCommit: info.Commit, ModelSHA256: info.ModelSHA256, Dims: info.Dims,
			ChunkLines: o.ChunkLines, ChunkBytes: o.ChunkBytes, OverlapLines: o.OverlapLines,
			PrefixesSHA256: o.prefixesSHA(), GraphFingerprint: idx.Fingerprint, Count: len(recs),
		},
		records: recs,
	}
	data, err := store.encode()
	if err != nil {
		return st, err
	}
	return st, writeFileAtomic(o.Path, data)
}

func recordKey(p string, start, end int, sha [32]byte) string {
	return fmt.Sprintf("%s\x00%d\x00%d\x00%x", p, start, end, sha)
}

func finite(v []float32) bool {
	for _, f := range v {
		if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
			return false
		}
	}
	return true
}

// unitFiles returns the repository-relative files behind idx's file and note
// nodes, sorted: exactly what the indexer read, docs included.
func unitFiles(idx *Index) []string {
	seen := map[string]bool{}
	var out []string
	for i := range idx.Nodes {
		n := &idx.Nodes[i]
		if (n.Type != NodeFile && n.Type != NodeNote) || n.Path == "" {
			continue
		}
		p := filepath.ToSlash(n.Path)
		if seen[p] || !filepath.IsLocal(filepath.FromSlash(p)) {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// collectUnits reads each file through an os.Root, so a symlink cannot lead
// outside the repository, skipping deny-listed names before any read.
func collectUnits(repoRoot string, idx *Index, o VectorOptions) (units []unit, files, denied int, err error) {
	abs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, 0, 0, err
	}
	if r, rerr := filepath.EvalSymlinks(abs); rerr == nil {
		abs = r
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, 0, 0, err
	}
	defer root.Close()
	for _, rel := range unitFiles(idx) {
		if secrets.Denied(rel) {
			denied++
			continue
		}
		text, ok := readUnitFile(root, rel, o.MaxFileBytes)
		if !ok {
			continue
		}
		redacted, _ := secrets.Redact(text)
		cs := chunkText(redacted, o.ChunkLines, o.ChunkBytes, o.OverlapLines)
		if len(cs) == 0 {
			continue
		}
		files++
		for _, c := range cs {
			units = append(units, unit{path: rel, start: c.start, end: c.end, text: c.text, sha: sha256.Sum256([]byte(c.text))})
		}
	}
	return units, files, denied, nil
}

func readUnitFile(root *os.Root, rel string, maxBytes int64) (string, bool) {
	// The deny list matches the indexed name, so a symlink anywhere on the
	// path (docs/notes.md -> ../.env) would smuggle a denied file in: refuse
	// symlinks on every component instead of resolving them.
	parts := strings.Split(rel, "/")
	for i := range parts {
		fi, err := root.Lstat(filepath.FromSlash(strings.Join(parts[:i+1], "/")))
		if err != nil || fi.Mode()&os.ModeSymlink != 0 {
			return "", false
		}
	}
	f, err := root.Open(filepath.FromSlash(rel))
	if err != nil {
		return "", false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxBytes {
		return "", false
	}
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil || int64(len(data)) > maxBytes || bytes.IndexByte(data, 0) >= 0 {
		return "", false
	}
	return strings.ToValidUTF8(string(data), "�"), true
}

type chunk struct {
	start, end int
	text       string
}

type segment struct {
	line int
	text string
}

// chunkText cuts text on line boundaries into chunks of at most maxLines
// lines and maxBytes bytes, overlapping by up to overlap lines. A line longer
// than maxBytes is split into pieces cut at UTF-8 boundaries. Whitespace-only
// chunks are skipped.
func chunkText(text string, maxLines, maxBytes, overlap int) []chunk {
	var segs []segment
	line := 1
	for rest := text; rest != ""; line++ {
		l := rest
		if i := strings.IndexByte(rest, '\n'); i >= 0 {
			l = rest[:i+1]
		}
		rest = rest[len(l):]
		for len(l) > maxBytes {
			cut := maxBytes
			for cut > 0 && !utf8.RuneStart(l[cut]) {
				cut--
			}
			if cut == 0 {
				cut = maxBytes
			}
			segs = append(segs, segment{line, l[:cut]})
			l = l[cut:]
		}
		if l != "" {
			segs = append(segs, segment{line, l})
		}
	}
	var out []chunk
	for i := 0; i < len(segs); {
		j, size := i, 0
		for j < len(segs) && j-i < maxLines && (j == i || size+len(segs[j].text) <= maxBytes) {
			size += len(segs[j].text)
			j++
		}
		var b strings.Builder
		for _, s := range segs[i:j] {
			b.WriteString(s.text)
		}
		if t := b.String(); strings.TrimSpace(t) != "" {
			out = append(out, chunk{segs[i].line, segs[j-1].line, t})
		}
		if j >= len(segs) {
			break
		}
		// A chunk cut short by bytes overlaps by at most half of itself, so a
		// file of long lines does not degrade into one chunk per line step.
		ov := min(overlap, (j-i)/2)
		i = j - ov
	}
	return out
}

func (s *VectorStore) encode() ([]byte, error) {
	hdr, err := json.Marshal(s.Header)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(vectorMagic)
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(hdr)))
	buf.Write(hdr)
	for i := range s.records {
		r := &s.records[i]
		if len(r.Path) == 0 || len(r.Path) > maxUnitPath || len(r.Vec) != s.Header.Dims {
			return nil, fmt.Errorf("graph: bad vector record %q", r.Path)
		}
		_ = binary.Write(&buf, binary.LittleEndian, uint16(len(r.Path)))
		buf.WriteString(r.Path)
		_ = binary.Write(&buf, binary.LittleEndian, uint32(r.Start))
		_ = binary.Write(&buf, binary.LittleEndian, uint32(r.End))
		buf.Write(r.SHA[:])
		if r.Truncated {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}
		_ = binary.Write(&buf, binary.LittleEndian, r.Vec)
	}
	return buf.Bytes(), nil
}

// LoadVectors reads graph.vec. Layout: "TBVEC1", u32 LE header length, JSON
// header, then count records of u16 path length, path, u32 start line, u32
// end line, 32-byte text sha256, u8 truncated, dims LE float32. Any
// mismatch, including a size that is not exactly what the header implies,
// is ErrVectorsInvalid; nothing is read past the end.
func LoadVectors(path string) (*VectorStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeVectors(data, 0)
}

// LoadVectorsOpts is LoadVectors that also enforces o.MaxChunks on count.
func LoadVectorsOpts(path string, o VectorOptions) (*VectorStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeVectors(data, o.normalized().MaxChunks)
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrVectorsInvalid}, a...)...)
}

func decodeVectors(data []byte, maxChunks int) (*VectorStore, error) {
	if len(data) < len(vectorMagic)+4 || string(data[:len(vectorMagic)]) != vectorMagic {
		return nil, invalid("bad magic")
	}
	p := data[len(vectorMagic):]
	hl := binary.LittleEndian.Uint32(p)
	p = p[4:]
	if hl == 0 || hl > maxHeaderBytes || uint64(hl) > uint64(len(p)) {
		return nil, invalid("bad header length")
	}
	var h VectorHeader
	dec := json.NewDecoder(bytes.NewReader(p[:hl]))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&h); err != nil || dec.More() {
		return nil, invalid("bad header")
	}
	p = p[hl:]
	if h.V != vectorVersion || h.Dims <= 0 || h.Dims > maxVectorDims || h.Count < 0 {
		return nil, invalid("bad header fields")
	}
	if maxChunks > 0 && h.Count > maxChunks {
		return nil, invalid("count %d > max_chunks %d", h.Count, maxChunks)
	}
	vecBytes := h.Dims * 4
	minRec := 2 + 1 + 8 + 32 + 1 + vecBytes
	if uint64(h.Count)*uint64(minRec) > uint64(len(p)) {
		return nil, invalid("file too short for %d records", h.Count)
	}
	s := &VectorStore{Header: h, records: make([]vectorRecord, h.Count)}
	for i := range s.records {
		if len(p) < 2 {
			return nil, invalid("truncated record %d", i)
		}
		pl := int(binary.LittleEndian.Uint16(p))
		p = p[2:]
		if pl == 0 || pl > maxUnitPath || len(p) < pl+8+32+1+vecBytes {
			return nil, invalid("truncated record %d", i)
		}
		r := &s.records[i]
		r.Path = string(p[:pl])
		p = p[pl:]
		r.Start = int(binary.LittleEndian.Uint32(p))
		r.End = int(binary.LittleEndian.Uint32(p[4:]))
		p = p[8:]
		copy(r.SHA[:], p[:32])
		p = p[32:]
		switch p[0] {
		case 0:
		case 1:
			r.Truncated = true
		default:
			return nil, invalid("bad truncated flag in record %d", i)
		}
		p = p[1:]
		r.Vec = make([]float32, h.Dims)
		for d := range r.Vec {
			r.Vec[d] = math.Float32frombits(binary.LittleEndian.Uint32(p[d*4:]))
		}
		p = p[vecBytes:]
		if r.Start < 1 || r.End < r.Start || !finite(r.Vec) || path.IsAbs(r.Path) {
			return nil, invalid("bad record %d", i)
		}
	}
	if len(p) != 0 {
		return nil, invalid("%d trailing bytes", len(p))
	}
	return s, nil
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".graph.vec.tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	// Persist the rename itself; on failure the old store comes back after a
	// crash, which is safe, so the error only matters on platforms that
	// support syncing a directory.
	if d, err := os.Open(filepath.Dir(path)); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// unitNodes maps a unit path to its node: the file node, else the vault
// note backed by that file.
func unitNodes(idx *Index) map[string]Node {
	m := map[string]Node{}
	for i := range idx.Nodes {
		n := idx.Nodes[i]
		if n.Path == "" {
			continue
		}
		p := filepath.ToSlash(n.Path)
		switch {
		case n.Type == NodeFile && n.ID == "file:"+p:
			m[p] = n
		case n.Type == NodeNote:
			if cur, ok := m[p]; !ok || (cur.Type == NodeNote && n.ID < cur.ID) {
				m[p] = n
			}
		}
	}
	return m
}

// SearchSemantic ranks file nodes by the cosine similarity of the query to
// their best chunk (brute force). Units whose node left the index are
// dropped. The caller checks Stale first and reports StaleGraph.
func SearchSemantic(ctx context.Context, idx *Index, store *VectorStore, emb embed.Embedder, query string, limit int) ([]SearchHit, error) {
	hits, err := semanticRanking(ctx, idx, store, emb, query)
	if err != nil {
		return nil, err
	}
	if limit >= 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func semanticRanking(ctx context.Context, idx *Index, store *VectorStore, emb embed.Embedder, query string) ([]SearchHit, error) {
	if idx == nil || store == nil || emb == nil || strings.TrimSpace(query) == "" {
		return nil, nil
	}
	info := emb.Info()
	if info.ModelSHA256 != store.Header.ModelSHA256 || info.Dims != store.Header.Dims {
		return nil, fmt.Errorf("%w: store built with another model", ErrVectorsStale)
	}
	vecs, _, err := emb.Embed(ctx, embed.KindQuery, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vecs) != 1 || len(vecs[0]) != store.Header.Dims || !finite(vecs[0]) {
		return nil, fmt.Errorf("%w: bad query vector", embed.ErrEmbedProtocol)
	}
	q := vecs[0]
	qn := norm(q)
	nodes := unitNodes(idx)
	type best struct {
		score      float64
		start, end int
	}
	byPath := map[string]best{}
	for i := range store.records {
		r := &store.records[i]
		if _, ok := nodes[r.Path]; !ok {
			continue
		}
		sc := cosine(q, qn, r.Vec)
		if b, ok := byPath[r.Path]; !ok || sc > b.score || (sc == b.score && r.Start < b.start) {
			byPath[r.Path] = best{sc, r.Start, r.End}
		}
	}
	hits := make([]SearchHit, 0, len(byPath))
	for p, b := range byPath {
		sc := b.score
		hits = append(hits, SearchHit{
			Node: nodes[p], Score: sc, ScoreSemantic: &sc,
			Lines: fmt.Sprintf("%d-%d", b.start, b.end),
		})
	}
	sort.Slice(hits, func(a, b int) bool {
		if hits[a].Score != hits[b].Score {
			return hits[a].Score > hits[b].Score
		}
		return hits[a].Node.ID < hits[b].Node.ID
	})
	return hits, nil
}

func norm(v []float32) float64 {
	var s float64
	for _, f := range v {
		s += float64(f) * float64(f)
	}
	return math.Sqrt(s)
}

func cosine(q []float32, qn float64, v []float32) float64 {
	var dot float64
	for i := range q {
		dot += float64(q[i]) * float64(v[i])
	}
	vn := norm(v)
	if qn == 0 || vn == 0 {
		return 0
	}
	return dot / (qn * vn)
}

// SearchHybrid fuses the full lexical ranking (SearchRanked) and the
// semantic ranking by Reciprocal Rank Fusion, k = 60; equal fused scores
// are ordered by node ID.
func SearchHybrid(ctx context.Context, idx *Index, store *VectorStore, emb embed.Embedder, query string, limit int) ([]SearchHit, error) {
	if idx == nil {
		return nil, nil
	}
	sem, err := semanticRanking(ctx, idx, store, emb, query)
	if err != nil {
		return nil, err
	}
	lex := (&Store{index: idx}).SearchRanked(query, len(idx.Nodes))
	hits := fuseRRF(lex, sem)
	if limit >= 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func fuseRRF(lex, sem []SearchHit) []SearchHit {
	byID := map[string]*SearchHit{}
	var order []string
	get := func(n Node) *SearchHit {
		h, ok := byID[n.ID]
		if !ok {
			h = &SearchHit{Node: n}
			byID[n.ID] = h
			order = append(order, n.ID)
		}
		return h
	}
	for i := range lex {
		h := get(lex[i].Node)
		r := i + 1
		h.RankLexical = &r
	}
	for i := range sem {
		h := get(sem[i].Node)
		r := i + 1
		h.RankSemantic = &r
		h.ScoreSemantic = sem[i].ScoreSemantic
		h.Lines = sem[i].Lines
	}
	out := make([]SearchHit, 0, len(order))
	for _, id := range order {
		h := byID[id]
		var f float64
		// Sum in a fixed order (lexical, then semantic) so equal inputs give
		// bit-identical scores.
		if h.RankLexical != nil {
			f += 1 / float64(rrfK+*h.RankLexical)
		}
		if h.RankSemantic != nil {
			f += 1 / float64(rrfK+*h.RankSemantic)
		}
		h.Score = f
		h.ScoreFused = &f
		out = append(out, *h)
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Score != out[b].Score {
			return out[a].Score > out[b].Score
		}
		return out[a].Node.ID < out[b].Node.ID
	})
	return out
}
