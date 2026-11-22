package verify

import (
	"strings"
	"testing"
)

// dogfood #10: with many packages the tail of `go test ./...` is all `ok` and
// `[no test files]`; the failing package printed early must still reach the
// reason.
func TestExcerptKeepsFailLinesBeyondTheTail(t *testing.T) {
	var b strings.Builder
	b.WriteString("--- FAIL: TestEarly (0.00s)\n    early_test.go:12: boom\nFAIL\tmod/early\t0.01s\n")
	for i := 0; i < 400; i++ {
		b.WriteString("ok  \tmod/pkg" + strings.Repeat("x", 20) + "\t0.01s\n?   \tmod/none\t[no test files]\n")
	}
	got := excerpt(b.String(), 600)
	if len(got) > 700 {
		t.Fatalf("excerpt too long: %d", len(got))
	}
	for _, want := range []string{"--- FAIL: TestEarly", "FAIL\tmod/early"} {
		if !strings.Contains(got, want) {
			t.Errorf("excerpt lost %q:\n%s", want, got)
		}
	}
}
