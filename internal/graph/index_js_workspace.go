package graph

import (
	"encoding/json"
	"fmt"
	"os"
	gopath "path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// jsWorkspace maps the package names declared by a monorepo workspace
// (pnpm-workspace.yaml, or "workspaces" in the root package.json) to their
// directories, so `import { x } from "@scope/pkg"` resolves to the local
// sources instead of an external pkg: node (field study: paprika).
type jsWorkspace struct {
	pkgs map[string]jsWorkspacePkg
}

type jsWorkspacePkg struct {
	dir     string // repo-relative, slash-separated
	exports json.RawMessage
	module  string
	main    string
	types   string
	typings string
	source  string
}

type jsWorkspaceCacheEntry struct {
	stamp string
	ws    *jsWorkspace
}

var jsWorkspaceCache sync.Map // root -> jsWorkspaceCacheEntry

func loadJSWorkspace(root string) *jsWorkspace {
	stamp := jsWorkspaceStamp(root)
	if v, ok := jsWorkspaceCache.Load(root); ok {
		if e := v.(jsWorkspaceCacheEntry); e.stamp == stamp {
			return e.ws
		}
	}
	ws := readJSWorkspace(root)
	jsWorkspaceCache.Store(root, jsWorkspaceCacheEntry{stamp: stamp, ws: ws})
	return ws
}

// jsWorkspaceStamp changes whenever a file that declares workspace members
// changes; member package.json edits are picked up on the next stamp change
// or a new process (graph index runs are short-lived).
func jsWorkspaceStamp(root string) string {
	var b strings.Builder
	for _, name := range []string{"pnpm-workspace.yaml", "package.json"} {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			b.WriteString("-|")
			continue
		}
		fmt.Fprintf(&b, "%s:%d|", info.ModTime().Format(time.RFC3339Nano), info.Size())
	}
	return b.String()
}

func readJSWorkspace(root string) *jsWorkspace {
	ws := &jsWorkspace{pkgs: map[string]jsWorkspacePkg{}}
	var globs []string
	if data, err := readSourceBytes(root, "pnpm-workspace.yaml"); err == nil {
		var raw struct {
			Packages []string `yaml:"packages"`
		}
		if yaml.Unmarshal(data, &raw) == nil {
			globs = append(globs, raw.Packages...)
		}
	}
	globs = append(globs, packageJSONWorkspaces(root)...)
	for _, dir := range expandWorkspaceGlobs(root, globs) {
		if pkg, name, ok := readWorkspacePackage(root, dir); ok {
			if _, dup := ws.pkgs[name]; !dup {
				ws.pkgs[name] = pkg
			}
		}
	}
	return ws
}

// packageJSONWorkspaces reads "workspaces" from the root package.json, in the
// array form (npm/yarn) or the {"packages": [...]} object form (yarn classic).
func packageJSONWorkspaces(root string) []string {
	data, err := readSourceBytes(root, "package.json")
	if err != nil {
		return nil
	}
	var raw struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}
	if json.Unmarshal(data, &raw) != nil || len(raw.Workspaces) == 0 {
		return nil
	}
	var list []string
	if json.Unmarshal(raw.Workspaces, &list) == nil {
		return list
	}
	var obj struct {
		Packages []string `json:"packages"`
	}
	if json.Unmarshal(raw.Workspaces, &obj) == nil {
		return obj.Packages
	}
	return nil
}

func expandWorkspaceGlobs(root string, globs []string) []string {
	seen := map[string]bool{}
	excluded := map[string]bool{}
	var dirs []string
	for _, g := range globs {
		g = strings.TrimSpace(g)
		neg := strings.HasPrefix(g, "!")
		g = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(g, "!"), "./"), "/")
		if g == "" || !insideRoot(g) {
			continue
		}
		var matches []string
		if strings.HasSuffix(g, "/**") {
			matches = walkPackageDirs(root, strings.TrimSuffix(g, "/**"))
		} else {
			found, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(g)))
			for _, m := range found {
				if rel, err := filepath.Rel(root, m); err == nil {
					matches = append(matches, filepath.ToSlash(rel))
				}
			}
		}
		for _, m := range matches {
			if neg {
				excluded[m] = true
				continue
			}
			if !seen[m] {
				seen[m] = true
				dirs = append(dirs, m)
			}
		}
	}
	out := dirs[:0]
	for _, d := range dirs {
		if !excluded[d] && !strings.Contains(d, "node_modules") {
			out = append(out, d)
		}
	}
	sort.Strings(out)
	return out
}

