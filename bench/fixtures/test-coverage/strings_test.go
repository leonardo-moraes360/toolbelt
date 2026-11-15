package strings

import "testing"

func TestReverse(t *testing.T) {
	if got := Reverse("hello"); got != "olleh" {
		t.Errorf("Reverse(hello) = %q, want olleh", got)
	}
}
