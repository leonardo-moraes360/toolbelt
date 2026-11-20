package vault

import (
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/mdparse"
)

const snippetRunes = 200

// SearchResult is tb vault search (R13).
type SearchResult struct {
	Query     string      `json:"query"`
	Hits      []SearchHit `json:"hits"`
	Truncated bool        `json:"truncated,omitempty"` // more notes matched than limit
}

// SearchHit is one matching note.
type SearchHit struct {
	Path  string     `json:"path"`
	File  string     `json:"file"`
	Line  int        `json:"line"`
	Field string     `json:"field"` // title, aliases, tags or body
	Data  SearchData `json:"data"`
}

// SearchData is the note-derived part of a hit, redacted.
type SearchData struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

// Search is tb vault search (R13). Each note is redacted as a whole first,
// and only the redacted text is searched, so a query can never probe a
// secret through hit counts or order. Matching is case- and
// accent-insensitive; one hit per note, sorted by path.
func (v *Vault) Search(query string, limit int) (*SearchResult, error) {
	q := mdparse.NormalizeForSearch(strings.TrimSpace(query))
	if q == "" {
		return nil, errorf(ExitError, "usage", "the query is empty")
	}
	if limit <= 0 {
		limit = 20
	}
	sc, err := v.scan()
	if err != nil {
		return nil, err
	}
	res := &SearchResult{Query: Redact(query), Hits: []SearchHit{}}
	for _, n := range sc.notes {
		h, ok := searchNote(n, q)
		if !ok {
			continue
		}
		if len(res.Hits) == limit {
			res.Truncated = true
			break
		}
		res.Hits = append(res.Hits, h)
	}
	return res, nil
}

func searchNote(n *Note, q string) (SearchHit, bool) {
	title := Redact(n.Title)
	h := SearchHit{Path: n.Path, File: n.File, Data: SearchData{Title: title}}
	fields := []struct {
		name string
		text string
		line int
	}{
		{"title", title, n.titleLine},
		{"aliases", strings.Join(redactAll(n.Aliases), "\n"), n.fieldLines["aliases"]},
		{"tags", strings.Join(redactAll(n.Tags), "\n"), n.fieldLines["tags"]},
	}
	for _, f := range fields {
		if at := matchAt(f.text, q); at >= 0 {
			h.Field, h.Line, h.Data.Snippet = f.name, max(1, f.line), snippet(f.text, at, q)
			return h, true
		}
	}
	body := Redact(n.Body)
	at := matchAt(body, q)
	if at < 0 {
		return SearchHit{}, false
	}
	h.Field = "body"
	h.Line = bodyStartLine(n) + strings.Count(string([]rune(body)[:at]), "\n")
	h.Data.Snippet = snippet(body, at, q)
	return h, true
}

// bodyStartLine is the 1-based file line where Body starts.
func bodyStartLine(n *Note) int {
	fm, ok, err := mdparse.SplitFrontmatter(n.rawHead)
	if ok && err == nil {
		return fm.EndLine + 1
	}
	return 1
}

// matchAt returns the rune offset in text of the first match of the
// normalized query q, walking text rune by rune so the offset is valid in
// text itself; -1 when there is none.
func matchAt(text, q string) int {
	// The redaction placeholder is not searchable: a query for "redacted"
	// must not reveal which notes hold a secret.
	text = strings.ReplaceAll(text, redacted, strings.Repeat(" ", len(redacted)))
	// Cheap pre-check on the whole text: most notes do not match, and the
	// rune-by-rune walk below only runs for those that do.
	if !strings.Contains(mdparse.NormalizeForSearch(text), q) {
		return -1
	}
	rs := []rune(text)
	norm := make([]string, len(rs))
	for i, r := range rs {
		if r < 0x80 { // ASCII needs no decomposition
			norm[i] = asciiLower[r]
			continue
		}
		norm[i] = mdparse.NormalizeForSearch(string(r))
	}
	for i := range rs {
		if matchesFrom(norm, i, q) {
			return i
		}
	}
	return -1
}

func matchesFrom(norm []string, i int, q string) bool {
	rest := q
	for j := i; j < len(norm) && rest != ""; j++ {
		if !strings.HasPrefix(rest, norm[j]) {
			return false
		}
		rest = rest[len(norm[j]):]
	}
	return rest == ""
}

// snippet is up to snippetRunes runes of text around offset at.
func snippet(text string, at int, q string) string {
	rs := []rune(text)
	span := len([]rune(q))
	start := max(0, at-(snippetRunes-span)/2)
	end := min(len(rs), start+snippetRunes)
	start = max(0, end-snippetRunes)
	return strings.TrimSpace(string(rs[start:end]))
}

// asciiLower holds the one-byte lower-cased strings of ASCII runes.
var asciiLower = func() (t [128]string) {
	for i := range t {
		t[i] = strings.ToLower(string(rune(i)))
	}
	return t
}()
