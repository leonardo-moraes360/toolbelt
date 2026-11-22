//go:build unix

package graph

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

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

func TestIndexConfigFIFOsDoNotHang(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"go.mod", "package.json", "pnpm-workspace.yaml", "tsconfig.json"} {
		if err := syscall.Mkfifo(filepath.Join(root, name), 0o600); err != nil {
			t.Skip("mkfifo unsupported:", err)
		}
	}
	mustWrite(t, filepath.Join(root, "a.go"), "package a\nfunc F() {}\n")
	mustWrite(t, filepath.Join(root, "b.ts"), "export function g() {}\n")
	done := make(chan struct{})
	go func() {
		_, _ = BuildFromRoot(root)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("index hung on a config FIFO")
	}
}