func walkPackageDirs(root, base string) []string {
	var out []string
	start := filepath.Join(root, filepath.FromSlash(base))
	_ = filepath.WalkDir(start, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if d.Name() == "node_modules" || (strings.HasPrefix(d.Name(), ".") && p != start) {
			return filepath.SkipDir
		}
		if _, statErr := os.Stat(filepath.Join(p, "package.json")); statErr == nil {
			if rel, relErr := filepath.Rel(root, p); relErr == nil {
				out = append(out, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	return out
}

func readWorkspacePackage(root, dir string) (jsWorkspacePkg, string, bool) {
	data, err := readSourceBytes(root, gopath.Join(filepath.ToSlash(dir), "package.json"))
	if err != nil {
		return jsWorkspacePkg{}, "", false
	}
	var raw struct {
		Name    string          `json:"name"`
		Exports json.RawMessage `json:"exports"`
		Module  string          `json:"module"`
		Main    string          `json:"main"`
		Types   string          `json:"types"`
		Typings string          `json:"typings"`
		Source  string          `json:"source"`
	}
	if json.Unmarshal(data, &raw) != nil || strings.TrimSpace(raw.Name) == "" {
		return jsWorkspacePkg{}, "", false
	}
	pkg := jsWorkspacePkg{
		dir: dir, exports: raw.Exports, module: raw.Module, main: raw.Main,
		types: raw.Types, typings: raw.Typings, source: raw.Source,
	}
	return pkg, strings.TrimSpace(raw.Name), true
}

// resolve maps a bare import to repo-relative candidate paths inside a
// workspace package, or nil when imp names no workspace package.
func (ws *jsWorkspace) resolve(imp string) []string {
	if ws == nil || len(ws.pkgs) == 0 {
		return nil
	}
	name, sub := splitJSPackageImport(imp)
	pkg, ok := ws.pkgs[name]
	if !ok {
		return nil
	}
	key := "."
	if sub != "" {
		key = "./" + sub
	}
	var targets []string
	targets = append(targets, exportTargets(pkg.exports, key)...)
	if sub == "" {
		for _, f := range []string{pkg.source, pkg.types, pkg.typings, pkg.module, pkg.main} {
			if f != "" {
				targets = append(targets, f)
			}
		}
		targets = append(targets, "src/index", "index")
	} else {
		targets = append(targets, "src/"+sub, sub)
	}
	var out []string
	for _, t := range targets {
		t = strings.TrimPrefix(filepath.ToSlash(t), "./")
		if t == "" || !insideRoot(filepath.ToSlash(filepath.Join(pkg.dir, t))) {
			continue
		}
		out = append(out, filepath.ToSlash(filepath.Join(pkg.dir, t)))
		// Built entry points (dist/index.js) usually mirror src/; prefer sources.
		if strings.HasPrefix(t, "dist/") || strings.HasPrefix(t, "lib/") || strings.HasPrefix(t, "build/") {
			rest := t[strings.IndexByte(t, '/')+1:]
			out = append(out, filepath.ToSlash(filepath.Join(pkg.dir, "src", stripJSImportExt(rest))))
		}
	}
	return uniqueStrings(out)
}

// insideRoot rejects repo-relative paths that are absolute or climb out of
// the repository (workspace globs or package.json entries like "../../x").
func insideRoot(rel string) bool {
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(rel))
	return clean != ".." && !strings.HasPrefix(clean, "../")
}

// resetJSWorkspaceCache drops cached workspaces so a new index run (also in
// the long-lived MCP server) sees member packages added or renamed since.
func resetJSWorkspaceCache() {
	jsWorkspaceCache.Range(func(k, _ any) bool {
		jsWorkspaceCache.Delete(k)
		return true
	})
}

func splitJSPackageImport(imp string) (name, sub string) {
	parts := strings.Split(imp, "/")
	n := 1
	if strings.HasPrefix(imp, "@") && len(parts) > 1 {
		n = 2
	}
	if len(parts) < n {
		return imp, ""
	}
	return strings.Join(parts[:n], "/"), strings.Join(parts[n:], "/")
}

// exportTargets reads a package.json "exports" entry for key ("." or "./x"),
// accepting the string, subpath-map and condition-map forms.
func exportTargets(raw json.RawMessage, key string) []string {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if key == "." {
			return []string{s}
		}
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	if !hasSubpathKeys(m) {
		if key != "." {
			return nil
		}
		return conditionTargets(raw)
	}
	if v, ok := m[key]; ok {
		return conditionTargets(v)
	}
	return patternExportTargets(m, key)
}

func hasSubpathKeys(m map[string]json.RawMessage) bool {
	for k := range m {
		if strings.HasPrefix(k, ".") {
			return true
		}
	}
	return false
}

// patternExportTargets matches key against "./x/*" subpath patterns.
func patternExportTargets(m map[string]json.RawMessage, key string) []string {
	for pattern, v := range m {
		star := strings.Index(pattern, "*")
		if star < 0 {
			continue
		}
		prefix, suffix := pattern[:star], pattern[star+1:]
		if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, suffix) && len(key) >= len(prefix)+len(suffix) {
			captured := key[len(prefix) : len(key)-len(suffix)]
			var out []string
			for _, t := range conditionTargets(v) {
				out = append(out, strings.Replace(t, "*", captured, 1))
			}
			return out
		}
	}
	return nil
}

var exportConditionOrder = []string{"source", "development", "types", "import", "module", "default", "require", "node", "browser"}

func conditionTargets(raw json.RawMessage) []string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return []string{s}
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	var out []string
	for _, cond := range exportConditionOrder {
		if v, ok := m[cond]; ok {
			out = append(out, conditionTargets(v)...)
		}
	}
	return out
}
