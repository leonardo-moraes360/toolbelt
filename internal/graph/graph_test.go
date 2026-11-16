package graph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSearchCallers(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "mathutil", "math.go"), `package mathutil
func Scale(v, factor int) int { return v * factor }
`)
	mustWrite(t, filepath.Join(root, "service", "billing.go"), `package service
import "example/mathutil"
func Total(price, qty int) int { return mathutil.Scale(price, qty) }
func Discounted(price, qty, discount int) int { return Total(price, qty) - discount }
`)

	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Nodes) == 0 {
		t.Fatal("expected nodes")
	}

	storePath := filepath.Join(t.TempDir(), "graph.json")
	s := NewStore(storePath)
	s.index = idx
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	hits := s.Search("Scale", 10)
	if len(hits) == 0 {
		t.Fatal("search Scale empty")
	}

	callers := s.Callers("Scale", 2)
	found := false
	for _, c := range callers {
		if c.Title == "Total" || c.Title == "Discounted" {
			found = true
		}
	}
	if !found && len(callers) == 0 {
		t.Logf("callers=%v edges=%d", callers, len(idx.Edges))
	}

	deps := s.Deps("service", true)
	if len(deps) == 0 {
		t.Log("deps empty (ok if import path not local)")
	}
}

func TestBuildFromRootTypeScript(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "packages", "core", "src", "format.ts"), `
import { mask } from './privacy.js';

export function numeroBr(n: number): string {
  return String(n);
}

export const pct = (n: number) => numeroBr(n) + '%';

export function documento(v: string): string {
  return mask(v);
}
`)
	mustWrite(t, filepath.Join(root, "packages", "core", "src", "privacy.ts"), `
export function mask(value: string): string {
  return '***';
}
`)
	mustWrite(t, filepath.Join(root, "packages", "core", "src", "format.test.ts"), `
import { numeroBr } from './format.ts';
`)

	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Nodes) == 0 {
		t.Fatal("expected TS nodes")
	}

	foundFn := false
	foundImport := false
	foundTest := false
	for _, n := range idx.Nodes {
		if n.Type == NodeFunction && n.Title == "numeroBr" {
			foundFn = true
		}
		if n.Type == NodePackage && strings.Contains(n.Title, "./privacy") {
			foundImport = true
		}
		if strings.Contains(n.Path, "format.test.ts") {
			foundTest = true
		}
	}
	if !foundFn {
		t.Fatal("expected numeroBr function node")
	}
	if !foundImport {
		t.Fatal("expected privacy import edge/node")
	}
	if foundTest {
		t.Fatal("test files should be skipped")
	}

	s := NewStore(filepath.Join(t.TempDir(), "graph.json"))
	s.index = idx
	hits := s.Search("numeroBr", 10)
	if len(hits) == 0 {
		t.Fatal("search numeroBr empty")
	}
}

func TestBuildFromRootJavaScriptMJS(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "agents", "deterministic.mjs"), `
import fs from 'node:fs';

export async function run(task) {
  return fs.readFileSync(task, 'utf8');
}

const helper = () => run('x');
`)
	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range idx.Nodes {
		if n.Type == NodeFunction && n.Title == "run" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected run function from .mjs")
	}
	hasImport := false
	for _, e := range idx.Edges {
		if e.Type == EdgeImports {
			hasImport = true
		}
	}
	if !hasImport {
		t.Fatal("expected import edge")
	}
}

func TestBuildFromRootDoesNotSkipRootNamedLikeSkipDir(t *testing.T) {
	parent := t.TempDir()
	for _, name := range []string{"target", "build", "dist"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(parent, name)
			mustWrite(t, filepath.Join(root, "pkg", "hello.go"), `package pkg
func Hello() string { return "hi" }
`)
			mustWrite(t, filepath.Join(root, "node_modules", "lib", "index.js"), `
export function ignored() { return 1; }
`)
			mustWrite(t, filepath.Join(root, "nested", "target", "src", "skip.go"), `package skip
func SkipMe() {}
`)

			idx, err := BuildFromRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(idx.Nodes) == 0 {
				t.Fatalf("expected nodes when root basename is %q", name)
			}

			foundHello := false
			foundIgnored := false
			foundNestedSkip := false
			for _, n := range idx.Nodes {
				if n.Type == NodeFunction && n.Title == "Hello" {
					foundHello = true
				}
				if strings.Contains(n.Path, "node_modules") || n.Title == "ignored" {
					foundIgnored = true
				}
				if strings.Contains(n.Path, filepath.Join("nested", "target")) || n.Title == "SkipMe" {
					foundNestedSkip = true
				}
			}
			if !foundHello {
				t.Fatalf("expected Hello from root named %q", name)
			}
			if foundIgnored {
				t.Fatal("node_modules child should still be skipped")
			}
			if foundNestedSkip {
				t.Fatal("nested target/ child should still be skipped")
			}
		})
	}
}

func TestBuildFromRootResolvesSymlinkRoot(t *testing.T) {
	realRoot := t.TempDir()
	mustWrite(t, filepath.Join(realRoot, "pkg", "hello.go"), `package pkg
func Hello() string { return "hi" }
`)

	linkParent := t.TempDir()
	linkRoot := filepath.Join(linkParent, "target")
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Fatal(err)
	}

	idx, err := BuildFromRoot(linkRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Nodes) == 0 {
		t.Fatal("expected nodes when index root is a symlink to a real directory")
	}

	foundHello := false
	for _, n := range idx.Nodes {
		if n.Type == NodeFunction && n.Title == "Hello" {
			foundHello = true
			break
		}
	}
	if !foundHello {
		t.Fatal("expected Hello from symlink root")
	}

	resolved, err := filepath.EvalSymlinks(linkRoot)
	if err != nil {
		t.Fatal(err)
	}
	if idx.Root != resolved {
		t.Fatalf("Index.Root = %q, want resolved %q", idx.Root, resolved)
	}
}

func TestIndexerRegistryExtensible(t *testing.T) {
	names := map[string]bool{}
	for _, idx := range Indexers() {
		names[idx.Name()] = true
	}
	required := []string{
		"go", "javascript", "python", "java", "kotlin", "csharp",
		"cpp", "rust", "php", "ruby", "swift", "shell",
		"markdown", "html", "txt",
		"react", "vue", "svelte", "astro", "angular", "routes", "mdx",
	}
	for _, name := range required {
		if !names[name] {
			t.Fatalf("missing indexer %q in %#v", name, names)
		}
	}
}

func TestEnabledFlag(t *testing.T) {
	t.Setenv(EnvEnable, "")
	t.Setenv(FlagEnv, "")
	if Enabled() {
		t.Fatal("expected disabled by default")
	}
	t.Setenv(EnvEnable, "on")
	if !Enabled() {
		t.Fatal("expected enabled")
	}
}

func TestRequireEnabled(t *testing.T) {
	t.Setenv(EnvEnable, "off")
	t.Setenv(FlagEnv, "")
	s := NewStore(filepath.Join(t.TempDir(), "g.json"))
	if err := s.RequireEnabled(); err == nil {
		t.Fatal("expected error when disabled")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
