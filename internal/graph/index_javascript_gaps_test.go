package graph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTSGapsFixture(t *testing.T, root string) {
	t.Helper()
	mustWrite(t, filepath.Join(root, "tsconfig.json"), `{
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"]
    }
  }
}
`)
	mustWrite(t, filepath.Join(root, "src", "utils.ts"), `
export function identity<T>(value: T): T {
  return value;
}

export function helper(x: string): string {
  return x.trim();
}

export function mask(value: string): string {
  return '***';
}
`)
	mustWrite(t, filepath.Join(root, "src", "index.ts"), `
export { identity, helper, mask } from './utils.js';
`)
	mustWrite(t, filepath.Join(root, "src", "app.ts"), `
import { helper as format } from './utils';
import * as Utils from './utils.js';
import { identity } from '@/utils';
import { mask } from './index.js';

export function run(s: string) {
  return format(identity(Utils.helper(mask(s))));
}
`)
	mustWrite(t, filepath.Join(root, "src", "utils.test.ts"), `
import { helper } from './utils';
export function testHelper() {
  return helper('x');
}
`)
	mustWrite(t, filepath.Join(root, "src", "utils.spec.ts"), `
import { mask } from './utils';
export function specMask() {
  return mask('y');
}
`)
	mustWrite(t, filepath.Join(root, "src", "__tests__", "helper.test.ts"), `
import { helper } from '../utils';
export function suiteHelper() {
  return helper('z');
}
`)
	mustWrite(t, filepath.Join(root, "e2e", "app.e2e.ts"), `
import { run } from '../src/app';
export function e2eRun() {
  return run('ok');
}
`)
}

func TestTypeScriptGraphGapsFixture(t *testing.T) {
	root := t.TempDir()
	writeTSGapsFixture(t, root)
	opts := DefaultBuildOptions()
	opts.Languages = []string{"javascript"}
	opts.Docs = []string{}
	opts.Frameworks = []string{}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("a_test_files_indexed", func(t *testing.T) { assertTSTestFilesIndexed(t, idx) })
	t.Run("b_generic_functions", func(t *testing.T) { assertTSGenericIndexed(t, idx) })
	t.Run("c_tsconfig_paths", func(t *testing.T) { assertTSConfigPathsResolved(t, idx) })
	t.Run("d_nodenext_relative_imports", func(t *testing.T) { assertTSNodeNextImports(t, idx) })
	t.Run("e_import_alias_and_namespace", func(t *testing.T) { assertTSImportBindings(t, idx) })
}

func assertTSTestFilesIndexed(t *testing.T, idx *Index) {
	t.Helper()
	for _, p := range []string{
		"src/utils.test.ts", "src/utils.spec.ts", "src/__tests__/helper.test.ts", "e2e/app.e2e.ts",
	} {
		if !hasFilePath(idx, p) {
			t.Fatalf("expected test/e2e file %q indexed; files=%v", p, filePaths(idx))
		}
	}
	for _, title := range []string{"testHelper", "specMask", "suiteHelper", "e2eRun"} {
		if !hasNode(idx, NodeFunction, title) {
			t.Fatalf("expected function %q; funcs=%v", title, titles(idx, NodeFunction))
		}
	}
}

func assertTSGenericIndexed(t *testing.T, idx *Index) {
	t.Helper()
	if !hasNode(idx, NodeFunction, "identity") {
		t.Fatalf("expected generic identity<T>; funcs=%v", titles(idx, NodeFunction))
	}
}

func assertTSConfigPathsResolved(t *testing.T, idx *Index) {
	t.Helper()
	if hasImportTitle(idx, "@/utils") {
		t.Fatalf("@/utils should resolve via tsconfig paths, not stay as pkg; imports=%v", importTitles(idx))
	}
	if !hasImportResolvedTo(idx, "src/utils.ts") {
		t.Fatalf("expected @/utils resolved to file:src/utils.ts; imports=%v", importTargets(idx))
	}
}

func assertTSNodeNextImports(t *testing.T, idx *Index) {
	t.Helper()
	for _, bad := range []string{"./utils.js", "./utils", "./index.js", "../src/app"} {
		if hasImportTitle(idx, bad) {
			t.Fatalf("relative import %q should resolve to file, not pkg; imports=%v", bad, importTitles(idx))
		}
	}
	for _, want := range []string{"src/utils.ts", "src/index.ts", "src/app.ts"} {
		if !hasImportResolvedTo(idx, want) {
			t.Fatalf("expected resolved import to %q; imports=%v", want, importTargets(idx))
		}
	}
}

