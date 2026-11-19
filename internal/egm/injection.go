package egm

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
)

//go:embed injection_patterns.txt
var injectionPatternsTxt string

var injectionPatterns = compileInjectionPatterns(injectionPatternsTxt)

func compileInjectionPatterns(s string) []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, regexp.MustCompile("(?i)"+l))
		}
	}
	return out
}

const injectionChecker = "injection-scan"

type injectionMatch struct {
	line    int
	text    string
	context string
}

var reInlineCode = regexp.MustCompile("`[^`\n]+`")

// matchContext classifies where a match sits (R17a): code (fenced block or
// inline code), quote (line starting with >) or prose.
func matchContext(d *Doc, n int, line string, loc []int) string {
	if d.InFence(n) {
		return "code"
	}
	for _, c := range reInlineCode.FindAllStringIndex(line, -1) {
		if loc[0] >= c[0] && loc[1] <= c[1] {
			return "code"
		}
	}
	if strings.HasPrefix(strings.TrimLeft(line, " "), ">") {
		return "quote"
	}
	return "prose"
}

func scanLines(d *Doc) []injectionMatch {
	var out []injectionMatch
	for i, l := range d.Lines {
		for _, re := range injectionPatterns {
			for _, loc := range re.FindAllStringIndex(l, -1) {
				out = append(out, injectionMatch{line: i + 1, text: l[loc[0]:loc[1]], context: matchContext(d, i+1, l, loc)})
			}
		}
	}
	return out
}

func matchSHA(ms []injectionMatch) string {
	texts := make([]string, 0, len(ms))
	for _, m := range ms {
		texts = append(texts, strings.ToLower(m.text))
	}
	sort.Strings(texts)
	h := sha256.Sum256([]byte(strings.Join(texts, "\x00")))
	return hex.EncodeToString(h[:])
}

// ScanInjection sweeps the whole snapshot: the matches under the same heading
// form one major injection-scan finding whose scope is the union of the lines
// (R17a).
func ScanInjection(d *Doc, docPath string) []Finding {
	byHeading := map[string][]injectionMatch{}
	var order []string
	for _, m := range scanLines(d) {
		h := d.HeadingAt(m.line)
		if _, ok := byHeading[h]; !ok {
			order = append(order, h)
		}
		byHeading[h] = append(byHeading[h], m)
	}
	var out []Finding
	for _, h := range order {
		ms := byHeading[h]
		lo, hi := ms[0].line, ms[0].line
		for _, m := range ms {
			lo, hi = min(lo, m.line), max(hi, m.line)
		}
		out = append(out, Finding{
			Reviewer: injectionChecker,
			Severity: SevMajor,
			Scope:    Scope{Heading: h, Lines: []int{lo, hi}},
			Message:  "text that reads as an instruction to the reviewers: " + quoteList(ms),
			Evidence: []Evidence{{Path: docPath, Line: lo}},
			Key:      "injection:" + h,
			Context:  ms[0].context,
			MatchSHA: matchSHA(ms),
		})
	}
	return out
}

// ScanInjectionSource gives one finding per source file with matches.
func ScanInjectionSource(path, text string) *Finding {
	d := ParseDoc(text, nil)
	ms := scanLines(d)
	if len(ms) == 0 {
		return nil
	}
	return &Finding{
		Reviewer: injectionChecker,
		Severity: SevMajor,
		Scope:    Scope{Heading: "", Lines: []int{ms[0].line, ms[len(ms)-1].line}},
		Message:  "source " + path + " has text that reads as an instruction to the reviewers: " + quoteList(ms),
		Evidence: []Evidence{{Path: path, Line: ms[0].line}},
		Key:      "injection-src:" + path,
		Context:  "source",
		MatchSHA: matchSHA(ms),
	}
}

func quoteList(ms []injectionMatch) string {
	seen := map[string]bool{}
	var q []string
	for _, m := range ms {
		t := strings.ToLower(m.text)
		if !seen[t] {
			seen[t] = true
			q = append(q, "\""+t+"\"")
		}
	}
	return strings.Join(q, ", ")
}
