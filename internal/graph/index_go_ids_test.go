package graph

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
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

func TestIndexSkipsSymlinksAndFIFOs(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "secret.md"), "# Leaked heading\n")
	mustWrite(t, filepath.Join(root, "docs", "real.md"), "# Real heading\n")
	if err := os.Symlink(filepath.Join(outside, "secret.md"), filepath.Join(root, "docs", "link.md")); err != nil {
		t.Skip("symlink unsupported:", err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "docs", "pipe.md"), 0o600); err != nil {
		t.Skip("mkfifo unsupported:", err)
	}

	done := make(chan *Index, 1)
	go func() {
		idx, _ := BuildFromRoot(root)
		done <- idx
	}()
	var idx *Index
	select {
	case idx = <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("index hung on a FIFO")
	}
	if idx == nil {
		t.Fatal("no index")
	}
	for _, n := range idx.Nodes {
		if n.Path == "docs/link.md" || n.Path == "docs/pipe.md" || n.Title == "Leaked heading" {
			t.Errorf("indexed an unsafe entry: %+v", n)
		}
	}

	if _, err := readSource(root, "docs/link.md"); err == nil {
		t.Error("readSource followed a symlink")
	}
	if _, err := readSource(root, "docs/pipe.md"); err == nil {
		t.Error("readSource read a FIFO")
	}
	if _, err := readSource(root, "docs/real.md"); err != nil {
		t.Errorf("readSource regular file: %v", err)
	}
}
