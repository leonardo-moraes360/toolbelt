package graph

import (
	"path/filepath"
	"strings"
	"testing"
)

func writePnpmMonorepo(t *testing.T, root string) {
	t.Helper()
	mustWrite(t, filepath.Join(root, "pnpm-workspace.yaml"), "packages:\n  - \"packages/*\"\n  - \"apps/*\"\n")
	mustWrite(t, filepath.Join(root, "package.json"), `{"name":"root","private":true}`)
	mustWrite(t, filepath.Join(root, "packages", "core", "package.json"),
		`{"name":"@paprika/core","exports":{".":{"types":"./dist/index.d.ts","import":"./dist/index.js"},"./ops":"./src/ops.ts"}}`)
	mustWrite(t, filepath.Join(root, "packages", "core", "src", "index.ts"), "export * from './render'\n")
	mustWrite(t, filepath.Join(root, "packages", "core", "src", "render.ts"),
		"export function nodeStyle(n: any) { return { color: n.c } }\n")
	mustWrite(t, filepath.Join(root, "packages", "core", "src", "ops.ts"),
		"export function applyOps(o: any[]) { return o.length }\n")
	mustWrite(t, filepath.Join(root, "apps", "web", "package.json"), `{"name":"@paprika/web"}`)
	mustWrite(t, filepath.Join(root, "apps", "web", "src", "Canvas.tsx"), `import { nodeStyle } from '@paprika/core'
import { applyOps } from '@paprika/core/ops'
export function Canvas(p: any) { applyOps([]); return <div style={nodeStyle(p.n)} /> }
`)
	mustWrite(t, filepath.Join(root, "apps", "web", "src", "App.tsx"), `import { Canvas } from './Canvas'
import React from 'react'
export function App() { return <Canvas n={1} /> }
`)
}

func buildStore(t *testing.T, root string) *Store {
	t.Helper()
	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(filepath.Join(t.TempDir(), "graph.json"))
	s.index = idx
	return s
}

func TestWorkspaceImportResolvesToLocalPackage(t *testing.T) {
	root := t.TempDir()
	writePnpmMonorepo(t, root)
	s := buildStore(t, root)

	var titles []string
	for _, c := range s.Callers("nodeStyle", 1) {
		titles = append(titles, c.Title)
	}
	if !strings.Contains(strings.Join(titles, ","), "Canvas") {
		t.Fatalf("callers must cross the workspace boundary, got %v", titles)
	}
	titles = nil
	for _, c := range s.Callers("applyOps", 1) {
		titles = append(titles, c.Title)
	}
	if !strings.Contains(strings.Join(titles, ","), "Canvas") {
		t.Fatalf("subpath export callers, got %v", titles)
	}

	for _, e := range s.index.Edges {
		if e.Type != EdgeImports {
			continue
		}
		if e.TargetID == "pkg:@paprika/core" || e.TargetID == "pkg:./Canvas" {
			t.Fatalf("local import left as external package: %+v", e)
		}
	}
	external := false
	for _, e := range s.index.Edges {
		if e.Type == EdgeImports && e.TargetID == "pkg:react" {
			external = true
		}
	}
	if !external {
		t.Fatal("real external packages must stay pkg:")
	}
}

func TestNpmWorkspacesObjectForm(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "package.json"), `{"name":"root","workspaces":{"packages":["libs/**"]}}`)
	mustWrite(t, filepath.Join(root, "libs", "a", "util", "package.json"), `{"name":"util","main":"lib/index.js"}`)
	mustWrite(t, filepath.Join(root, "libs", "a", "util", "src", "index.ts"), "export const x = 1\n")
	ws := readJSWorkspace(root)
	cands := ws.resolve("util")
	if len(cands) == 0 {
		t.Fatal("npm object-form workspaces not read")
	}
	rel, ok := resolveJSExisting(root, cands[0])
	for _, c := range cands[1:] {
		if ok {
			break
		}
		rel, ok = resolveJSExisting(root, c)
	}
	if !ok || rel != "libs/a/util/src/index.ts" {
		t.Fatalf("lib/ entry should fall back to src/: %v -> %q", cands, rel)
	}
	if ws.resolve("lodash") != nil {
		t.Fatal("non-workspace package must not resolve")
	}
}

func TestWorkspaceRejectsPathsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pnpm-workspace.yaml"), "packages:\n  - \"../*\"\n  - \"pkgs/*\"\n")
	mustWrite(t, filepath.Join(root, "pkgs", "evil", "package.json"), `{"name":"evil","main":"../../../etc/passwd"}`)
	ws := readJSWorkspace(root)
	for name, p := range ws.pkgs {
		if !insideRoot(p.dir) {
			t.Fatalf("package %s dir escapes root: %s", name, p.dir)
		}
	}
	for _, c := range ws.resolve("evil") {
		if !insideRoot(c) {
			t.Fatalf("candidate escapes root: %s", c)
		}
	}
}

func TestBarrelPrefersExactModule(t *testing.T) {
	nodes := map[string]Node{
		"func:ui.render":        {ID: "func:ui.render", Type: NodeFunction, Path: "ui/index.ts"},
		"func:ui/Button.render": {ID: "func:ui/Button.render", Type: NodeFunction, Path: "ui/Button.tsx"},
	}
	ids := []string{"func:ui.render", "func:ui/Button.render"}
	got := filterByModuleHint(nodes, ids, "ui/index.ts")
	if len(got) != 1 || got[0] != "func:ui.render" {
		t.Fatalf("barrel defining the symbol must win over siblings: %v", got)
	}
	got = filterByModuleHint(nodes, []string{"func:ui/Button.render"}, "ui/index.ts")
	if len(got) != 1 {
		t.Fatalf("barrel re-export falls back to directory: %v", got)
	}
}
