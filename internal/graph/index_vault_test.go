package graph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func vaultRepo(t *testing.T, cfg string) string {
	t.Helper()
	root := t.TempDir()
	v := filepath.Join(root, ".toolbelt", "vault")
	mustWrite(t, filepath.Join(root, ".toolbelt", "config.yaml"), cfg)
	mustWrite(t, filepath.Join(v, "a.md"), "---\ntags: [db]\nsource: inferred\nconfidence: 0.6\n---\n# Alpha\n\n## Plan\n\nSee [[b]], [[b#Details]], [[missing]] and src/lib.go.\n")
	mustWrite(t, filepath.Join(v, "dir", "b.md"), "# Beta ghp_"+strings.Repeat("q", 30)+"\n\n## Details\n")
	mustWrite(t, filepath.Join(v, ".obsidian", "w.md"), "[[a]]")
	mustWrite(t, filepath.Join(root, "src", "lib.go"), "package lib\n")
	return root
}

func edgeExists(idx *Index, from, to string, typ EdgeType) bool {
	for _, e := range idx.Edges {
		if e.SourceID == from && e.TargetID == to && e.Type == typ {
			return true
		}
	}
	return false
}

func nodeByID(idx *Index, id string) (Node, bool) {
	for _, n := range idx.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

// TestVaultPass is R20–R24 with an old config (docs: markdown only).
func TestVaultPass(t *testing.T) {
	root := vaultRepo(t, "graph:\n  docs: [markdown]\n")
	outside := filepath.Join(t.TempDir(), "secret.md")
	mustWrite(t, outside, "# TOP SECRET\n")
	if err := os.Symlink(outside, filepath.Join(root, ".toolbelt", "vault", "link.md")); err != nil {
		t.Fatal(err)
	}
	opts := DefaultBuildOptions()
	opts.Languages, opts.Docs = []string{"go"}, []string{"markdown"}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := nodeByID(idx, "note:a")
	if !ok || a.Type != NodeNote || a.Title != "Alpha" || a.Source != "inferred" || a.Confidence != 0.6 {
		t.Fatalf("note a: %+v", a)
	}
	if _, ok := nodeByID(idx, "file:.toolbelt/vault/a.md"); ok {
		t.Fatal("a note is also a file: node")
	}
	if b, _ := nodeByID(idx, "note:dir/b"); strings.Contains(b.Title, "ghp_") {
		t.Fatalf("title not redacted: %q", b.Title)
	}
	checkVaultEdgesAndLeaks(t, idx)
}

func TestVaultPassOptOut(t *testing.T) {
	root := vaultRepo(t, "vault:\n  graph: false\n")
	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := nodeByID(idx, "note:a"); ok {
		t.Fatal("vault.graph: false still indexed notes")
	}
	if _, ok := nodeByID(idx, "file:.toolbelt/vault/a.md"); ok {
		t.Fatal("opted-out vault fell back to the markdown indexer")
	}
}

func TestVaultFingerprint(t *testing.T) {
	root := vaultRepo(t, "")
	opts := DefaultBuildOptions()
	fp1, err := SourceFingerprint(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, ".toolbelt", "vault", ".obsidian", "workspace.json"), "{}")
	if fp2, _ := SourceFingerprint(root, &opts); fp2 != fp1 {
		t.Fatal(".obsidian changes made the index stale")
	}
	mustWrite(t, filepath.Join(root, ".toolbelt", "vault", "new.md"), "# New\n")
	if fp3, _ := SourceFingerprint(root, &opts); fp3 == fp1 {
		t.Fatal("a new note did not change the fingerprint")
	}
}

func TestVaultExplain(t *testing.T) {
	root := vaultRepo(t, "")
	s := NewStore(filepath.Join(t.TempDir(), "graph.json"))
	if _, err := s.Rebuild(root); err != nil {
		t.Fatal(err)
	}
	ex, err := s.Explain("note:a", 1)
	if err != nil || len(ex) != 1 {
		t.Fatalf("explain: %+v %v", ex, err)
	}
	if _, err := s.ShortestPath("note:a", "tag:db", 3); err != nil {
		t.Fatalf("path: %v", err)
	}
}

func checkVaultEdgesAndLeaks(t *testing.T, idx *Index) {
	t.Helper()
	for _, e := range [][3]string{
		{"note:a", "note:dir/b", string(EdgeLinks)},
		{"note:a", "section:.toolbelt/vault/dir/b.md#details", string(EdgeLinks)},
		{"note:a", "tag:db", string(EdgeTagged)},
		{"note:a", "section:.toolbelt/vault/a.md#plan", string(EdgeDefines)},
	} {
		if !edgeExists(idx, e[0], e[1], EdgeType(e[2])) {
			t.Errorf("missing edge %v", e)
		}
	}
	for _, n := range idx.Nodes {
		if strings.Contains(n.Title, "TOP SECRET") || strings.HasPrefix(n.ID, "note:.obsidian") || n.ID == "note:missing" {
			t.Fatalf("unexpected node %+v", n)
		}
	}
}

