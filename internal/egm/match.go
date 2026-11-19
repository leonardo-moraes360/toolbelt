package egm

import (
	_ "embed"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

//go:embed stopwords.txt
var stopwordsTxt string

var stopwords = parseStopwords(stopwordsTxt)

func parseStopwords(s string) map[string]bool {
	out := map[string]bool{}
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			out[l] = true
		}
	}
	return out
}

// Normalize lowercases, strips accents and punctuation, drops stopwords and
// cuts each token to its first stem runes (0 keeps whole words) (R18).
func Normalize(msg string, stem int) map[string]bool {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	s, _, err := transform.String(t, strings.ToLower(msg))
	if err != nil {
		s = strings.ToLower(msg)
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s)
	out := map[string]bool{}
	for _, w := range strings.Fields(s) {
		if stopwords[w] {
			continue
		}
		if r := []rune(w); stem > 0 && len(r) > stem {
			w = string(r[:stem])
		}
		out[w] = true
	}
	return out
}

// Sim is the Jaccard of the normalized tokens (R18).
func Sim(a, b string, stem int) float64 {
	ta, tb := Normalize(a, stem), Normalize(b, stem)
	if len(ta) == 0 && len(tb) == 0 {
		return 0
	}
	inter := 0
	for w := range ta {
		if tb[w] {
			inter++
		}
	}
	return float64(inter) / float64(len(ta)+len(tb)-inter)
}

// Region holds when both scopes share the heading and, when both carry lines,
// their gap is at most gap lines (R18).
func Region(a, b Scope, gap int) bool {
	if strings.TrimSpace(a.Heading) != strings.TrimSpace(b.Heading) {
		return false
	}
	if len(a.Lines) != 2 || len(b.Lines) != 2 {
		return true
	}
	return max(0, max(a.Lines[0], b.Lines[0])-min(a.Lines[1], b.Lines[1])) <= gap
}
