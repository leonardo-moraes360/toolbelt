package graph

import (
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// BM25 parameters (Robertson/Zaragoza defaults).
const (
	bm25K1 = 1.2
	bm25B  = 0.75

	// Field weights (BM25F-lite): a term in the title counts more than the
	// same term in the path or package.
	weightTitle   = 3.0
	weightPath    = 1.0
	weightPackage = 1.0
)

// Ranking tiers, highest first. Within a tier hits are ordered by BM25 score,
// then by node ID.
const (
	tierTerms     = iota // only BM25 term matches
	tierSubstring        // query is a substring of title/path/package/ID
	tierExact            // title equals the query (case-insensitive)
)

// SearchHit is one ranked node. The semantic/hybrid fields are omitempty so
// a lexical hit marshals exactly as before vectors existed (acceptance 8).
type SearchHit struct {
	Node  Node
	Score float64
	tier  int

	Lines         string   `json:"lines,omitempty"`
	ScoreSemantic *float64 `json:"score_semantic,omitempty"`
	RankLexical   *int     `json:"rank_lexical,omitempty"`
	RankSemantic  *int     `json:"rank_semantic,omitempty"`
	ScoreFused    *float64 `json:"score_fused,omitempty"`
}

type searchStats struct {
	idx      *Index
	nodes    int
	docs     []map[string]float64
	lengths  []float64
	avgLen   float64
	df       map[string]int
	haystack []string
}

var (
	searchCacheMu sync.Mutex
	searchCache   *searchStats
)

// See SearchRanked for the ranking rules.
func (s *Store) Search(query string, limit int) []Node {
	hits := s.SearchRanked(query, limit)
	if len(hits) == 0 {
		return nil
	}
	out := make([]Node, len(hits))
	for i := range hits {
		out[i] = hits[i].Node
	}
	return out
}

// SearchRanked ranks nodes by tier, then by BM25 over title/path/package
// tokens (camelCase, snake_case and path separators are split; multi-word
// queries match any term). Tiers, highest first: title equal to the query,
// query contained as a substring of title/path/package/ID (what the old
// search matched), BM25 term match only. Ties are broken by node ID so
// results are deterministic.
func (s *Store) SearchRanked(query string, limit int) []SearchHit {
	s.mu.RLock()
	idx := s.index
	s.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" || limit <= 0 || idx == nil || len(idx.Nodes) == 0 {
		return nil
	}
	st := statsFor(idx)
	qTerms := uniqueTerms(tokenize(query))
	n := float64(len(idx.Nodes))
	idf := make(map[string]float64, len(qTerms))
	for _, t := range qTerms {
		d := float64(st.df[t])
		idf[t] = math.Log(1 + (n-d+0.5)/(d+0.5))
	}

	var hits []SearchHit
	for i := range idx.Nodes {
		nd := &idx.Nodes[i]
		score := 0.0
		for _, t := range qTerms {
			f := st.docs[i][t]
			if f == 0 {
				continue
			}
			norm := 1 - bm25B + bm25B*st.lengths[i]/st.avgLen
			score += idf[t] * f * (bm25K1 + 1) / (f + bm25K1*norm)
		}
		tier := tierTerms
		switch {
		case strings.EqualFold(nd.Title, q):
			tier = tierExact
		case strings.Contains(st.haystack[i], q):
			tier = tierSubstring
		case score == 0:
			continue
		}
		hits = append(hits, SearchHit{Node: *nd, Score: score, tier: tier})
	}
	sort.Slice(hits, func(a, b int) bool {
		ha, hb := &hits[a], &hits[b]
		if ha.tier != hb.tier {
			return ha.tier > hb.tier
		}
		if ha.Score != hb.Score {
			return ha.Score > hb.Score
		}
		return ha.Node.ID < hb.Node.ID
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// statsFor returns term statistics for idx, reusing the last computed set
// when it belongs to the same index. Indexes are replaced, not mutated, on
// Load/Rebuild; the node count guards against in-place appends.
func statsFor(idx *Index) *searchStats {
	searchCacheMu.Lock()
	defer searchCacheMu.Unlock()
	if c := searchCache; c != nil && c.idx == idx && c.nodes == len(idx.Nodes) {
		return c
	}
	st := &searchStats{
		idx:      idx,
		nodes:    len(idx.Nodes),
		docs:     make([]map[string]float64, len(idx.Nodes)),
		lengths:  make([]float64, len(idx.Nodes)),
		df:       map[string]int{},
		haystack: make([]string, len(idx.Nodes)),
	}
	var total float64
	for i := range idx.Nodes {
		nd := &idx.Nodes[i]
		tf, l := weightedTermFreqs(nd)
		st.docs[i], st.lengths[i] = tf, l
		st.haystack[i] = strings.ToLower(nd.Title + " " + nd.Path + " " + nd.Package + " " + nd.ID)
		total += l
		for t := range tf {
			st.df[t]++
		}
	}
	st.avgLen = total / float64(len(idx.Nodes))
	if st.avgLen == 0 {
		st.avgLen = 1
	}
	searchCache = st
	return st
}

func weightedTermFreqs(n *Node) (tf map[string]float64, length float64) {
	tf = map[string]float64{}
	add := func(text string, w float64) {
		for _, t := range tokenize(text) {
			tf[t] += w
			length += w
		}
	}
	add(n.Title, weightTitle)
	add(n.Path, weightPath)
	add(n.Package, weightPackage)
	return tf, length
}

// tokenize lowercases text and splits it on runs that are not letters, digits
// or combining marks, and on camelCase / letter-digit boundaries. A word that
// splits into several parts is also emitted whole, so "numeroBr" yields
// "numerobr", "numero" and "br".
func tokenize(text string) []string {
	var out []string
	for _, word := range strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r)
	}) {
		parts := splitCamel(word)
		if len(parts) > 1 {
			out = append(out, strings.ToLower(word))
		}
		for _, p := range parts {
			out = append(out, strings.ToLower(p))
		}
	}
	return out
}

// splitCamel splits "parseHTTPRequest2x" into ["parse", "HTTP", "Request", "2", "x"].
// An upper-case run followed by a single lower-case letter is kept whole, so
// plural acronyms such as "APIs" or "IDs" are not split.
func splitCamel(word string) []string {
	rs := []rune(word)
	var parts []string
	start := 0
	for i := 1; i < len(rs); i++ {
		prev, cur := rs[i-1], rs[i]
		boundary := false
		switch {
		case unicode.IsMark(cur):
			// combining mark stays with its base letter
		case unicode.IsLower(prev) && unicode.IsUpper(cur):
			boundary = true
		case unicode.IsUpper(prev) && unicode.IsUpper(cur) &&
			i+2 < len(rs) && unicode.IsLower(rs[i+1]) && unicode.IsLower(rs[i+2]):
			boundary = true
		case unicode.IsDigit(prev) != unicode.IsDigit(cur) && !unicode.IsMark(prev):
			boundary = true
		}
		if boundary {
			parts = append(parts, string(rs[start:i]))
			start = i
		}
	}
	return append(parts, string(rs[start:]))
}

func uniqueTerms(terms []string) []string {
	seen := map[string]bool{}
	out := terms[:0]
	for _, t := range terms {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}
