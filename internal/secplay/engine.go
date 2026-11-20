package secplay

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"unicode/utf8"

	"github.com/leonardo-moraes360/toolbelt/internal/mdparse"
)

//go:embed corpus
var corpusFS embed.FS

// DefaultLoadBudget is the token cap for `load` when none is given.
const DefaultLoadBudget = 2000

// Corpus is the parsed set of playbooks, split by color and keyed by name.
type Corpus struct {
	Red  map[string]*Playbook
	Blue map[string]*Playbook
}

// Load parses the embedded corpus. It fails if any file is malformed or if the
// red/blue mirror links are inconsistent, so a broken corpus never ships.
func Load() (*Corpus, error) {
	return loadFS(corpusFS)
}

func loadFS(fsys fs.FS) (*Corpus, error) {
	c := &Corpus{Red: map[string]*Playbook{}, Blue: map[string]*Playbook{}}
	err := fs.WalkDir(fsys, "corpus", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		raw, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		pb, err := parsePlaybook(path, string(raw))
		if err != nil {
			return err
		}
		m := c.forColor(pb.Color)
		if _, dup := m[pb.Name]; dup {
			return fmt.Errorf("%s: duplicate playbook name %q", path, pb.Name)
		}
		m[pb.Name] = pb
		return nil
	})
	if err != nil {
		return nil, err
	}
	for name := range c.Red {
		if _, dup := c.Blue[name]; dup {
			return nil, fmt.Errorf("playbook name %q used by both a red and a blue playbook; names must be unique across colors", name)
		}
	}
	if err := c.validateMirrors(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Corpus) forColor(color Color) map[string]*Playbook {
	if color == Blue {
		return c.Blue
	}
	return c.Red
}

// validateMirrors checks that every declared mirror resolves to a playbook of
// the opposite color and that the link is reciprocal.
func (c *Corpus) validateMirrors() error {
	check := func(from map[string]*Playbook, to map[string]*Playbook, fromColor Color) error {
		for _, pb := range from {
			if pb.Mirror == "" {
				continue
			}
			other, ok := to[pb.Mirror]
			if !ok {
				return fmt.Errorf("%s: mirror %q not found in the opposite color", pb.Name, pb.Mirror)
			}
			if other.Mirror != pb.Name {
				return fmt.Errorf("%s: mirror %q does not point back (got %q)", pb.Name, pb.Mirror, other.Mirror)
			}
			if other.Class != pb.Class {
				return fmt.Errorf("%s: mirror %q is a different class (%q vs %q)", pb.Name, pb.Mirror, other.Class, pb.Class)
			}
		}
		return nil
	}
	if err := check(c.Red, c.Blue, Red); err != nil {
		return err
	}
	return check(c.Blue, c.Red, Blue)
}

// Get returns the playbook of the given color by exact name.
func (c *Corpus) Get(color Color, name string) (*Playbook, bool) {
	pb, ok := c.forColor(color)[name]
	return pb, ok
}

// Pair returns the mirror playbook of name, searching both colors. It resolves
// a red name to its blue mirror and vice-versa.
func (c *Corpus) Pair(name string) (*Playbook, bool) {
	if pb, ok := c.Red[name]; ok && pb.Mirror != "" {
		m, ok := c.Blue[pb.Mirror]
		return m, ok
	}
	if pb, ok := c.Blue[name]; ok && pb.Mirror != "" {
		m, ok := c.Red[pb.Mirror]
		return m, ok
	}
	return nil, false
}

// List returns the playbooks of a color in name order, optionally filtered by
// area ("" = all).
func (c *Corpus) List(color Color, area string) []*Playbook {
	m := c.forColor(color)
	var out []*Playbook
	for _, name := range sortedNames(m) {
		pb := m[name]
		if area != "" && pb.Area != area {
			continue
		}
		out = append(out, pb)
	}
	return out
}

// RenderOptions controls how Render returns a playbook body.
type RenderOptions struct {
	Section string // return only this section (by heading/slug) when set
	Budget  int    // token cap; <= 0 means DefaultLoadBudget
	Full    bool   // return the whole body, ignoring Budget
}

// Rendered is the result of Render.
type Rendered struct {
	Name      string   `json:"name"`
	Color     Color    `json:"color"`
	Summary   string   `json:"summary"`
	Section   string   `json:"section,omitempty"`
	Body      string   `json:"body"`
	Tokens    int      `json:"tokens"`
	Truncated bool     `json:"truncated"`
	Sections  []string `json:"sections,omitempty"`  // full section index
	Remaining []string `json:"remaining,omitempty"` // sections omitted by the budget
}

