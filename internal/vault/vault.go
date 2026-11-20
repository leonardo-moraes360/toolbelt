package vault

import "sort"

// NoteInfo is a note in command output: tb-generated and validated fields
// outside, note data inside Data (R11, R18).
type NoteInfo struct {
	Path       string   `json:"path"`
	File       string   `json:"file"`
	Type       string   `json:"type"`
	Source     string   `json:"source"`
	Confidence *float64 `json:"confidence,omitempty"`
	Created    string   `json:"created,omitempty"`
	Updated    string   `json:"updated,omitempty"`
	Data       NoteData `json:"data"`
	Warnings   []Issue  `json:"warnings,omitempty"`
}

// NoteData is the note-derived part of NoteInfo, redacted (R15).
type NoteData struct {
	Title   string   `json:"title"`
	Tags    []string `json:"tags,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
	Body    string   `json:"body,omitempty"`
}

func info(n *Note, withBody bool) NoteInfo {
	ni := NoteInfo{Path: n.Path, File: n.File, Type: n.Type, Source: n.Source, Confidence: n.Confidence,
		Created: n.Created, Updated: n.Updated,
		Data:     NoteData{Title: Redact(n.Title), Tags: redactAll(n.Tags), Aliases: redactAll(n.Aliases)},
		Warnings: redactIssues(n.Issues)}
	if withBody {
		ni.Data.Body = Redact(n.Body)
	}
	return ni
}

func redactIssues(list []Issue) []Issue {
	if len(list) == 0 {
		return nil
	}
	out := make([]Issue, len(list))
	for i, is := range list {
		out[i] = is
		if is.Data != nil {
			d := IssueData{Target: Redact(is.Data.Target), Detail: Redact(is.Data.Detail)}
			out[i].Data = &d
		}
	}
	return out
}

func redactLinks(list []Link) []Link {
	for i := range list {
		d := &list[i].Data
		d.Target, d.Fragment, d.Label = Redact(d.Target), Redact(d.Fragment), Redact(d.Label)
	}
	return list
}

// Filter narrows List (R11); empty fields match everything.
type Filter struct {
	Type, Tag, Source string
}

func (f Filter) match(n *Note) bool {
	return (f.Type == "" || n.Type == f.Type) && (f.Source == "" || n.Source == f.Source) && (f.Tag == "" || contains(n.Tags, f.Tag))
}

// ListResult is tb vault list.
type ListResult struct {
	Notes []NoteInfo `json:"notes"`
}

// List is tb vault list (R11), sorted by path.
func (v *Vault) List(f Filter) (*ListResult, error) {
	sc, err := v.scan()
	if err != nil {
		return nil, err
	}
	out := &ListResult{Notes: []NoteInfo{}}
	for _, n := range sc.notes {
		if f.match(n) {
			out.Notes = append(out.Notes, info(n, false))
		}
	}
	return out, nil
}

// ShowResult is tb vault show (R12).
type ShowResult struct {
	Note      NoteInfo   `json:"note"`
	Links     []Link     `json:"links"`
	Backlinks []Backlink `json:"backlinks"`
}

// Show is tb vault show <target>.
func (v *Vault) Show(target string) (*ShowResult, error) {
	ix, n, err := v.resolveTarget(target, true)
	if err != nil {
		return nil, err
	}
	return &ShowResult{Note: info(n, true), Links: redactLinks(ix.links(n)), Backlinks: ix.backlinksOf(n)}, nil
}

// LinksResult is tb vault links (R14).
type LinksResult struct {
	Path  string `json:"path"`
	Links []Link `json:"links"`
}

// Links is tb vault links <target>.
func (v *Vault) Links(target string) (*LinksResult, error) {
	ix, n, err := v.resolveTarget(target, false)
	if err != nil {
		return nil, err
	}
	return &LinksResult{Path: n.Path, Links: redactLinks(ix.links(n))}, nil
}

// BacklinksResult is tb vault backlinks (R14).
type BacklinksResult struct {
	Path      string     `json:"path"`
	Backlinks []Backlink `json:"backlinks"`
}

// Backlinks is tb vault backlinks <target>.
func (v *Vault) Backlinks(target string) (*BacklinksResult, error) {
	ix, n, err := v.resolveTarget(target, true)
	if err != nil {
		return nil, err
	}
	return &BacklinksResult{Path: n.Path, Backlinks: ix.backlinksOf(n)}, nil
}

func (v *Vault) resolveTarget(target string, backlinks bool) (*index, *Note, error) {
	sc, err := v.scan()
	if err != nil {
		return nil, nil, err
	}
	ix := newIndex(sc)
	n, err := ix.find(target)
	if err != nil {
		return nil, nil, err
	}
	if backlinks {
		ix.computeBacklinks()
	}
	return ix, n, nil
}

func (ix *index) backlinksOf(n *Note) []Backlink {
	out := append([]Backlink{}, ix.backlinks[n]...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})
	return out
}
