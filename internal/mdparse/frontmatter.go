package mdparse

import (
	"errors"
	"strings"
)

// ErrUnterminatedFrontmatter is a "---" first line with no closing line.
var ErrUnterminatedFrontmatter = errors.New("unterminated frontmatter")

// Frontmatter is the YAML block at the top of a note (R4).
type Frontmatter struct {
	// YAML is the text between the markers, lines joined with "\n".
	YAML string
	// EndLine is the 1-based line of the closing marker; the body starts
	// on the next line.
	EndLine int
}

// SplitFrontmatter finds the frontmatter of src: the first line is "---"
// (CRLF accepted) and the block ends at the next line that is only "---"
// or "...". ok is false when src has no frontmatter.
func SplitFrontmatter(src string) (fm Frontmatter, ok bool, err error) {
	lines := Lines(src)
	if len(lines) == 0 || !isMarker(lines[0], false) {
		return Frontmatter{}, false, nil
	}
	end, found := frontmatterRange(lines)
	if !found {
		return Frontmatter{}, true, ErrUnterminatedFrontmatter
	}
	return Frontmatter{YAML: strings.Join(lines[1:end], "\n"), EndLine: end + 1}, true, nil
}

// frontmatterRange returns the 0-based index of the closing marker.
func frontmatterRange(lines []string) (int, bool) {
	if len(lines) == 0 || !isMarker(lines[0], false) {
		return 0, false
	}
	for i := 1; i < len(lines); i++ {
		if isMarker(lines[i], true) {
			return i, true
		}
	}
	return 0, false
}

// isMarker accepts "---" (and "..." when closing) with trailing spaces or
// tabs, as Obsidian does.
func isMarker(line string, closing bool) bool {
	t := strings.TrimRight(line, " \t")
	return t == "---" || (closing && t == "...")
}
