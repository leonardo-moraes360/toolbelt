package review

import (
	"encoding/json"
	"testing"
)

func TestDriftReportEmptyListsAreArrays(t *testing.T) {
	rep := DetectDrift(&TicketScope{
		Scope:     []string{"a.go"},
		Forbidden: []string{"secrets/**"},
	}, []string{"a.go"}, "HEAD~1")
	raw, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"out_of_scope", "forbidden_hit"} {
		v, ok := payload[key]
		if !ok {
			t.Fatalf("missing %s", key)
		}
		if _, ok := v.([]any); !ok {
			t.Fatalf("%s=%T want []: %s", key, v, raw)
		}
	}
}
