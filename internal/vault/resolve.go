package vault

import (
	"path"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/mdparse"
)

// Link states (R14).
const (
	StateOK                = "ok"
	StateBroken            = "broken"
	StateAmbiguous         = "ambiguous"
	StateBrokenFragment    = "broken_fragment"
	StateAttachment        = "attachment"
	StateMissingAttachment = "missing_attachment"
)

// index resolves wikilink targets over one scan (R8).
type index struct {
	notes     []*Note
	byPath    map[string][]*Note // lower note path, and every "/"-suffix of it
	byName    map[string][]*Note // lower note name
	attPath   map[string][]string
	attName   map[string][]string
	headings  map[*Note]map[string]bool
	backlinks map[*Note][]Backlink
}

func newIndex(sc *scan) *index {
	ix := &index{notes: sc.notes, byPath: map[string][]*Note{}, byName: map[string][]*Note{},
		attPath: map[string][]string{}, attName: map[string][]string{}, headings: map[*Note]map[string]bool{}}
	for _, n := range sc.notes {
		for _, s := range pathKeys(n.Path) {
			ix.byPath[s] = append(ix.byPath[s], n)
		}
		ix.byName[strings.ToLower(n.Name)] = append(ix.byName[strings.ToLower(n.Name)], n)
		hs := map[string]bool{}
		for _, h := range n.Headings {
			hs[h.Slug] = true
		}
		ix.headings[n] = hs
	}
	for _, a := range sc.attachments {
		for _, s := range pathKeys(a) {
			ix.attPath[s] = append(ix.attPath[s], a)
		}
		ix.attName[strings.ToLower(path.Base(a))] = append(ix.attName[strings.ToLower(path.Base(a))], a)
	}
	return ix
}

// pathKeys are the lower-cased path and each of its suffixes that still
// contains a "/" (Obsidian's partial-path match: [[b/a]] finds x/b/a).
func pathKeys(p string) []string {
	p = strings.ToLower(p)
	keys := []string{p}
	for i := strings.IndexByte(p, '/'); i >= 0; {
		rest := p[i+1:]
		if !strings.Contains(rest, "/") {
			break
		}
		keys = append(keys, rest)
		j := strings.IndexByte(rest, '/')
		i = i + 1 + j
	}
	return keys
}

// notesFor is R8's note resolution of a non-empty target.
func (ix *index) notesFor(t string) []*Note {
	t = strings.ToLower(t)
	if strings.Contains(t, "/") {
		return ix.byPath[t]
	}
	return ix.byName[t]
}

func (ix *index) attachmentsFor(t string) []string {
	t = strings.ToLower(t)
	if strings.Contains(t, "/") {
		return ix.attPath[t]
	}
	return ix.attName[t]
}

// Link is one outgoing wikilink with its state; the target text is note
// data (R18).
type Link struct {
	Line  int      `json:"line"`
	Col   int      `json:"col"`
	Embed bool     `json:"embed,omitempty"`
	State string   `json:"state"`
	To    string   `json:"to,omitempty"` // the resolved note path or attachment
	Data  LinkData `json:"data"`
}

// LinkData is the note-derived part of a Link.
type LinkData struct {
	Target   string `json:"target,omitempty"`
	Fragment string `json:"fragment,omitempty"`
	Label    string `json:"label,omitempty"`
}

// Backlink is a note that links to another one.
type Backlink struct {
	Path string `json:"path"`
	File string `json:"file"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
}

// resolve gives the state of one wikilink of from (R8, R9). A target with
// no note candidate and an extension falls back to the vault's attachments,
// so a note named "v1.2" still wins over an attachment.
func (ix *index) resolve(from *Note, w mdparse.Wikilink) (Link, *Note) {
	l := Link{Line: w.Line, Col: w.Col, Embed: w.Embed, Data: LinkData{Target: w.Target, Fragment: w.Fragment, Label: w.Label}}
	var to *Note
	if w.Target == "" {
		to = from
	} else {
		cands := ix.notesFor(w.Target)
		switch {
		case len(cands) == 1:
			to = cands[0]
		case len(cands) > 1:
			l.State = StateAmbiguous
			return l, nil
		case path.Ext(w.Target) != "":
			ix.attachment(&l, w.Target)
			return l, nil
		default:
			l.State = StateBroken
			return l, nil
		}
	}
	l.To, l.State = to.Path, StateOK
	if w.Fragment != "" && !strings.HasPrefix(w.Fragment, "^") && !ix.headings[to][mdparse.Slugify(w.Fragment)] {
		l.State = StateBrokenFragment
	}
	return l, to
}

func (ix *index) attachment(l *Link, t string) {
	switch a := ix.attachmentsFor(t); len(a) {
	case 0:
		l.State = StateMissingAttachment
	case 1:
		l.State, l.To = StateAttachment, a[0]
	default:
		l.State = StateAmbiguous
	}
}

// links resolves every wikilink of n.
func (ix *index) links(n *Note) []Link {
	out := make([]Link, 0, len(n.Links))
	for _, w := range n.Links {
		l, _ := ix.resolve(n, w)
		out = append(out, l)
	}
	return out
}

// computeBacklinks fills backlinks once: notes sorted by path, then line,
// column (R14).
func (ix *index) computeBacklinks() {
	ix.backlinks = map[*Note][]Backlink{}
	for _, n := range ix.notes {
		for _, w := range n.Links {
			if _, to := ix.resolve(n, w); to != nil && to != n {
				ix.backlinks[to] = append(ix.backlinks[to], Backlink{Path: n.Path, File: n.File, Line: w.Line, Col: w.Col})
			}
		}
	}
}

// find resolves a command's <target>: a note path (with or without .md),
// else R8 (R12).
func (ix *index) find(target string) (*Note, error) {
	t := strings.TrimSpace(target)
	if len(t) >= 3 && strings.EqualFold(t[len(t)-3:], ".md") {
		t = t[:len(t)-3]
	}
	for _, n := range ix.notes {
		if strings.EqualFold(n.Path, t) {
			return n, nil
		}
	}
	switch c := ix.notesFor(t); len(c) {
	case 1:
		return c[0], nil
	case 0:
		return nil, errorf(ExitFail, "note_not_found", "no note matches %q", Redact(t))
	default:
		return nil, errorf(ExitFail, "ambiguous_link", "%d notes match %q: use the note path", len(c), Redact(t))
	}
}