func hasNodeID(idx *Index, id string) bool {
	_, ok := nodeByID(idx, id)
	return ok
}

// TestVaultBrokenConfigStillSkipped: an unparsable vault config must not
// hand .toolbelt/vault to the symlink-following markdown indexer (R21).
func TestVaultBrokenConfigStillSkipped(t *testing.T) {
	root := vaultRepo(t, "vault:\n  max_note_bytes: 0\n")
	outside := filepath.Join(t.TempDir(), "private.md")
	mustWrite(t, outside, "# PRIVATE HEADING\n")
	if err := os.Symlink(outside, filepath.Join(root, ".toolbelt", "vault", "x.md")); err != nil {
		t.Fatal(err)
	}
	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range idx.Nodes {
		if strings.Contains(n.Title, "PRIVATE") || strings.HasPrefix(n.Path, ".toolbelt/") {
			t.Fatalf("vault content indexed with a broken config: %+v", n)
		}
	}
}

// TestVaultSymlinkedDirInsideRepo: vault.path is a symlink to a real
// directory in the repo; the real directory is skipped by the generic walk
// (os.SameFile) and indexed once as notes.
func TestVaultSymlinkedDirInsideRepo(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".toolbelt", "config.yaml"), "vault:\n  path: notes\n")
	mustWrite(t, filepath.Join(root, "real", "a.md"), "# A\n")
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "notes")); err != nil {
		t.Fatal(err)
	}
	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNodeID(idx, "note:a") || hasNodeID(idx, "file:real/a.md") {
		t.Fatalf("nodes %v", titles(idx, NodeNote))
	}
}

func TestVaultRedactionAndLinks(t *testing.T) {
	root := t.TempDir()
	v := filepath.Join(root, ".toolbelt", "vault")
	mustWrite(t, filepath.Join(root, ".toolbelt", "config.yaml"), "")
	mustWrite(t, filepath.Join(v, "a.md"), "## password: hunter2abc\n\n## Safe\n")
	mustWrite(t, filepath.Join(v, "b.md"), "---\ntags: [sk-"+strings.Repeat("t", 24)+", ok]\n---\n[[a#password: hunter2abc]] [[a#Safe]] [[a]] [[a]] and [md](a.md)\n")
	mustWrite(t, filepath.Join(root, "docs", "guide.md"), "# Guide\n\nSee [note](../.toolbelt/vault/a.md).\n")
	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	links := 0
	for _, e := range idx.Edges {
		if strings.Contains(e.TargetID, "hunter2") || strings.Contains(e.TargetID, "sk-ttt") {
			t.Fatalf("secret in an edge: %+v", e)
		}
		if e.SourceID == "note:b" && e.TargetID == "note:a" && e.Type == EdgeLinks {
			links++
		}
	}
	for _, n := range idx.Nodes {
		if strings.Contains(n.ID, "sk-ttt") || strings.Contains(n.Title, "hunter2") {
			t.Fatalf("secret in a node: %+v", n)
		}
	}
	if links != 2 { // [[a]] twice is one wikilink edge, plus the Markdown link remapped from file:
		t.Fatalf("note:b -> note:a links: %d", links)
	}
	if !edgeExists(idx, "note:b", "section:.toolbelt/vault/a.md#safe", EdgeLinks) || !edgeExists(idx, "file:docs/guide.md", "note:a", EdgeLinks) {
		t.Fatal("missing section link or docs link remapped to note:")
	}
}

func TestVaultFingerprintConfigAndToggle(t *testing.T) {
	root := vaultRepo(t, "")
	opts := DefaultBuildOptions()
	fp1, _ := SourceFingerprint(root, &opts)
	mustWrite(t, filepath.Join(root, ".toolbelt", "config.yaml"), "vault:\n  max_note_bytes: 1024\n")
	opts = DefaultBuildOptions()
	fp2, _ := SourceFingerprint(root, &opts)
	mustWrite(t, filepath.Join(root, ".toolbelt", "config.yaml"), "vault:\n  graph: false\n")
	opts = DefaultBuildOptions()
	fp3, _ := SourceFingerprint(root, &opts)
	if fp1 == fp2 || fp2 == fp3 || fp1 == fp3 {
		t.Fatalf("config changes not in the fingerprint: %s %s %s", fp1, fp2, fp3)
	}
}

// TestVaultUnderSubdirRoot: indexing a subdirectory whose vault is
// configured by the parent repository still skips the vault directory.
func TestVaultUnderSubdirRoot(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".toolbelt", "config.yaml"), "vault:\n  path: sub/notes\n")
	mustWrite(t, filepath.Join(root, "sub", "notes", "a.md"), "# A\n")
	mustWrite(t, filepath.Join(root, "sub", "doc.md"), "# Doc\n")
	idx, err := BuildFromRoot(filepath.Join(root, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	if hasNodeID(idx, "file:notes/a.md") || !hasNodeID(idx, "file:doc.md") {
		t.Fatalf("subdir root: %v", titles(idx, NodeFile))
	}
}
