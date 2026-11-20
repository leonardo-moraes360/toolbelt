package mdparse

import "strings"

// Wikilink is one [[...]] link (R7).
type Wikilink struct {
	Line     int // 1-based
	Col      int // 1-based, in runes, of the first "[" (after a "!")
	Target   string
	Fragment string
	Label    string
	Embed    bool
}

// Wikilinks extracts the wikilinks of src outside the frontmatter, fenced
// blocks and inline code spans. A candidate runs from "[[" to the next "]]"
// on the same line and is dropped when its inner text holds "[" or "]",
// or when both target and fragment are empty.
func Wikilinks(src string) []Wikilink {
	lines := Lines(src)
	mask := masked(lines)
	var out []Wikilink
	for i, l := range lines {
		if mask[i] || !strings.Contains(l, "[[") {
			continue
		}
		out = append(out, lineLinks(i+1, l)...)
	}
	return out
}

func lineLinks(n int, line string) []Wikilink {
	code := codeSpans(line)
	var out []Wikilink
	for i := 0; i+1 < len(line); {
		if line[i] != '[' || line[i+1] != '[' || code[i] {
			i++
			continue
		}
		end := strings.Index(line[i+2:], "]]")
		if end < 0 {
			break
		}
		inner := line[i+2 : i+2+end]
		next := i + 2 + end + 2
		if strings.ContainsAny(inner, "[]") || anyIn(code, i, next) {
			i++
			continue
		}
		if w, ok := parseInner(inner); ok {
			w.Line = n
			w.Embed = i > 0 && line[i-1] == '!'
			w.Col = runeCol(line, i)
			out = append(out, w)
		}
		i = next
	}
	return out
}

func anyIn(code []bool, from, to int) bool {
	for k := from; k < to && k < len(code); k++ {
		if code[k] {
			return true
		}
	}
	return false
}

func runeCol(line string, byteIdx int) int {
	return len([]rune(line[:byteIdx])) + 1
}

// parseInner applies R7's steps: label at the first "|" ("\|", the table
// form, counts too and loses its "\"), fragment at the first "#", target
// trimmed and without a ".md" suffix.
func parseInner(inner string) (Wikilink, bool) {
	var w Wikilink
	rest := inner
	if p := strings.IndexByte(rest, '|'); p >= 0 {
		w.Label = strings.TrimSpace(rest[p+1:])
		rest = strings.TrimSuffix(rest[:p], `\`)
	}
	if h := strings.IndexByte(rest, '#'); h >= 0 {
		w.Fragment = strings.TrimSpace(rest[h+1:])
		rest = rest[:h]
	}
	t := strings.TrimSpace(rest)
	if len(t) >= 3 && strings.EqualFold(t[len(t)-3:], ".md") {
		t = t[:len(t)-3]
	}
	w.Target = t
	if w.Target == "" && w.Fragment == "" {
		return Wikilink{}, false
	}
	return w, true
}

// codeSpans marks the bytes of line inside inline code: a run of N
// backticks opens a span closed by the next run of exactly N (CommonMark);
// an unmatched run is literal.
func codeSpans(line string) []bool {
	out := make([]bool, len(line))
	for i := 0; i < len(line); {
		if line[i] != '`' {
			i++
			continue
		}
		n := run(line, i)
		closeAt := -1
		for j := i + n; j < len(line); {
			if line[j] != '`' {
				j++
				continue
			}
			m := run(line, j)
			if m == n {
				closeAt = j
				break
			}
			j += m
		}
		if closeAt < 0 {
			i += n
			continue
		}
		for k := i; k < closeAt+n; k++ {
			out[k] = true
		}
		i = closeAt + n
	}
	return out
}

func run(s string, i int) int {
	n := 0
	for i+n < len(s) && s[i+n] == '`' {
		n++
	}
	return n
}
