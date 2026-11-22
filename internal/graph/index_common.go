package graph

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

func moduleNameFromRel(rel string) string {
	slash := filepath.ToSlash(rel)
	dir := filepath.ToSlash(filepath.Dir(slash))
	if dir == "." || dir == "" {
		return strings.TrimSuffix(filepath.Base(slash), filepath.Ext(slash))
	}
	return dir
}

func uniqueMatches(re *regexp.Regexp, src string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range re.FindAllStringSubmatch(src, -1) {
		if len(m) < 2 || m[1] == "" || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	return out
}

func uniqueMatchesMulti(res []*regexp.Regexp, src string) []string {
	seen := map[string]bool{}
	var out []string
	for _, re := range res {
		if re == nil {
			continue
		}
		for _, name := range uniqueMatches(re, src) {
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// maxIndexReadBytes caps what the walk reads from one file. The walk already
// skips files over graph.max_file_bytes; this bounds a file that grew, or a
// direct IndexFile call, between the stat and the read.
const maxIndexReadBytes = 16 << 20

var (
	errNotRegular = errors.New("not a regular file or crosses a symlink")
	errTooLarge   = errors.New("file too large")
)

// readRegularFile reads rel under root, refusing a symlink on any path
// component (the deny list and the repo boundary match the indexed name, so a
// link such as docs/notes.md -> ../.env would smuggle a file in) and anything
// that is not a regular file. The final open is non-blocking so a FIFO that
// replaces the file after the Lstat cannot hang the walk.
func readRegularFile(root *os.Root, rel string, maxBytes int64) ([]byte, error) {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i := range parts {
		fi, err := root.Lstat(filepath.FromSlash(strings.Join(parts[:i+1], "/")))
		if err != nil {
			return nil, err
		}
		if fi.Mode()&os.ModeSymlink != 0 || (i == len(parts)-1 && !fi.Mode().IsRegular()) {
			return nil, errNotRegular
		}
	}
	f, err := root.OpenFile(filepath.FromSlash(rel), os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errNotRegular
	}
	if fi.Size() > maxBytes {
		return nil, errTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errTooLarge
	}
	return data, nil
}

func readSourceBytes(root, rel string) ([]byte, error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return readRegularFile(r, rel, maxIndexReadBytes)
}

func readSource(root, rel string) (string, error) {
	data, err := readSourceBytes(root, rel)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func baseFileNodes(modName, rel string, conf float64) (nodes []Node, edges []Edge, pkgID, fileID string) {
	pkgID = "pkg:" + modName
	fileID = "file:" + filepath.ToSlash(rel)
	nodes = []Node{
		{ID: pkgID, Type: NodePackage, Title: modName, Package: modName, Source: "extracted", Confidence: conf},
		{ID: fileID, Type: NodeFile, Path: filepath.ToSlash(rel), Title: filepath.Base(rel), Package: modName, Source: "extracted", Confidence: conf},
	}
	edges = []Edge{
		{SourceID: pkgID, TargetID: fileID, Type: EdgeDefines, Source: "extracted", Confidence: conf},
	}
	return nodes, edges, pkgID, fileID
}

func appendNamedNodes(
	nodes []Node,
	edges []Edge,
	fileID, modName, relPath string,
	names []string,
	nodeType NodeType,
	idPrefix string,
	conf float64,
) ([]Node, []Edge) {
	slash := filepath.ToSlash(relPath)
	for _, name := range names {
		id := idPrefix + ":" + modName + "." + name
		nodes = append(nodes, Node{
			ID: id, Type: nodeType, Path: slash,
			Title: name, Package: modName, Source: "extracted", Confidence: conf,
		})
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: id, Type: EdgeDefines, Source: "extracted", Confidence: conf,
		})
	}
	return nodes, edges
}

// appendJSImportEdges is appendImportEdges for JS-family sources: relative,
// tsconfig-alias and workspace imports become file: edges, the rest pkg:.
func appendJSImportEdges(nodes []Node, edges []Edge, root, rel, fileID string, imports []string, conf float64) ([]Node, []Edge) {
	cfg := loadJSPathConfig(root)
	for _, imp := range imports {
		nodes, edges = appendResolvedJSImport(nodes, edges, root, rel, fileID, imp, cfg, conf)
	}
	return nodes, edges
}

func appendImportEdges(nodes []Node, edges []Edge, fileID string, imports []string, conf float64) ([]Node, []Edge) {
	for _, imp := range imports {
		target := "pkg:" + imp
		nodes = append(nodes, Node{
			ID: target, Type: NodePackage, Title: imp, Source: "extracted", Confidence: conf,
		})
		edges = append(edges, Edge{
			SourceID: fileID, TargetID: target, Type: EdgeImports, Source: "extracted", Confidence: conf,
		})
	}
	return nodes, edges
}

type regexLang struct {
	name      string
	exts      map[string]bool
	funcRes   []*regexp.Regexp
	classRes  []*regexp.Regexp
	importRes []*regexp.Regexp
	skipTests bool
}

func (r *regexLang) Name() string     { return r.name }
func (r *regexLang) Category() string { return CategoryLanguage }

func (r *regexLang) Match(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if !r.exts[ext] {
		return false
	}
	if r.skipTests && skipIndexedTestFile(path) {
		return false
	}
	return true
}

func (r *regexLang) IndexFile(root, rel string) ([]Node, []Edge, error) {
	src, err := readSource(root, rel)
	if err != nil {
		return nil, nil, err
	}
	modName := moduleNameFromRel(rel)
	nodes, edges, _, fileID := baseFileNodes(modName, rel, 0.85)
	funcs := uniqueMatchesMulti(r.funcRes, src)
	nodes, edges = appendNamedNodes(nodes, edges, fileID, modName, rel, funcs, NodeFunction, "func", 0.8)
	classes := uniqueMatchesMulti(r.classRes, src)
	nodes, edges = appendNamedNodes(nodes, edges, fileID, modName, rel, classes, NodeClass, "class", 0.8)
	imports := uniqueMatchesMulti(r.importRes, src)
	nodes, edges = appendImportEdges(nodes, edges, fileID, imports, 0.85)
	return nodes, edges, nil
}

func extSet(exts ...string) map[string]bool {
	m := make(map[string]bool, len(exts))
	for _, e := range exts {
		m[e] = true
	}
	return m
}

func mustRE(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}