// Render returns a playbook body honoring the section/budget/full options. Over
// budget, it returns the top sections (document order) that fit, plus the
// section index and the names of the omitted sections, with Truncated set.
func (p *Playbook) Render(opts RenderOptions) (Rendered, error) {
	budget := opts.Budget
	if budget <= 0 {
		budget = DefaultLoadBudget
	}
	r := Rendered{Name: p.Name, Color: p.Color, Summary: p.Summary, Sections: p.SectionNames()}

	if opts.Section != "" {
		s, ok := p.Section(opts.Section)
		if !ok {
			return Rendered{}, fmt.Errorf("section %q not found in %s (have: %s)", opts.Section, p.Name, strings.Join(p.SectionNames(), ", "))
		}
		r.Section = s.Heading
		r.Body = s.Body
		r.Tokens = EstimateTokens(s.Body)
		return r, nil
	}

	full := p.body
	if opts.Full || EstimateTokens(full) <= budget {
		r.Body = full
		r.Tokens = EstimateTokens(full)
		return r, nil
	}

	// Over budget: accumulate whole blocks (an optional preamble, then each
	// section, in document order) until the next would overflow. If nothing
	// fits — a single huge block, or a body with no `##` headings — include a
	// character-truncated prefix of the first block so content is never lost
	// silently.
	var b strings.Builder
	used := 0
	for _, bl := range p.budgetBlocks(full) {
		cost := EstimateTokens(bl.text) + 1 // +1 for the trailing blank line
		if used+cost > budget {
			if used == 0 {
				t := truncateToTokens(bl.text, budget)
				b.WriteString(t)
				b.WriteString("\n\n")
				used += EstimateTokens(t)
			}
			r.Remaining = append(r.Remaining, bl.label)
			continue
		}
		b.WriteString(bl.text)
		b.WriteString("\n\n")
		used += cost
	}
	r.Body = strings.TrimRight(b.String(), "\n")
	r.Tokens = used
	r.Truncated = true
	return r, nil
}

type budgetBlock struct {
	label string
	text  string
}

// budgetBlocks returns the renderable blocks of a body in order: an optional
// preamble (text before the first `##`) labeled "(intro)", then each section
// as a re-headed block. full is the whole body passed in to avoid recomputing.
func (p *Playbook) budgetBlocks(full string) []budgetBlock {
	var out []budgetBlock
	if pre := preambleOf(full); pre != "" {
		out = append(out, budgetBlock{label: "(intro)", text: pre})
	}
	for _, s := range p.Sections {
		out = append(out, budgetBlock{label: s.Heading, text: "## " + s.Heading + "\n\n" + s.Body})
	}
	if len(out) == 0 {
		// No preamble and no sections: fall back to the whole body as one block.
		out = append(out, budgetBlock{label: "(body)", text: strings.TrimRight(full, "\n")})
	}
	return out
}

// preambleOf returns the body text before the first level-2 heading, or the
// whole body when there is none.
func preambleOf(body string) string {
	heads := mdparse.Headings(body)
	first := 0
	found := false
	for _, h := range heads {
		if h.Level == 2 {
			first = h.Line // 1-based
			found = true
			break
		}
	}
	lines := mdparse.Lines(body)
	if !found {
		return strings.TrimRight(strings.Join(lines, "\n"), "\n")
	}
	return strings.TrimRight(strings.Join(lines[:first-1], "\n"), "\n")
}

// truncateToTokens returns a prefix of s within about budget tokens, with an
// ellipsis marker, cutting on a rune boundary.
func truncateToTokens(s string, budget int) string {
	if budget <= 0 {
		return ""
	}
	maxRunes := budget*4 - 1 // leave room for the marker
	if maxRunes < 1 {
		maxRunes = 1
	}
	rs := []rune(s)
	if len(rs) <= maxRunes {
		return s
	}
	return string(rs[:maxRunes]) + "…"
}

// EstimateTokens is a cheap, provider-agnostic heuristic: about one token per
// four characters (runes), rounded up. It is used only to cap load output, not
// for billing.
func EstimateTokens(s string) int {
	n := utf8.RuneCountInString(s)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4
}