func assertTSImportBindings(t *testing.T, idx *Index) {
	t.Helper()
	store := NewStore(filepath.Join(t.TempDir(), "graph.json"))
	store.index = idx
	callers := store.Callers("helper", 1)
	if !callerTitlesContain(callers, "run") {
		t.Fatalf("alias/namespace callers(helper) missing run; got %#v calls=%v", callers, jsCallEdges(idx))
	}
	if !callerTitlesContain(callers, "testHelper") || !callerTitlesContain(callers, "suiteHelper") {
		t.Fatalf("impact/callers should include tests; got %#v", callers)
	}
	idCallers := store.Callers("identity", 1)
	if !callerTitlesContain(idCallers, "run") {
		t.Fatalf("callers(identity) missing run (alias @/ + generic); got %#v calls=%v", idCallers, jsCallEdges(idx))
	}
	impact := store.Impact("src/utils.ts", 2)
	if !callerTitlesContain(impact, "run") || !callerTitlesContain(impact, "testHelper") {
		t.Fatalf("impact(utils) should list run and tests; got %#v", impact)
	}
}

func TestTypeScriptGraphStaleDetection(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "src", "a.ts"), `export function alpha() { return 1; }`)

	opts := DefaultBuildOptions()
	opts.Languages = []string{"javascript"}
	opts.Docs = []string{}
	opts.Frameworks = []string{}

	store := NewStore(filepath.Join(t.TempDir(), "graph.json"))
	idx, err := store.RebuildOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if idx.Fingerprint == "" {
		t.Fatal("expected fingerprint on index")
	}
	stale, err := store.CheckStale(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Fatal("fresh index should not be stale")
	}

	time.Sleep(10 * time.Millisecond)
	mustWrite(t, filepath.Join(root, "src", "a.ts"), `export function alpha() { return 2; }
export function beta() { return 3; }
`)

	stale, err = store.CheckStale(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Fatal("expected stale after source mtime/content change")
	}

	refreshed, err := store.EnsureFresh(root, &opts, true)
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed {
		t.Fatal("index_on_change should reindex when stale")
	}
	if !hasNode(store.index, NodeFunction, "beta") {
		t.Fatalf("reindex should pick up beta; funcs=%v", titles(store.index, NodeFunction))
	}
	if store.Stale() {
		t.Fatal("after reindex store should not report stale")
	}

	time.Sleep(10 * time.Millisecond)
	mustWrite(t, filepath.Join(root, "src", "a.ts"), `export function alpha() { return 9; }`)
	refreshed, err = store.EnsureFresh(root, &opts, false)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed {
		t.Fatal("index_on_change=false must not reindex")
	}
	if !store.Stale() {
		t.Fatal("expected Stale() true when outdated and index_on_change=false")
	}
}

func hasFilePath(idx *Index, want string) bool {
	want = filepath.ToSlash(want)
	for _, n := range idx.Nodes {
		if n.Type == NodeFile && filepath.ToSlash(n.Path) == want {
			return true
		}
	}
	return false
}

func filePaths(idx *Index) []string {
	var out []string
	for _, n := range idx.Nodes {
		if n.Type == NodeFile {
			out = append(out, n.Path)
		}
	}
	return out
}

func hasImportTitle(idx *Index, title string) bool {
	for _, n := range idx.Nodes {
		if n.Type == NodePackage && n.Title == title {
			return true
		}
	}
	return false
}

func importTitles(idx *Index) []string {
	var out []string
	for _, n := range idx.Nodes {
		if n.Type == NodePackage && (strings.HasPrefix(n.Title, ".") || strings.HasPrefix(n.Title, "@/") || strings.HasPrefix(n.Title, "@")) {
			out = append(out, n.Title)
		}
	}
	return out
}

func importTargets(idx *Index) []string {
	var out []string
	for _, e := range idx.Edges {
		if e.Type == EdgeImports {
			out = append(out, e.TargetID)
		}
	}
	return out
}

func hasImportResolvedTo(idx *Index, rel string) bool {
	want := "file:" + filepath.ToSlash(rel)
	for _, e := range idx.Edges {
		if e.Type == EdgeImports && e.TargetID == want {
			return true
		}
	}
	return false
}

func jsCallEdges(idx *Index) []string {
	byID := map[string]Node{}
	for _, n := range idx.Nodes {
		byID[n.ID] = n
	}
	var out []string
	for _, e := range idx.Edges {
		if e.Type != EdgeCalls {
			continue
		}
		s, tgt := byID[e.SourceID], byID[e.TargetID]
		out = append(out, s.Title+"->"+tgt.Title)
	}
	return out
}

func TestMainFingerprintUsesMtime(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "src", "a.ts")
	mustWrite(t, path, `export function a() {}`)
	opts := DefaultBuildOptions()
	opts.Languages = []string{"javascript"}
	opts.Docs = []string{}
	opts.Frameworks = []string{}
	fp1, err := SourceFingerprint(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = info
	time.Sleep(10 * time.Millisecond)
	mustWrite(t, path, `export function a() { return 1; }`)
	fp2, err := SourceFingerprint(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if fp1 == fp2 {
		t.Fatal("fingerprint should change when file content/mtime changes")
	}
}
