package main

import (
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/version"
)

func TestRootVersionFlag(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)
	out, code := runTBCapture(t, tbBin, "", "--version")
	if code != 0 {
		t.Fatalf("tb --version deveria sair 0\n%s", out)
	}
	info := version.Get()
	if !strings.Contains(out, info.Version) {
		t.Fatalf("stdout deveria conter versão %q:\n%s", info.Version, out)
	}
	if strings.Contains(out, "unknown flag") {
		t.Fatalf("tb --version não deve reportar unknown flag:\n%s", out)
	}
}
