// Package secplay is the AppSec red/blue playbook engine behind `tb red` and
// `tb blue` (docs/specs/appsec-playbooks.md). It owns a native, embedded corpus
// of offensive (red) and defensive (blue) playbooks, paired by vulnerability
// class, and serves them compact in search and under a token budget on load.
// It never executes anything from a playbook body; blue checks are grep/regex
// detectors run only by the explicit `tb blue check` path (see checks.go).
package secplay

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/leonardo-moraes360/toolbelt/internal/mdparse"
)

// Color is the playbook side: offensive or defensive.
type Color string

const (
	// Red is the offensive side.
	Red Color = "red"
	// Blue is the defensive side.
	Blue Color = "blue"
)

// Valid reports whether c is a known color.
func (c Color) Valid() bool { return c == Red || c == Blue }

// Section is one `##`-level block of a playbook body: the unit of budget
// slicing. Body excludes the heading line itself.
type Section struct {
	Heading string `json:"heading"`
	Slug    string `json:"slug"`
	Body    string `json:"body"`
}

// frontmatter is the YAML header of a playbook.
type frontmatter struct {
	Name     string   `yaml:"name"`
	Color    Color    `yaml:"color"`
	Area     string   `yaml:"area"`
	Class    string   `yaml:"class"`
	Mirror   string   `yaml:"mirror"`
	Triggers []string `yaml:"triggers"`
	Summary  string   `yaml:"summary"`
	Checks   []Check  `yaml:"checks"`
}

// Playbook is one parsed red or blue playbook.
type Playbook struct {
	Name     string    `json:"name"`
	Color    Color     `json:"color"`
	Area     string    `json:"area"`
	Class    string    `json:"class"`
	Mirror   string    `json:"mirror"`
	Triggers []string  `json:"triggers"`
	Summary  string    `json:"summary"`
	Checks   []Check   `json:"checks,omitempty"`
	Sections []Section `json:"sections"`

	// body is the Markdown after the frontmatter, retained for full-document
	// rendering. Lowercase: not serialized, used by search/render.
	body string
}

// parsePlaybook parses one SKILL/PLAYBOOK Markdown file. path is only used in
// error messages.
func parsePlaybook(path, src string) (*Playbook, error) {
	fm, ok, err := mdparse.SplitFrontmatter(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if !ok {
		return nil, fmt.Errorf("%s: missing frontmatter", path)
	}
	var meta frontmatter
	if err := yaml.Unmarshal([]byte(fm.YAML), &meta); err != nil {
		return nil, fmt.Errorf("%s: frontmatter: %w", path, err)
	}
	if meta.Name == "" {
		return nil, fmt.Errorf("%s: frontmatter name is required", path)
	}
	if !meta.Color.Valid() {
		return nil, fmt.Errorf("%s: frontmatter color must be red or blue, got %q", path, meta.Color)
	}
	if meta.Class == "" {
		return nil, fmt.Errorf("%s: frontmatter class is required", path)
	}

	body := bodyAfterFrontmatter(src, fm.EndLine)
	pb := &Playbook{
		Name:     meta.Name,
		Color:    meta.Color,
		Area:     meta.Area,
		Class:    meta.Class,
		Mirror:   meta.Mirror,
		Triggers: meta.Triggers,
		Summary:  meta.Summary,
		Checks:   meta.Checks,
		Sections: splitSections(body),
		body:     body,
	}
	for i := range pb.Checks {
		if pb.Checks[i].Class == "" {
			pb.Checks[i].Class = pb.Class
		}
	}
	return pb, nil
}

// bodyAfterFrontmatter returns src with the frontmatter (lines 1..endLine)
// removed. endLine is 1-based and is the closing-marker line; the body starts
// on the next line.
func bodyAfterFrontmatter(src string, endLine int) string {
	lines := mdparse.Lines(src)
	if endLine >= len(lines) {
		return ""
	}
	return strings.Join(lines[endLine:], "\n")
}

// splitSections carves body into `##`-level sections. Content before the first
// `##` heading, and headings deeper than level 2, are folded into the current
// section. A deeper heading keeps its own `#` markers in the section body.
func splitSections(body string) []Section {
	lines := mdparse.Lines(body)
	heads := mdparse.Headings(body)
	// Keep only level-2 headings as section boundaries.
	var bounds []mdparse.Heading
	for _, h := range heads {
		if h.Level == 2 {
			bounds = append(bounds, h)
		}
	}
	if len(bounds) == 0 {
		return nil
	}
	var out []Section
	for i, h := range bounds {
		start := h.Line // 1-based heading line; body begins next line (index h.Line)
		end := len(lines)
		if i+1 < len(bounds) {
			end = bounds[i+1].Line - 1 // up to line before next heading (0-based exclusive)
		}
		var seg []string
		if start < end {
			seg = lines[start:end]
		}
		out = append(out, Section{
			Heading: h.Text,
			Slug:    h.Slug,
			Body:    strings.TrimRight(strings.Join(seg, "\n"), "\n"),
		})
	}
	return out
}

// Section returns the section whose heading or slug matches q (case-insensitive,
// exact on slug or heading, else prefix on heading). ok is false when none match.
func (p *Playbook) Section(q string) (Section, bool) {
	qTrim := strings.TrimSpace(q)
	if qTrim == "" {
		return Section{}, false
	}
	want := strings.ToLower(qTrim)
	wantSlug := mdparse.Slugify(q)
	for _, s := range p.Sections {
		if s.Slug == wantSlug || strings.EqualFold(s.Heading, qTrim) {
			return s, true
		}
	}
	for _, s := range p.Sections {
		if strings.HasPrefix(strings.ToLower(s.Heading), want) {
			return s, true
		}
	}
	return Section{}, false
}

// SectionNames lists the section headings in document order.
func (p *Playbook) SectionNames() []string {
	out := make([]string, 0, len(p.Sections))
	for _, s := range p.Sections {
		out = append(out, s.Heading)
	}
	return out
}

// Body returns the full Markdown body (after the frontmatter).
func (p *Playbook) Body() string { return p.body }

// sortedNames returns the keys of m sorted, for deterministic listing.
func sortedNames(m map[string]*Playbook) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
