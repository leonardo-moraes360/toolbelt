package graph

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexMarkdownSectionsAndLinks(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "docs", "guide.md"), `# Guide

See [API](./api.md#overview) and `+"`Scale`"+`.

Also touches src/lib.go.
`)
	mustWrite(t, filepath.Join(root, "docs", "api.md"), `# Overview

Details.
`)
	mustWrite(t, filepath.Join(root, "src", "lib.go"), "package lib\nfunc Scale(v int) int { return v }\n")

	opts := DefaultBuildOptions()
	opts.Languages = []string{"go"}
	opts.Docs = []string{"markdown"}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}

	if !hasNode(idx, NodeSection, "Guide") {
		t.Fatalf("missing Guide section: %v", titles(idx, NodeSection))
	}
	if !hasEdgeType(idx, EdgeLinks) {
		t.Fatal("expected links edge from markdown")
	}
	if !hasEdgeType(idx, EdgeMentions) {
		t.Fatal("expected mentions edge")
	}
	foundScale := false
	for _, e := range idx.Edges {
		if e.Type == EdgeMentions {
			for _, n := range idx.Nodes {
				if n.ID == e.TargetID && n.Title == "Scale" {
					foundScale = true
				}
			}
		}
	}
	if !foundScale {
		t.Fatal("expected mention resolved to Scale")
	}
}

func TestIndexHTMLHeadingsAndAnchors(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "site", "index.html"), `<html><body>
<h1>Home</h1>
<a href="./about.html">About</a>
<code>boot</code>
</body></html>`)
	mustWrite(t, filepath.Join(root, "site", "about.html"), `<h2>About</h2>`)
	mustWrite(t, filepath.Join(root, "scripts", "boot.sh"), "boot() {\n  echo ok\n}\n")

	opts := DefaultBuildOptions()
	opts.Languages = []string{"shell"}
	opts.Docs = []string{"html"}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNode(idx, NodeSection, "Home") {
		t.Fatalf("missing Home section: %v", titles(idx, NodeSection))
	}
	if !hasEdgeType(idx, EdgeLinks) {
		t.Fatal("expected html link edge")
	}
}

func TestIndexTXTMentions(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "NOTES.txt"), "Read src/util.py and call parse_config later.\n")
	mustWrite(t, filepath.Join(root, "src", "util.py"), "def parse_config():\n    return {}\n")

	opts := DefaultBuildOptions()
	opts.Languages = []string{"python"}
	opts.Docs = []string{"txt"}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	foundFile := false
	for _, e := range idx.Edges {
		if e.Type == EdgeLinks || e.Type == EdgeMentions {
			if strings.Contains(e.TargetID, "src/util.py") || strings.Contains(e.TargetID, "file:src/util.py") {
				foundFile = true
			}
		}
	}
	if !foundFile {
		t.Fatalf("expected txt reference to src/util.py, edges=%v", idx.Edges)
	}
}

func TestDocsFilter(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.md"), "# A\n")
	mustWrite(t, filepath.Join(root, "a.txt"), "hello\n")
	opts := DefaultBuildOptions()
	opts.Docs = []string{"markdown"}
	opts.Languages = []string{"go"}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	hasMD := false
	hasTXT := false
	for _, n := range idx.Nodes {
		if strings.HasSuffix(n.Path, ".md") {
			hasMD = true
		}
		if strings.HasSuffix(n.Path, ".txt") {
			hasTXT = true
		}
	}
	if !hasMD {
		t.Fatal("expected markdown indexed")
	}
	if hasTXT {
		t.Fatal("txt should be filtered out")
	}
}

func hasEdgeType(idx *Index, typ EdgeType) bool {
	for _, e := range idx.Edges {
		if e.Type == typ {
			return true
		}
	}
	return false
}
