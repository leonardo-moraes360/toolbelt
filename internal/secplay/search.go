package secplay

import (
	"sort"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/mdparse"
)

// Hit is a search result: the playbook and its relevance score.
type Hit struct {
	Playbook *Playbook `json:"-"`
	Name     string    `json:"name"`
	Color    Color     `json:"color"`
	Area     string    `json:"area"`
	Summary  string    `json:"summary"`
	Score    float64   `json:"score"`
}

// SearchOptions filters and limits a search.
type SearchOptions struct {
	Area  string
	Limit int
}

// Search ranks the playbooks of a color against query over name, class,
// summary and triggers. Scoring is tiered: an exact name/class match outranks a
// substring match, which outranks term-overlap. Ties break by name, so results
// are deterministic. An empty query returns the (optionally area-filtered) list
// in name order.
func (c *Corpus) Search(color Color, query string, opts SearchOptions) []Hit {
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	items := c.List(color, opts.Area)

	q := mdparse.NormalizeForSearch(strings.TrimSpace(query))
	if q == "" {
		out := make([]Hit, 0, len(items))
		for _, pb := range items {
			out = append(out, hitOf(pb, 0))
		}
		if len(out) > limit {
			out = out[:limit]
		}
		return out
	}

	qTerms := tokenize(q)
	var scored []Hit
	for _, pb := range items {
		name := mdparse.NormalizeForSearch(pb.Name)
		class := mdparse.NormalizeForSearch(pb.Class)
		hay := mdparse.NormalizeForSearch(strings.Join([]string{
			pb.Name, pb.Class, pb.Summary, strings.Join(pb.Triggers, " "),
		}, " "))

		var score float64
		switch {
		case name == q || class == q:
			score = 1000
		case strings.Contains(name, q) || strings.Contains(class, q) || strings.Contains(hay, q):
			score = 500
		default:
			hayTerms := tokenize(hay)
			overlap := termOverlap(qTerms, hayTerms)
			if overlap == 0 {
				continue
			}
			score = float64(overlap)
		}
		scored = append(scored, hitOf(pb, score))
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Name < scored[j].Name
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored
}

func hitOf(pb *Playbook, score float64) Hit {
	return Hit{Playbook: pb, Name: pb.Name, Color: pb.Color, Area: pb.Area, Summary: pb.Summary, Score: score}
}

// tokenize splits on non-alphanumeric runs and lowercases; input is expected to
// be normalized already.
func tokenize(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
}

func termOverlap(q, hay []string) int {
	set := make(map[string]struct{}, len(hay))
	for _, t := range hay {
		set[t] = struct{}{}
	}
	seen := make(map[string]struct{}, len(q))
	n := 0
	for _, t := range q {
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		if _, ok := set[t]; ok {
			n++
		}
	}
	return n
}
