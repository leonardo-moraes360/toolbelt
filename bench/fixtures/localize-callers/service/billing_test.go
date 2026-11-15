package service

import (
	"os"
	"strings"
	"testing"
)

func TestDiscounted(t *testing.T) {
	got := Discounted(10, 3, 5)
	if got != 25 {
		t.Fatalf("Discounted = %d, want 25", got)
	}
}

func TestReportCallers(t *testing.T) {
	data, err := os.ReadFile("../CALLERS.md")
	if err != nil {
		t.Fatalf("CALLERS.md ausente: %v", err)
	}
	text := string(data)
	needles := []string{"Scale", "Total", "Discounted", "mathutil"}
	for _, n := range needles {
		if !strings.Contains(text, n) {
			t.Fatalf("CALLERS.md incompleto (falta %q):\n%s", n, text)
		}
	}
}
