package egm

import (
	"math"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
)

// TestTouchesHugeScope: model-chosen line numbers never drive a loop over
// the range (review finding: [5, MaxInt64] hung tb under the lock).
func TestTouchesHugeScope(t *testing.T) {
	done := make(chan bool, 1)
	go func() { done <- Touches([]int{5, math.MaxInt}, map[int]bool{3: true}, 5) }()
	select {
	case got := <-done:
		if !got {
			t.Fatal("line 3 is within 5 of line 5")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Touches loops over the scope range")
	}
	if Touches([]int{100, math.MaxInt}, map[int]bool{3: true}, 5) {
		t.Fatal("line 3 is not near [100, max]")
	}
}

func TestClampLines(t *testing.T) {
	l := []int{5, math.MaxInt}
	if !clampLines(l, 10) || l[1] != 10 || l[0] != 5 {
		t.Fatalf("%v", l)
	}
	if clampLines([]int{1, 3}, 10) {
		t.Fatal("in range")
	}
}

func TestNoRetryInterrupted(t *testing.T) {
	for _, r := range []string{"interrupted", "tools_used", "escaped_process", "budget"} {
		if !noRetry(r) {
			t.Errorf("%s must not be retried", r)
		}
	}
	if noRetry("timeout") || noRetry("invalid_schema") {
		t.Error("timeout and invalid output are retried (R16)")
	}
}

// TestForgedOwnerIsUnattributed: an orphan declaring an owner this runner
// never started cannot pick a victim or hide (review finding 5).
func TestForgedOwnerIsUnattributed(t *testing.T) {
	r := &Runner{escaped: map[string]bool{}, known: map[string]bool{"egm:x:r1:a": true}, running: map[*runSlot]struct{}{}}
	r.attributeLocked(&sandbox.Orphan{Vars: map[string]string{envOwner: "bogus"}})
	if !r.unattributed || len(r.escaped) != 0 {
		t.Fatalf("unattributed=%v escaped=%v", r.unattributed, r.escaped)
	}
	r.ResetUnattributed()
	r.attributeLocked(&sandbox.Orphan{Vars: map[string]string{envOwner: "egm:x:r1:a"}})
	if r.unattributed || !r.escaped["egm:x:r1:a"] {
		t.Fatalf("known owner: unattributed=%v escaped=%v", r.unattributed, r.escaped)
	}
}
