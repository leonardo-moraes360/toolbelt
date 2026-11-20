package vault

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/leonardo-moraes360/toolbelt/internal/mdparse"
)

// Sources of a note (frontmatter source, ADR-006 provenance).
const (
	SourceManual    = "manual"
	SourceExtracted = "extracted"
	SourceInferred  = "inferred"
)

// Issue levels.
const (
	LevelError   = "error"
	LevelWarning = "warning"
)

var (
	reTag      = regexp.MustCompile(`^[a-z0-9][a-z0-9/_-]*$`)
	reYAMLLine = regexp.MustCompile(`line (\d+)`)
)

// Issue is a check finding. Everything derived from note content sits in
// Data (R18); the other fields are tb-generated.
type Issue struct {
	Code  string     `json:"code"`
	Level string     `json:"level"`
	File  string     `json:"file"` // relative to the repository root
	Line  int        `json:"line,omitempty"`
	Col   int        `json:"col,omitempty"`
	Data  *IssueData `json:"data,omitempty"`
}

// IssueData is the note-derived part of an Issue.
type IssueData struct {
	Target string `json:"target,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// Note is one parsed note (R4–R7).
type Note struct {
	Path       string // relative to the vault, "/", without ".md"
	File       string // relative to the repository root, "/"
	Name       string // file name without ".md"
	Title      string
	Type       string
	Source     string
	Tags       []string
	Aliases    []string
	Confidence *float64
	Created    string
	Updated    string
	Body       string // text after the frontmatter
	Headings   []mdparse.Heading
	Links      []mdparse.Wikilink
	Issues     []Issue

	line    int    // the frontmatter line being parsed
	unread  bool   // too large or not regular: linkable, never parsed
	rawHead string // the source, for line arithmetic (search)
}

// parseNote parses src as the note at file (repository-relative) with note
// path p. It never fails: problems become Issues and the affected fields
// keep their defaults (R4).
func parseNote(file, p, src string, types []string) *Note {
	n := &Note{Path: p, File: file, Name: p[strings.LastIndex(p, "/")+1:], Type: "fact", Source: SourceManual, Body: src, rawHead: src}
	fm, ok, err := mdparse.SplitFrontmatter(src)
	switch {
	case err != nil:
		n.issue("bad_frontmatter", LevelError, 1, "", "unterminated frontmatter")
	case ok:
		n.Body = strings.Join(bodyLines(src, fm.EndLine), "\n")
		n.frontmatter(fm.YAML, types)
	}
	n.Headings = mdparse.Headings(src)
	n.Links = mdparse.Wikilinks(src)
	if n.Title == "" {
		n.Title = n.Name
		for _, h := range n.Headings {
			if h.Level == 1 {
				n.Title = h.Text
				break
			}
		}
	}
	return n
}

func bodyLines(src string, endLine int) []string {
	ls := mdparse.Lines(src)
	if endLine >= len(ls) {
		return nil
	}
	return ls[endLine:]
}

func (n *Note) issue(code, level string, line int, target, detail string) {
	is := Issue{Code: code, Level: level, File: n.File, Line: line}
	if target != "" || detail != "" {
		is.Data = &IssueData{Target: target, Detail: detail}
	}
	n.Issues = append(n.Issues, is)
}

func (n *Note) frontmatter(src string, types []string) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		line := 1
		if m := reYAMLLine.FindStringSubmatch(err.Error()); m != nil {
			l, _ := strconv.Atoi(m[1])
			line = l + 1 // YAML line 1 is file line 2
		}
		n.issue("bad_frontmatter", LevelError, line, "", err.Error())
		return
	}
	if len(doc.Content) == 0 {
		return // empty frontmatter
	}
	m := doc.Content[0]
	if m.Kind != yaml.MappingNode {
		n.issue("bad_frontmatter", LevelError, m.Line+1, "", "frontmatter is not a mapping")
		return
	}
	lines := map[string]int{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i], m.Content[i+1]
		line := k.Line + 1 // YAML line 1 is file line 2
		lines[k.Value] = line
		n.field(k.Value, v, line, types)
	}
	switch {
	case n.Source == SourceInferred && n.Confidence == nil:
		n.issue("missing_confidence", LevelError, lines["source"], "", "")
	case n.Source != SourceInferred && n.Confidence != nil:
		n.issue("unused_confidence", LevelWarning, lines["confidence"], "", "")
	}
}

// fieldParsers handle the recognized frontmatter keys (R4); unknown keys
// are ignored.
var fieldParsers = map[string]func(n *Note, v *yaml.Node, types []string) string{
	"title":      (*Note).parseTitle,
	"type":       (*Note).parseType,
	"tags":       (*Note).parseTags,
	"aliases":    (*Note).parseAliases,
	"source":     (*Note).parseSource,
	"confidence": (*Note).parseConfidence,
	"created":    func(n *Note, v *yaml.Node, _ []string) string { return parseDate(v, &n.Created) },
	"updated":    func(n *Note, v *yaml.Node, _ []string) string { return parseDate(v, &n.Updated) },
}

// field applies one key; a parser returns the bad_frontmatter detail, or "".
func (n *Note) field(key string, v *yaml.Node, line int, types []string) {
	parse, ok := fieldParsers[key]
	if !ok {
		return
	}
	n.line = line
	if detail := parse(n, v, types); detail != "" {
		n.issue("bad_frontmatter", LevelError, line, "", key+": "+detail)
	}
}

func (n *Note) parseTitle(v *yaml.Node, _ []string) string {
	s, ok := str(v)
	if !ok {
		return "must be a string"
	}
	n.Title = strings.TrimSpace(s)
	return ""
}

func (n *Note) parseType(v *yaml.Node, types []string) string {
	s, ok := str(v)
	switch {
	case !ok:
		return "must be a string"
	case !contains(types, s):
		return "not one of vault.types"
	}
	n.Type = s
	return ""
}

func (n *Note) parseTags(v *yaml.Node, _ []string) string {
	list, ok := strs(v)
	if !ok {
		return "must be a string or a list of strings"
	}
	for _, t := range list {
		if reTag.MatchString(t) {
			n.Tags = append(n.Tags, t)
		} else {
			n.issue("bad_tag", LevelWarning, n.line, t, "")
		}
	}
	return ""
}

func (n *Note) parseAliases(v *yaml.Node, _ []string) string {
	list, ok := strs(v)
	if !ok {
		return "must be a string or a list of strings"
	}
	n.Aliases = list
	return ""
}

func (n *Note) parseSource(v *yaml.Node, _ []string) string {
	s, ok := str(v)
	if !ok || (s != SourceManual && s != SourceExtracted && s != SourceInferred) {
		return "must be manual, extracted or inferred"
	}
	n.Source = s
	return ""
}

func (n *Note) parseConfidence(v *yaml.Node, _ []string) string {
	f, err := strconv.ParseFloat(v.Value, 64)
	if v.Kind != yaml.ScalarNode || (v.Tag != "!!int" && v.Tag != "!!float") || err != nil || f < 0 || f > 1 {
		return "must be a number in [0, 1]"
	}
	n.Confidence = &f
	return ""
}

func parseDate(v *yaml.Node, dst *string) string {
	if !validDate(v) {
		return "must be RFC 3339 or YYYY-MM-DD"
	}
	*dst = v.Value
	return ""
}

func str(v *yaml.Node) (string, bool) {
	if v.Kind != yaml.ScalarNode || v.Tag != "!!str" {
		return "", false
	}
	return v.Value, true
}

// strs accepts a single string or a sequence of strings.
func strs(v *yaml.Node) ([]string, bool) {
	if s, ok := str(v); ok {
		return []string{s}, true
	}
	if v.Kind != yaml.SequenceNode {
		return nil, false
	}
	out := make([]string, 0, len(v.Content))
	for _, c := range v.Content {
		s, ok := str(c)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func validDate(v *yaml.Node) bool {
	if v.Kind != yaml.ScalarNode || (v.Tag != "!!str" && v.Tag != "!!timestamp") {
		return false
	}
	if _, err := time.Parse(time.RFC3339, v.Value); err == nil {
		return true
	}
	_, err := time.Parse("2006-01-02", v.Value)
	return err == nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
