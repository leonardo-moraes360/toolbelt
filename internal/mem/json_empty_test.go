package mem

import (
	"encoding/json"
	"testing"
)

func TestListSearchEmptyAreArrays(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if _, err := s.Load(); err != nil {
		t.Fatal(err)
	}
	list, err := s.List("", false)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "[]" {
		t.Fatalf("list empty JSON=%s want []", raw)
	}
	if _, err := s.Add("anti_cheat", "keep hidden intact", "", false); err != nil {
		t.Fatal(err)
	}
	miss, err := s.Search("zzzz-none", false)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(miss)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "[]" {
		t.Fatalf("search empty JSON=%s want []", raw)
	}
	rep, err := s.Check("", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["missing"].([]any); !ok {
		t.Fatalf("missing should be []: %s", raw)
	}
}

func TestEnsureDefaultAntiCheatResult(t *testing.T) {
	dir := t.TempDir()
	res, err := EnsureDefaultAntiCheat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created || res.Count == 0 || res.Path == "" {
		t.Fatalf("unexpected init: %+v", res)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"path", "created", "count", "invariants"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing %s in %s", key, raw)
		}
	}
	again, err := EnsureDefaultAntiCheat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if again.Created {
		t.Fatal("second init should detect existing invariants")
	}
	if again.Count != res.Count {
		t.Fatalf("count=%d want %d", again.Count, res.Count)
	}
}
