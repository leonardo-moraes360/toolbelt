package mdparse

import "strings"

// Heading is an ATX heading outside the frontmatter and fenced blocks (R6).
type Heading struct {
	Line  int // 1-based
	Level int
	Text  string
	Slug  string
}

// Headings returns the ATX headings of src in order: up to 3 spaces of
// indentation, 1 to 6 "#", a space or tab, then the text; an optional
// closing sequence of "#" preceded by a space is removed.
func Headings(src string) []Heading {
	lines := Lines(src)
	mask := masked(lines)
	var out []Heading
	for i, l := range lines {
		if mask[i] {
			continue
		}
		if level, text, ok := atx(l); ok {
			out = append(out, Heading{Line: i + 1, Level: level, Text: text, Slug: Slugify(text)})
		}
	}
	return out
}

func atx(line string) (level int, text string, ok bool) {
	t := line
	for i := 0; i < 3 && strings.HasPrefix(t, " "); i++ {
		t = t[1:]
	}
	for level < len(t) && t[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level == len(t) || (t[level] != ' ' && t[level] != '\t') {
		return 0, "", false
	}
	text = strings.TrimSpace(t[level:])
	if j := strings.TrimRight(text, "#"); j != text && (j == "" || strings.HasSuffix(j, " ") || strings.HasSuffix(j, "\t")) {
		text = strings.TrimSpace(j)
	}
	if text == "" {
		return 0, "", false
	}
	return level, text, true
}
