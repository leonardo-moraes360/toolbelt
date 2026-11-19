package egm

import (
	"strings"
	"unicode/utf8"
)

// Quote is one entry of the attestation "checked" list (R17).
type Quote struct {
	Heading string `json:"heading"`
	Quote   string `json:"quote"`
}

// Quote length bounds for non-heading quotes (R17): long enough to be
// specific, short enough that copying the document is not an attestation.
const (
	minQuoteLen = 20
	maxQuoteLen = 200
)

// attestSection is the body text under one heading inside the range,
// normalized and joined so that a quote may span wrapped lines.
type attestSection struct {
	heading string
	body    string
}

// attestView is the part of a snapshot that an attestation is checked
// against: the headings inside the range and the body text by heading.
type attestView struct {
	headings  []string
	sections  []attestSection
	textChars int
}

// normQuote applies the R17 comparison normalization: "|" removed (tables
// are quoted without their cell bars) and whitespace collapsed.
func normQuote(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "|", "")), " ")
}

func isHeadingLine(d *Doc, n int) bool {
	return !d.InFence(n) && reATX.MatchString(d.Lines[n-1])
}

// buildAttestView splits the range into sections. A line belongs to the
// last heading of the range above it, or to "" when no heading of the range
// precedes it: a heading outside the range may be invisible to the goldfish
// (the arbiter sees only excerpts), so it cannot be required (R17).
func buildAttestView(d *Doc, rng [2]int) *attestView {
	v := &attestView{headings: d.Headings(rng)}
	cur, heading := -1, ""
	for n := max(1, rng[0]); n <= rng[1] && n <= len(d.Lines); n++ {
		if isHeadingLine(d, n) {
			cur, heading = -1, strings.TrimSpace(d.Lines[n-1])
			continue
		}
		norm := normQuote(d.Lines[n-1])
		if norm == "" {
			continue
		}
		v.textChars += utf8.RuneCountInString(norm)
		if cur < 0 {
			v.sections = append(v.sections, attestSection{heading: heading, body: norm})
			cur = len(v.sections) - 1
			continue
		}
		v.sections[cur].body += " " + norm
	}
	return v
}

// quotable reports whether some section has enough text for a body quote.
func (v *attestView) quotable() bool {
	for i := range v.sections {
		if utf8.RuneCountInString(v.sections[i].body) >= minQuoteLen {
			return true
		}
	}
	return false
}

func (v *attestView) inRange(h string) bool {
	for _, x := range v.headings {
		if x == h {
			return true
		}
	}
	return false
}

// Quote kinds returned by classify.
const (
	quoteInvalid = iota
	quoteHeading
	quoteBody
)

func (v *attestView) classify(q Quote) (kind int, heading, norm string) {
	heading, norm = strings.TrimSpace(q.Heading), normQuote(q.Quote)
	if norm == "" {
		return quoteInvalid, heading, norm
	}
	if heading != "" && v.inRange(heading) &&
		(norm == normQuote(heading) || norm == normQuote(strings.TrimLeft(heading, "#"))) {
		return quoteHeading, heading, norm
	}
	if l := utf8.RuneCountInString(norm); l < minQuoteLen || l > maxQuoteLen {
		return quoteInvalid, heading, norm
	}
	for i := range v.sections {
		if v.sections[i].heading == heading && strings.Contains(v.sections[i].body, norm) {
			return quoteBody, heading, norm
		}
	}
	return quoteInvalid, heading, norm
}

func (v *attestView) requirement(minQuotes int) (quotes, headings int, skip bool) {
	h := len(v.headings)
	if h == 0 && v.textChars == 0 {
		return 0, 0, true
	}
	quotes = max(1, minQuotes)
	if v.textChars < quotes*minQuoteLen {
		quotes = 1
	}
	// min(H, max(1, ceil(H/3))); with H == 0 this is 0.
	headings = min(h, max(1, (h+2)/3))
	return quotes, headings, false
}

// AttestRequirement returns the R17 minimum for range rng: the number of
// quotes and of distinct headings. skip is true when the range has neither a
// heading nor text (the reviewer is skipped with range_too_small).
func AttestRequirement(d *Doc, rng [2]int, minQuotes int) (quotes, headings int, skip bool) {
	return buildAttestView(d, rng).requirement(minQuotes)
}

// CheckAttestation validates a "checked" list against the snapshot (R17).
// Every entry must be valid; duplicates count once. A skipped range has
// nothing to attest, so it accepts any list.
func CheckAttestation(checked []Quote, d *Doc, rng [2]int, minQuotes int) bool {
	v := buildAttestView(d, rng)
	needQuotes, needHeadings, skip := v.requirement(minQuotes)
	if skip {
		return true
	}
	seen := map[Quote]bool{}
	covered := map[string]bool{}
	hasBody := false
	for _, c := range checked {
		kind, heading, norm := v.classify(c)
		if kind == quoteInvalid {
			return false
		}
		seen[Quote{Heading: heading, Quote: norm}] = true
		hasBody = hasBody || kind == quoteBody
		if v.inRange(heading) {
			covered[heading] = true
		}
	}
	if len(seen) < needQuotes || len(covered) < needHeadings {
		return false
	}
	return hasBody || !v.quotable()
}
