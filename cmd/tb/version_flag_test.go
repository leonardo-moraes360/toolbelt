package main

import (
	"strings"
	"testing"
)

// The built binary and the test binary carry different build info (the test
// binary has no vcs stamp), so only the shape of the line is compared.
func TestRootVersionFlag(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	out, code := runTBCapture(t, tbBin, "", "--version")
	if code != 0 {
		t.Fatalf("tb --version should exit 0\n%s", out)
	}
	first, _, _ := strings.Cut(out, "\n")
	if !strings.HasPrefix(first, "toolbelt ") || strings.TrimSpace(strings.TrimPrefix(first, "toolbelt ")) == "" {
		t.Fatalf("first line should be `toolbelt <version>`:\n%s", out)
	}
	if strings.Contains(out, "unknown flag") {
		t.Fatalf("tb --version must not report unknown flag:\n%s", out)
	}
}
