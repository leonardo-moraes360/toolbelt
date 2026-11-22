package merge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

var reSHA40 = regexp.MustCompile(`[0-9a-f]{40}`)

// goldenJSON renders res with commit SHAs masked (they depend on timestamps)
// and compares it with testdata/<name>.golden.json. TB_GOLDEN_UPDATE=1 rewrites.
func goldenJSON(t *testing.T, name string, res *Result) {
	t.Helper()
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got := reSHA40.ReplaceAll(b, []byte("<sha>"))
	got = append(got, '\n')
	path := filepath.Join("testdata", name+".golden.json")
	if os.Getenv("TB_GOLDEN_UPDATE") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s drifted from the golden file:\n%s\nwant:\n%s", name, got, want)
	}
}

// The fail and last_wins JSON must stay byte-identical across releases (new
// strategies only add omitempty fields).
func TestGoldenFailAndLastWinsJSON(t *testing.T) {
	dir := conflictRepo(t)
	res, err := Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001", "TKT-002"}})
	if err == nil {
		t.Fatal("fail should conflict")
	}
	goldenJSON(t, "fail_conflict", res)

	dir = conflictRepo(t)
	res, err = Run(&Options{Repo: dir, Store: ticket.Open(dir), IDs: []string{"TKT-001", "TKT-002"}, Strategy: StrategyLastWins, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	goldenJSON(t, "last_wins_dry", res)
}
