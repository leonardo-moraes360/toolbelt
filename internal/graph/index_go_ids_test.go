package graph

import (
	"go/ast"
	"path/filepath"
	"testing"
)

func nodeIDs(idx *Index) map[string]Node {
	m := map[string]Node{}
	for _, n := range idx.Nodes {
		m[n.ID] = n
	}
	return m
}

func hasEdge(idx *Index, typ EdgeType, src, dst string) bool {
	for _, e := range idx.Edges {
		if e.Type == typ && e.SourceID == src && e.TargetID == dst {
			return true
		}
	}
	return false
}

func TestGoIndexerIDsDistinguishPackagesAndReceivers(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(root, "cmd", "a", "main.go"), "package main\nfunc main() {}\nfunc Run() {}\n")
	mustWrite(t, filepath.Join(root, "cmd", "b", "main.go"), "package main\nfunc main() {}\n")
	mustWrite(t, filepath.Join(root, "store", "store.go"), `package store
import (
	"fmt"
	"example.com/app/util"
)
type A struct{}
type B[T any] struct{}
func (a *A) Open() { a.close(); util.Do(); fmt.Println() }
func (a *A) close() {}
func (b B[T]) Open() {}
`)
	mustWrite(t, filepath.Join(root, "util", "util.go"), "package util\nfunc Do() {}\n")

	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	ids := nodeIDs(idx)
	for _, want := range []string{
		"pkg:example.com/app/cmd/a", "pkg:example.com/app/cmd/b",
		"func:example.com/app/cmd/a.main", "func:example.com/app/cmd/b.main",
		"func:example.com/app/store.A.Open", "func:example.com/app/store.B.Open",
		"func:example.com/app/util.Do",
	} {
		if _, ok := ids[want]; !ok {
			t.Errorf("missing node %q", want)
		}
	}
	if _, ok := ids["pkg:main"]; ok {
		t.Error("package IDs must not collapse to the package clause")
	}
	if !hasEdge(idx, EdgeCalls, "func:example.com/app/store.A.Open", "func:example.com/app/store.A.close") {
		t.Error("receiver call not resolved to the method ID")
	}
	if !hasEdge(idx, EdgeCalls, "func:example.com/app/store.A.Open", "func:example.com/app/util.Do") {
		t.Error("imported call not resolved to the import path")
	}
	if idx.Version != IndexVersion {
		t.Errorf("Version = %d", idx.Version)
	}
}

func TestGoIndexerIDsWithoutGoMod(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a", "x.go"), "package main\nfunc F() {}\n")
	mustWrite(t, filepath.Join(root, "b", "x.go"), "package main\nfunc F() {}\n")
	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	ids := nodeIDs(idx)
	for _, want := range []string{"func:a.F", "func:b.F", "pkg:a", "pkg:b"} {
		if _, ok := ids[want]; !ok {
			t.Errorf("missing node %q", want)
		}
	}
}

func TestStoreOlderIndexVersionIsStale(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.go"), "package a\nfunc F() {}\n")
	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(filepath.Join(t.TempDir(), "graph.json"))
	s.index = idx
	opts := DefaultBuildOptions()
	if stale, err := s.CheckStale(root, &opts); err != nil || stale {
		t.Fatalf("fresh index: stale=%v err=%v", stale, err)
	}
	idx.Version = 1
	if stale, _ := s.CheckStale(root, &opts); !stale {
		t.Fatal("an index from an older ID scheme must be stale")
	}
}

func TestGoIndexerOneSegmentModuleDoesNotResolveLocalVariableCalls(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module app\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(root, "a.go"), `package app
func Run() {}
func Use(db *DB) { db.Close(); Close(); Gen[int](1) }
func Close() {}
func Gen[T any](v T) {}
type DB struct{}
func (d *DB) Close() {}
`)
	mustWrite(t, filepath.Join(root, "db", "db.go"), "package db\nfunc Close() {}\n")
	idx, err := BuildFromRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if hasEdge(idx, EdgeCalls, "func:app.Use", "func:db.Close") {
		t.Error("db.Close() on a local variable resolved to package db's Close")
	}
	if !hasEdge(idx, EdgeCalls, "func:app.Use", "func:?db.Close") {
		t.Error("unresolved selector must use the func:? namespace")
	}
	if !hasEdge(idx, EdgeCalls, "func:app.Use", "func:app.Gen") {
		t.Error("generic call not unwrapped")
	}
}

func TestGoImportName(t *testing.T) {
	for path, want := range map[string]string{
		"github.com/x/go-yaml": "yaml",
		"gopkg.in/yaml.v3":     "yaml",
		"github.com/x/y/v2":    "y",
		"example.com/app/util": "util",
	} {
		spec := &ast.ImportSpec{}
		if got := goImportName(spec, path); got != want {
			t.Errorf("goImportName(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestFormatOutdatedAndMaxFileBytesClamp(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "graph.json"))
	s.index = &Index{Fingerprint: "x", Version: 1}
	if !s.FormatOutdated() {
		t.Error("older version must report FormatOutdated")
	}
	s.index = &Index{Fingerprint: "x", Version: IndexVersion}
	if s.FormatOutdated() {
		t.Error("current version is not outdated")
	}
	opts := BuildOptions{MaxFileBytes: 1 << 40}
	normalizeBuildOptions(&opts)
	if opts.MaxFileBytes != maxIndexReadBytes {
		t.Errorf("MaxFileBytes = %d, want clamp to %d", opts.MaxFileBytes, maxIndexReadBytes)
	}
}
