// Package contracttest compares a contract document against its JSON
// snapshot under testdata/contract, shared by the CLI and MCP freeze tests.
package contracttest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Check encodes doc deterministically and compares it with
// testdata/contract/<name>; with update it rewrites the snapshot instead.
// howTo is appended to every failure so the reader knows how to regenerate.
func Check(t testing.TB, name string, doc any, update bool, howTo string) {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "contract", name)
	if update {
		if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v\n%s", err, howTo)
	}
	if !bytes.Equal(want, buf.Bytes()) {
		t.Fatalf("%s differs from the code.\n%s\n\nfirst difference: %s", path, howTo, firstDiff(string(want), buf.String()))
	}
}

func firstDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var a, b string
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			b = g[i]
		}
		if a != b {
			return "line " + strconv.Itoa(i+1) + ": snapshot " + strconv.Quote(a) + ", now " + strconv.Quote(b)
		}
	}
	return "none"
}
