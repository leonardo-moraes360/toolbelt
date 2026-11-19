package egm

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// LineMap is the result of `git diff --no-index -U0 old new` between two
// snapshots (R20).
type LineMap struct {
	hunks []hunk
}

type hunk struct {
	oldStart, oldLen, newStart, newLen int
	// blank is true when the hunk only changes whitespace (R28 ignores it).
	blank bool
}

var reHunk = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// DiffSnapshots diffs two texts by writing them under scratch (inside the
// state directory) and running DiffNoIndex there.
func DiffSnapshots(ctx context.Context, scratch, oldText, newText string) (*LineMap, error) {
	if oldText == newText {
		return &LineMap{}, nil
	}
	dir, err := os.MkdirTemp(scratch, "diff-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte(oldText), 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(b, []byte(newText), 0o600); err != nil {
		return nil, err
	}
	out, err := DiffNoIndex(ctx, dir, "a", "b")
	if err != nil {
		return nil, err
	}
	return parseHunks(out)
}

func atoiDef(s string, def int) int {
	if s == "" {
		return def
	}
	n, _ := strconv.Atoi(s)
	return n
}

func parseHunks(out []byte) (*LineMap, error) {
	m := &LineMap{}
	var cur *hunk
	var oldB, newB strings.Builder
	flush := func() {
		if cur != nil {
			cur.blank = squash(oldB.String()) == squash(newB.String())
			m.hunks = append(m.hunks, *cur)
		}
		oldB.Reset()
		newB.Reset()
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		l := sc.Text()
		if g := reHunk.FindStringSubmatch(l); g != nil {
			flush()
			cur = &hunk{oldStart: atoiDef(g[1], 0), oldLen: atoiDef(g[2], 1), newStart: atoiDef(g[3], 0), newLen: atoiDef(g[4], 1)}
			continue
		}
		if cur == nil || l == "" {
			continue
		}
		switch l[0] {
		case '-':
			oldB.WriteString(l[1:] + "\n")
		case '+':
			newB.WriteString(l[1:] + "\n")
		}
	}
	flush()
	return m, sc.Err()
}

func squash(s string) string { return strings.Join(strings.Fields(s), "") }

// Map maps an old line to the new snapshot. A line inside a changed hunk maps
// to the hunk's first new line (or the deletion point).
func (m *LineMap) Map(x int) int {
	if m == nil {
		return x
	}
	off := 0
	for _, h := range m.hunks {
		if h.oldLen == 0 {
			if x <= h.oldStart {
				return max(1, x+off)
			}
			off += h.newLen
			continue
		}
		switch {
		case x < h.oldStart:
			return max(1, x+off)
		case x < h.oldStart+h.oldLen:
			return max(1, h.newStart)
		}
		off += h.newLen - h.oldLen
	}
	return max(1, x+off)
}

// MapScope maps a scope's lines to the new snapshot.
func (m *LineMap) MapScope(s Scope) Scope {
	if len(s.Lines) != 2 {
		return s
	}
	a, b := m.Map(s.Lines[0]), m.Map(s.Lines[1])
	return Scope{Heading: s.Heading, Lines: []int{a, max(a, b)}}
}

// Changed returns the new-snapshot lines touched by non-whitespace hunks,
// plus both neighbors of each deletion point.
func (m *LineMap) Changed() map[int]bool {
	set := map[int]bool{}
	if m == nil {
		return set
	}
	for _, h := range m.hunks {
		if h.blank {
			continue
		}
		if h.newLen > 0 {
			for l := h.newStart; l < h.newStart+h.newLen; l++ {
				set[l] = true
			}
		} else {
			set[h.newStart], set[h.newStart+1] = true, true
		}
	}
	return set
}

// Touches reports whether any line of lines±margin is in changed. A scope
// without lines touches whenever anything changed.
func Touches(lines []int, changed map[int]bool, margin int) bool {
	if len(lines) != 2 {
		return len(changed) > 0
	}
	for l := lines[0] - margin; l <= lines[1]+margin; l++ {
		if changed[l] {
			return true
		}
	}
	return false
}
