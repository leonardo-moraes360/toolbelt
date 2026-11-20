package graph

import (
	"path/filepath"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/vault"
)

// Vault node and edge types (docs/specs/vault.md, R22–R24).
const (
	NodeNote   NodeType = "note"
	NodeTag    NodeType = "tag"
	EdgeTagged EdgeType = "tagged"
)

// vaultEnabled is R20: the vault pass runs when vault or markdown is in
// Docs, so an old config listing only markdown keeps its notes.
func vaultEnabled(opts *BuildOptions) bool {
	return containsFold(opts.Docs, "vault") || containsFold(opts.Docs, "markdown")
}

// resolveVault finds the vault of root (vault.Open from root, which reads
// root's own .toolbelt/config.yaml); nil when there is none or the
// .toolbelt/ found is not root's own. index reports whether the vault pass
// runs: vault indexing enabled and vault.graph not false. Either way the
// vault stays out of the per-file indexers, which follow symlinks (R21).
func resolveVault(root string, opts *BuildOptions) (v *vault.Vault, index bool) {
	v, err := vault.Open(root)
	if err != nil || v.Root != root {
		return nil, false
	}
	return v, vaultEnabled(opts) && v.Cfg.Graph
}

// inVault reports whether rel (root-relative) is inside the vault dir.
func inVault(vaultRel, rel string) bool {
	if vaultRel == "" {
		return false
	}
	r := filepath.ToSlash(rel)
	return r == vaultRel || strings.HasPrefix(r, vaultRel+"/")
}

// indexVault is the vault pass after the walk (R20–R24): one scan of the
// vault through internal/vault (R21: no symlinks, FIFOs or oversized
// files), notes as note: nodes, headings as sections, resolved wikilinks
// and tags as edges, and the body's Markdown links and mentions with the
// note as their source.
func indexVault(v *vault.Vault, nodes map[string]Node, edges *[]Edge) {
	data, err := v.Graph()
	if err != nil {
		return
	}
	for i := range data.Notes {
		n := &data.Notes[i]
		id := "note:" + n.Path
		nodes[id] = Node{ID: id, Type: NodeNote, Path: n.File, Title: n.Title, Source: n.Source, Confidence: n.Confidence}
		for _, h := range n.Headings {
			sec := "section:" + n.File + "#" + slugify(h.Text)
			nodes[sec] = Node{ID: sec, Type: NodeSection, Path: n.File, Title: h.Text, Source: "extracted", Confidence: 0.9}
			*edges = append(*edges, Edge{SourceID: id, TargetID: sec, Type: EdgeDefines, Source: "extracted", Confidence: 0.9})
		}
		for _, l := range n.Links {
			target := "note:" + l.ToPath
			if l.Section != "" {
				target = "section:" + l.ToFile + "#" + l.Section
			}
			*edges = append(*edges, Edge{SourceID: id, TargetID: target, Type: EdgeLinks, Source: "extracted", Confidence: 0.9})
		}
		for _, t := range n.Tags {
			tag := "tag:" + t
			nodes[tag] = Node{ID: tag, Type: NodeTag, Title: t, Source: "extracted", Confidence: 1.0}
			*edges = append(*edges, Edge{SourceID: id, TargetID: tag, Type: EdgeTagged, Source: "extracted", Confidence: 1.0})
		}
		*edges = append(*edges, docLinkEdges(id, n.File, collectMDLinks(n.Body))...)
		*edges = append(*edges, docMentionEdges(id, n.Body)...)
	}
}

// vaultHidden is a vault entry the scan skips: a name starting with "."
// (.obsidian/, .trash/), so Obsidian's own state never makes the index stale.
func vaultHidden(vaultRel, rel string) bool {
	if !inVault(vaultRel, rel) {
		return false
	}
	inner := strings.TrimPrefix(filepath.ToSlash(rel), vaultRel)
	for _, part := range strings.Split(strings.Trim(inner, "/"), "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}
