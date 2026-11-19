package egm

import (
	"regexp"
	"strings"
)

// Doc is a markdown snapshot split into lines (1-based in every public use).
type Doc struct {
	Lines []string
	// heading[i] is the heading line (trimmed, with the #) that governs line
	// i+1, or "" before the first heading.
	heading []string
	// fence[i] is true when line i+1 is inside a fenced code block or is a
	// fence delimiter.
	fence []bool
	// Feature and Detailed are the 1-based inclusive ranges of R6; Detailed is
	// {0,0} when the document has no detailed-plan heading.
	Feature, Detailed [2]int
}

var reATX = regexp.MustCompile(`^#{1,6} `)

// ParseDoc builds the line model. detailHeadings are the whole heading lines
// (egm.detail_headings) that open the DetailedRange (R6).
func ParseDoc(text string, detailHeadings []string) *Doc {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	d := &Doc{Lines: lines, heading: make([]string, len(lines)), fence: make([]bool, len(lines))}
	cur, fenceMark := "", ""
	cut := 0
	for i, l := range lines {
		if fenceMark != "" {
			d.fence[i] = true
			if strings.HasPrefix(strings.TrimSpace(l), fenceMark) {
				fenceMark = ""
			}
			d.heading[i] = cur
			continue
		}
		if m := fenceOpen(l); m != "" {
			fenceMark = m
			d.fence[i] = true
			d.heading[i] = cur
			continue
		}
		if reATX.MatchString(l) {
			cur = strings.TrimSpace(l)
			if cut == 0 && containsStr(detailHeadings, cur) {
				cut = i + 1
			}
		}
		d.heading[i] = cur
	}
	n := len(lines)
	if cut == 0 {
		d.Feature, d.Detailed = [2]int{1, n}, [2]int{0, 0}
	} else {
		d.Feature, d.Detailed = [2]int{1, cut - 1}, [2]int{cut, n}
	}
	return d
}

func fenceOpen(l string) string {
	t := strings.TrimLeft(l, " ")
	if len(l)-len(t) > 3 {
		return ""
	}
	for _, m := range []string{"```", "~~~"} {
		if strings.HasPrefix(t, m) {
			return m
		}
	}
	return ""
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if strings.TrimSpace(x) == s {
			return true
		}
	}
	return false
}

// HasDetailed reports whether the document has a DetailedRange.
func (d *Doc) HasDetailed() bool { return d.Detailed[0] > 0 }

// HeadingAt returns the heading that governs line n (1-based).
func (d *Doc) HeadingAt(n int) string {
	if n < 1 || n > len(d.Lines) {
		return ""
	}
	return d.heading[n-1]
}

// InFence reports whether line n (1-based) is code in a fenced block.
func (d *Doc) InFence(n int) bool {
	return n >= 1 && n <= len(d.Lines) && d.fence[n-1]
}

// Headings returns the heading lines, in order, inside range r.
func (d *Doc) Headings(r [2]int) []string {
	var out []string
	for n := max(1, r[0]); n <= r[1] && n <= len(d.Lines); n++ {
		if !d.fence[n-1] && reATX.MatchString(d.Lines[n-1]) {
			out = append(out, strings.TrimSpace(d.Lines[n-1]))
		}
	}
	return out
}

// HasHeading reports whether h (whole line, trimmed) is a heading of the
// document.
func (d *Doc) HasHeading(h string) bool {
	return containsStr(d.Headings([2]int{1, len(d.Lines)}), strings.TrimSpace(h))
}

// HeadingLine returns the 1-based line of heading h, or 0.
func (d *Doc) HeadingLine(h string) int {
	h = strings.TrimSpace(h)
	for n := 1; n <= len(d.Lines); n++ {
		if !d.fence[n-1] && strings.TrimSpace(d.Lines[n-1]) == h && reATX.MatchString(d.Lines[n-1]) {
			return n
		}
	}
	return 0
}
