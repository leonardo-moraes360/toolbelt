package main

import (
	"os/exec"
	"testing"
)

func TestGitDiffMap(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	old := "a\nb\nc\nd\ne\nf\n"
	// b changed, line inserted after d, f deleted.
	nw := "a\nB\nc\nd\nX\ne\n"
	m, err := gitDiffMap(old, nw)
	if err != nil {
		t.Fatal(err)
	}
	for in, want := range map[int]int{1: 1, 2: 2, 3: 3, 4: 4, 5: 6} {
		if got := m.mapLine(in); got != want {
			t.Errorf("mapLine(%d) = %d, want %d", in, got, want)
		}
	}
	ch := m.changed()
	for _, l := range []int{2, 5, 6, 7} {
		if !ch[l] {
			t.Errorf("line %d should be changed (set=%v)", l, ch)
		}
	}
	if ch[3] || ch[1] {
		t.Errorf("lines 1 and 3 should not be changed (set=%v)", ch)
	}
	if got := changedText(map[int]bool{2: true, 3: true, 7: true}); got != "2-3,7-7" {
		t.Errorf("changedText = %q", got)
	}
}

func TestTouchesAndRegion(t *testing.T) {
	ch := map[int]bool{10: true}
	if !touches([]int{12, 14}, ch, 3) || touches([]int{14, 16}, ch, 3) {
		t.Error("touches margin")
	}
	a, b := &finding{}, &finding{}
	a.Scope.Heading, b.Scope.Heading = "## X", "## X"
	a.Scope.Lines, b.Scope.Lines = []int{1, 5}, []int{8, 9}
	if !region(a, b) {
		t.Error("gap 3 should be in region")
	}
	b.Scope.Lines = []int{9, 9}
	if region(a, b) {
		t.Error("gap 4 should be out of region")
	}
}
