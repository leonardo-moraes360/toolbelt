package graph

import (
	"path/filepath"
	"testing"
)

func TestPythonCallsSameModule(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pkg", "util.py"), `
def helper(x):
    return x + 1

def run(x):
    return helper(x)

class Worker:
    def execute(self, x):
        return run(x)
`)
	opts := DefaultBuildOptions()
	opts.Languages = []string{"python"}
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

func TestPythonCallsCrossFileResolved(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pkg", "auth.py"), `
def build_digest_header(user, password):
    return user + password
`)
	mustWrite(t, filepath.Join(root, "pkg", "client.py"), `
from pkg.auth import build_digest_header

def login(user, password):
    return build_digest_header(user, password)
`)
	opts := DefaultBuildOptions()
	opts.Languages = []string{"python"}
	opts.Docs = []string{}
	opts.Frameworks = []string{}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}

	store := NewStore(filepath.Join(t.TempDir(), "graph.json"))
	store.index = idx
	callers := store.Callers("build_digest_header", 1)
	if len(callers) == 0 {
		t.Fatalf("callers cross-file empty; calls=%v", callEdges(idx))
	}
	found := false
	for _, c := range callers {
		if c.Title == "login" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected caller login, got %#v edges=%v", callers, callEdges(idx))
	}
}

func TestPythonCallsSkipBuiltinsAndUnresolved(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "app.py"), `
def process(items):
    n = len(items)
    print(n)
    missing_elsewhere(n)
    return n
`)
	opts := DefaultBuildOptions()
	opts.Languages = []string{"python"}
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

func TestPythonImpactUsesCalls(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "svc", "core.py"), `
def scrub_dict(data):
    return data
`)
	mustWrite(t, filepath.Join(root, "svc", "api.py"), `
def handle(event):
    return scrub_dict(event)
`)
	opts := DefaultBuildOptions()
	opts.Languages = []string{"python"}
	opts.Docs = []string{}
	opts.Frameworks = []string{}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(filepath.Join(t.TempDir(), "graph.json"))
	store.index = idx
	impact := store.Impact("svc/core.py", 2)
	if len(impact) == 0 {
		t.Fatalf("impact empty; calls=%v", callEdges(idx))
	}
}

func hasCallEdge(idx *Index, sourceTitle, targetTitle string) bool {
	byID := map[string]Node{}
	for _, n := range idx.Nodes {
		byID[n.ID] = n
	}
	for _, e := range idx.Edges {
		if e.Type != EdgeCalls {
			continue
		}
		src, okS := byID[e.SourceID]
		tgt, okT := byID[e.TargetID]
		if okS && okT && src.Title == sourceTitle && tgt.Title == targetTitle {
			return true
		}
	}
	return false
}

func callEdges(idx *Index) []string {
	byID := map[string]Node{}
	for _, n := range idx.Nodes {
		byID[n.ID] = n
	}
	var out []string
	for _, e := range idx.Edges {
		if e.Type != EdgeCalls {
			continue
		}
		src := e.SourceID
		tgt := e.TargetID
		if n, ok := byID[e.SourceID]; ok {
			src = n.Title
		}
		if n, ok := byID[e.TargetID]; ok {
			tgt = n.Title
		}
		out = append(out, src+"->"+tgt)
	}
	return out
}
