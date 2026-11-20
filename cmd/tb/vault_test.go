package main

import "testing"

func TestVaultTextEscapes(t *testing.T) {
	if got := one("evil\ninjected: error"); got != `"evil\ninjected: error"` {
		t.Fatalf("one: %s", got)
	}
	if got := one("plain"); got != "plain" {
		t.Fatalf("one plain: %s", got)
	}
	if got := block("a\n\x1b[31mred\tok\x07"); got != "a\n[31mred\tok" {
		t.Fatalf("block: %q", got)
	}
}
