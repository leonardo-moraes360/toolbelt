package native

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManifestMatches keeps the engine constants equal to the vendored
// MANIFEST, so tb version never names another commit than the one built.
func TestManifestMatches(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "third_party", "llama.cpp", "MANIFEST"))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, l := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(l, ": "); ok && !strings.HasPrefix(k, "#") {
			got[k] = v
		}
	}
	if got["tag"] != EngineTag || got["commit"] != EngineCommit || got["license"] != "MIT" {
		t.Fatalf("MANIFEST %v vs %s %s", got, EngineTag, EngineCommit)
	}
}

func TestEngineString(t *testing.T) {
	e := Engine()
	if Available != strings.HasPrefix(e, EngineName+" "+EngineTag) {
		t.Fatalf("Engine() = %q with Available=%v", e, Available)
	}
}
