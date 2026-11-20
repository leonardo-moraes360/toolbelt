package vault

import "github.com/leonardo-moraes360/toolbelt/internal/mdparse"

// GraphNote is a note as the graph sees it (docs/specs/vault.md, R22–R24):
// titles redacted, links already resolved by R8.
type GraphNote struct {
	Path       string // note path
	File       string // repository-relative file
	Title      string // redacted
	Source     string
	Confidence float64 // manual 1.0, extracted 0.9, inferred its confidence
	Tags       []string
	Headings   []mdparse.Heading // titles redacted
	Links      []GraphLink
	Body       string // redacted, for Markdown links and mentions
}

// GraphLink is a resolved wikilink (state ok or broken_fragment).
type GraphLink struct {
	ToPath  string // target note path
	ToFile  string // target note file
	Section string // heading slug when the fragment matches one, else ""
}

// GraphData is the vault pass input of tb graph index.
type GraphData struct {
	Rel   string // the vault directory, repository-relative
	Notes []GraphNote
}

// Graph scans the vault once and returns its notes for the graph; files the
// scan does not read (symlinks, FIFOs, oversized) never reach the graph (R21).
func (v *Vault) Graph() (*GraphData, error) {
	sc, err := v.scan()
	if err != nil {
		return nil, err
	}
	ix := newIndex(sc)
	out := &GraphData{Rel: v.Rel}
	for _, n := range sc.notes {
		gn := GraphNote{Path: n.Path, File: n.File, Title: Redact(n.Title), Source: n.Source,
			Confidence: confidenceOf(n), Tags: n.Tags, Body: Redact(n.Body)}
		for _, h := range n.Headings {
			h.Text = Redact(h.Text)
			gn.Headings = append(gn.Headings, h)
		}
		for _, w := range n.Links {
			l, to := ix.resolve(n, w)
			// Unparsed notes have no graph node; a plain self-link adds nothing.
			if to == nil || to.unread || to == n && w.Fragment == "" {
				continue
			}
			gl := GraphLink{ToPath: to.Path, ToFile: to.File}
			if l.State == StateOK && w.Fragment != "" {
				gl.Section = mdparse.Slugify(w.Fragment)
			}
			gn.Links = append(gn.Links, gl)
		}
		out.Notes = append(out.Notes, gn)
	}
	return out, nil
}

func confidenceOf(n *Note) float64 {
	switch {
	case n.Source == SourceInferred && n.Confidence != nil:
		return *n.Confidence
	case n.Source == SourceExtracted:
		return 0.9
	case n.Source == SourceInferred:
		return 0.5
	}
	return 1.0
}
