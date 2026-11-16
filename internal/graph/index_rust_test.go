package graph

import (
	"path/filepath"
	"testing"
)

func TestRustCallsSameModule(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "src", "lib.rs"), `
pub fn helper(x: i32) -> i32 {
    x + 1
}

pub fn run(x: i32) -> i32 {
    helper(x)
}

pub struct Worker;

impl Worker {
    pub fn execute(&self, x: i32) -> i32 {
        run(x)
    }
}
`)
	opts := DefaultBuildOptions()
	opts.Languages = []string{"rust"}
	opts.Docs = []string{}
	opts.Frameworks = []string{}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}

	if !hasCallEdge(idx, "run", "helper") {
		t.Fatalf("expected calls edge run→helper; calls=%v", callEdges(idx))
	}
	if !hasCallEdge(idx, "execute", "run") {
		t.Fatalf("expected calls edge execute→run; calls=%v", callEdges(idx))
	}

	store := NewStore(filepath.Join(t.TempDir(), "graph.json"))
	store.index = idx
	callers := store.Callers("helper", 1)
	if len(callers) == 0 {
		t.Fatal("callers(helper) should not be empty")
	}
	found := false
	for _, c := range callers {
		if c.Title == "run" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected caller run, got %#v", callers)
	}
}

func TestRustCallsCrossFileResolved(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "src", "range.rs"), `
pub fn largest_offset_from_end(n: usize) -> usize {
    n
}
`)
	mustWrite(t, filepath.Join(root, "src", "controller.rs"), `
use crate::range::largest_offset_from_end;

pub fn paint(n: usize) -> usize {
    largest_offset_from_end(n)
}
`)
	opts := DefaultBuildOptions()
	opts.Languages = []string{"rust"}
	opts.Docs = []string{}
	opts.Frameworks = []string{}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}

	store := NewStore(filepath.Join(t.TempDir(), "graph.json"))
	store.index = idx
	callers := store.Callers("largest_offset_from_end", 1)
	if len(callers) == 0 {
		t.Fatalf("callers cross-file empty; calls=%v", callEdges(idx))
	}
	found := false
	for _, c := range callers {
		if c.Title == "paint" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected caller paint, got %#v edges=%v", callers, callEdges(idx))
	}
}

func TestRustCallsSkipKeywordsAndUnresolved(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "src", "main.rs"), `
pub fn process(items: &[i32]) -> i32 {
    if items.is_empty() {
        return 0;
    }
    let n = items.len();
    println!("{}", n);
    missing_elsewhere(n);
    n as i32
}
`)
	opts := DefaultBuildOptions()
	opts.Languages = []string{"rust"}
	opts.Docs = []string{}
	opts.Frameworks = []string{}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range idx.Edges {
		if e.Type != EdgeCalls {
			continue
		}
		t.Fatalf("expected no resolvable calls edges, got %#v", e)
	}
}

func TestRustImpactUsesCalls(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "src", "core.rs"), `
pub fn scrub(data: &str) -> &str {
    data
}
`)
	mustWrite(t, filepath.Join(root, "src", "api.rs"), `
use crate::core::scrub;

pub fn handle(event: &str) -> &str {
    scrub(event)
}
`)
	opts := DefaultBuildOptions()
	opts.Languages = []string{"rust"}
	opts.Docs = []string{}
	opts.Frameworks = []string{}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(filepath.Join(t.TempDir(), "graph.json"))
	store.index = idx
	impact := store.Impact("src/core.rs", 2)
	if len(impact) == 0 {
		t.Fatalf("impact empty; calls=%v", callEdges(idx))
	}
}
