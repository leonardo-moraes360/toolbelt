// Package mdparse is the leaf Markdown parser shared by the vault and the
// graph (docs/specs/vault.md, R4, R6, R7, R13): slugs, fence-aware ATX
// headings, YAML frontmatter boundaries, Obsidian wikilinks and search
// normalization. It imports nothing from the project.
package mdparse

import "strings"

// Lines splits src into lines without their "\n" or "\r\n" terminators;
// a leading UTF-8 BOM (written by some Windows editors) is dropped.
func Lines(src string) []string {
	src = strings.TrimPrefix(src, "\ufeff")
	if src == "" {
		return nil
	}
	ls := strings.Split(strings.TrimSuffix(src, "\n"), "\n")
	for i, l := range ls {
		ls[i] = strings.TrimSuffix(l, "\r")
	}
	return ls
}

// masked reports, per line, whether it is frontmatter (including its
// markers) or inside a fenced code block (including the fences).
func masked(lines []string) []bool {
	out := make([]bool, len(lines))
	start := 0
	if fm, ok := frontmatterRange(lines); ok {
		for i := 0; i <= fm; i++ {
			out[i] = true
		}
		start = fm + 1
	}
	var fenceCh byte
	fenceLen := 0
	for i := start; i < len(lines); i++ {
		ch, n, rest := fenceOpen(lines[i])
		switch {
		case fenceLen == 0 && n > 0:
			fenceCh, fenceLen = ch, n
			out[i] = true
		case fenceLen > 0:
			out[i] = true
			if ch == fenceCh && n >= fenceLen && strings.TrimSpace(rest) == "" {
				fenceLen = 0
			}
		}
	}
	return out
}

// fenceOpen recognizes a fence line: up to 3 spaces, then 3 or more ` or ~.
// It returns the fence character, its length and the text after it.
func fenceOpen(line string) (ch byte, n int, rest string) {
	t := line
	for i := 0; i < 3 && strings.HasPrefix(t, " "); i++ {
		t = t[1:]
	}
	if t == "" || (t[0] != '`' && t[0] != '~') {
		return 0, 0, ""
	}
	c := t[0]
	for n < len(t) && t[n] == c {
		n++
	}
	if n < 3 {
		return 0, 0, ""
	}
	rest = t[n:]
	if c == '`' && strings.Contains(rest, "`") {
		return 0, 0, "" // a backtick info string cannot contain a backtick
	}
	return c, n, rest
}
