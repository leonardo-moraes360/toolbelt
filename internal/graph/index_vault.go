package graph

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/leonardo-moraes360/toolbelt/internal/log"
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

// vaultState is the vault of one build, resolved once (R20).
type vaultState struct {
	v     *vault.Vault
	dir   os.FileInfo // the vault directory, for os.SameFile in the walks
	index bool        // the note pass runs
	cfg   string      // the vault config, part of the fingerprint
}

// resolveVault finds the vault with vault.Open(root). The walks skip the
// vault directory whenever it resolves (compared with os.SameFile, so case
// or a symlinked spelling cannot slip it past the skip), because the
// per-file indexers follow symlinks (R21). The note pass runs only for
// root's own .toolbelt/, with vault indexing enabled and vault.graph true.
func resolveVault(root string, opts *BuildOptions) *vaultState {
	v, err := vault.Open(root)
	if err != nil {
		var e *vault.Error
		if errors.As(err, &e) && e.Reason != "no_toolbelt_dir" && e.Reason != "vault_not_found" {
			log.Warn("vault not indexed", "reason", e.Reason, "error", err)
		}
		return nil
	}
	fi, err := os.Stat(v.Dir)
	if err != nil {
		return nil
	}
	return &vaultState{v: v, dir: fi, index: v.Root == root && vaultEnabled(opts) && v.Cfg.Graph,
		cfg: fmt.Sprintf("%+v", v.Cfg)}
}

// skipDir reports whether the generic walks must not enter dir: the
// root's .toolbelt/ (local state, and the default vault even when its
// config does not parse) or the vault directory itself.
func (vs *vaultState) skipDir(root, path string, info os.FileInfo) bool {
	if rel, err := filepath.Rel(root, path); err == nil && filepath.ToSlash(rel) == ".toolbelt" {
		return true
	}
	return vs != nil && os.SameFile(info, vs.dir)
}

// indexVault is the vault pass after the walk (R20–R24): one scan of the
// vault through internal/vault (R21: no symlinks, FIFOs or oversized
// files), notes as note: nodes, headings as sections, resolved wikilinks
// and tags as edges, and the body's Markdown links and mentions with the
// note as their source.
func indexVault(v *vault.Vault, nodes map[string]Node, edges *[]Edge) {
	data, err := v.Graph()
	if err != nil {
		log.Warn("vault not indexed", "error", err)
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
		seen := map[string]bool{}
		for _, l := range n.Links {
			target := "note:" + l.ToPath
			if l.Heading != "" {
				target = "section:" + l.ToFile + "#" + slugify(l.Heading)
			}
			if seen[target] {
				continue
			}
			seen[target] = true
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
	// After the walk and the pass: links from docs/ and from notes alike.
	remapNoteFiles(data, *edges)
}

// remapNoteFiles points file: targets that are vault notes at their note:
// node, since a note has no file: node (R22): Markdown links and mentions,
// from other notes or from docs/, keep resolving.
func remapNoteFiles(data *vault.GraphData, edges []Edge) {
	byFile := map[string]string{}
	for i := range data.Notes {
		byFile["file:"+data.Notes[i].File] = "note:" + data.Notes[i].Path
	}
	for i := range edges {
		if id, ok := byFile[edges[i].TargetID]; ok {
			edges[i].TargetID = id
		}
	}
}
