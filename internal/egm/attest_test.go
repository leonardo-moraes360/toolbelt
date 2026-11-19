package egm

import (
	"os"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
)

// attestDoc has 3 headings in lines 1-9 and quotable text under each.
const attestDoc = `# Title
The introduction explains what this document is about.
## Alpha
Alpha has a body line that is long enough to be quoted,
and it wraps onto a second line.
| col a | col b |
## Beta
Beta also carries a body line long enough to quote.

`

func TestAttestRequirement(t *testing.T) {
	cases := []struct {
		name, text string
		rng        [2]int
		q, h       int
		skip       bool
	}{
		{"three headings", attestDoc, [2]int{1, 9}, 3, 1, false},
		{"four headings", "# a\n## b\n## c\n## d\n" + strings.Repeat("some body text that is long enough\n", 3), [2]int{1, 7}, 3, 2, false},
		{"text without heading", "plain text that goes on long enough to need three quotes, yes indeed it does.\n", [2]int{1, 1}, 3, 0, false},
		{"little text", "# a\nshort\n", [2]int{1, 2}, 1, 1, false},
		{"headings only", "# a\n## b\n", [2]int{1, 2}, 1, 1, false},
		{"range_too_small", "# a\n\n   \n| |\n", [2]int{2, 4}, 0, 0, true},
		{"empty range", "# a\n", [2]int{0, 0}, 0, 0, true},
	}
	for _, c := range cases {
		q, h, skip := AttestRequirement(ParseDoc(c.text, nil), c.rng, 3)
		if q != c.q || h != c.h || skip != c.skip {
			t.Errorf("%s: got (%d,%d,%v), want (%d,%d,%v)", c.name, q, h, skip, c.q, c.h, c.skip)
		}
	}
}

func TestAttestValid(t *testing.T) {
	d := ParseDoc(attestDoc, nil)
	ok := []Quote{
		{"## Alpha", "## Alpha"},
		{"## Alpha", "long enough to be quoted, and it   wraps onto"}, // spans two lines
		{"# Title", "The introduction explains what"},
	}
	if !CheckAttestation(ok, d, [2]int{1, 9}, 3) {
		t.Fatal("valid attestation rejected")
	}
	// Table bars are removed and whitespace collapsed on both sides; a
	// heading quote may omit the #s; duplicates count once.
	table := []Quote{
		{"## Alpha", "second line. col a col b"},
		{"## Beta", "Beta"},
		{"## Beta", "Beta also carries a body line"},
	}
	if !CheckAttestation(table, d, [2]int{1, 9}, 3) {
		t.Fatal("normalized quotes rejected")
	}
	dup := []Quote{ok[1], ok[1], {"## Beta", " Beta  also carries a body line"}}
	if CheckAttestation(dup, d, [2]int{1, 9}, 3) {
		t.Fatal("duplicate quotes must count once")
	}
}

func TestAttestInvalidQuotes(t *testing.T) {
	d := ParseDoc(attestDoc, nil)
	good := []Quote{{"## Alpha", "## Alpha"}, {"## Beta", "## Beta"}}
	bad := map[string]Quote{
		"wrong heading":   {"## Beta", "Alpha has a body line that is long"},
		"not literal":     {"## Alpha", "Alpha has a body line that is short"},
		"too short":       {"## Alpha", "Alpha has a body"},
		"too long":        {"## Alpha", strings.Repeat("x", 201)},
		"unknown heading": {"## Gamma", "Alpha has a body line that is long"},
		"empty":           {"## Alpha", "   "},
		"line prefix":     {"## Alpha", "4| Alpha has a body line that is long"},
	}
	for name, q := range bad {
		if CheckAttestation(append(append([]Quote{}, good...), q), d, [2]int{1, 9}, 3) {
			t.Errorf("%s: accepted", name)
		}
	}
	// Out of range: the quote is literal but its lines are outside rng.
	outside := []Quote{{"## Beta", "## Beta"}, {"## Beta", "Beta also carries a body line"}, {"## Alpha", "Alpha has a body line that is long"}}
	if CheckAttestation(outside, d, [2]int{7, 9}, 3) {
		t.Error("quote outside the range accepted")
	}
}

func TestAttestHeadingsOnlyRejected(t *testing.T) {
	d := ParseDoc(attestDoc, nil)
	only := []Quote{{"# Title", "# Title"}, {"## Alpha", "Alpha"}, {"## Beta", "## Beta"}}
	if CheckAttestation(only, d, [2]int{1, 9}, 3) {
		t.Fatal("headings-only attestation accepted over quotable text")
	}
	// Without quotable text, heading quotes are enough.
	tiny := ParseDoc("# a\nshort\n## b\n", nil)
	if !CheckAttestation([]Quote{{"# a", "# a"}}, tiny, [2]int{1, 3}, 3) {
		t.Fatal("heading quote rejected in a range without quotable text")
	}
}

func TestAttestCoverage(t *testing.T) {
	text := "# a\n## b\n## c\n## d\n" + "body under d that is long enough to quote\n"
	d := ParseDoc(text, nil)
	// H=4 → 2 distinct headings; under Q×20 characters of text → 1 quote.
	if q, h, _ := AttestRequirement(d, [2]int{1, 5}, 3); q != 1 || h != 2 {
		t.Fatalf("requirement %d %d", q, h)
	}
	one := []Quote{{"## d", "body under d that is long enough"}}
	if CheckAttestation(one, d, [2]int{1, 5}, 3) {
		t.Fatal("one heading covered, two required")
	}
	if !CheckAttestation(append(one, Quote{"## c", "c"}), d, [2]int{1, 5}, 3) {
		t.Fatal("two headings covered rejected")
	}
}

func TestAttestTextWithoutHeading(t *testing.T) {
	text := "# Top\n" + strings.Repeat("plain prose that is long enough for one quote here\n", 3)
	d := ParseDoc(text, nil)
	// Range 2-4 has no heading: zero headings and empty heading, even though
	// "# Top" governs those lines from outside the range.
	if q, h, skip := AttestRequirement(d, [2]int{2, 4}, 3); q != 3 || h != 0 || skip {
		t.Fatalf("requirement %d %d %v", q, h, skip)
	}
	ok := []Quote{{"", "plain prose that is long"}, {"", "long enough for one quote"}, {"", "quote here plain prose"}}
	if !CheckAttestation(ok, d, [2]int{2, 4}, 3) {
		t.Fatal("empty-heading quotes rejected")
	}
	ok[0].Heading = "# Top"
	if CheckAttestation(ok, d, [2]int{2, 4}, 3) {
		t.Fatal("a heading outside the range must not be declared")
	}
	if !CheckAttestation(nil, d, [2]int{5, 5}, 3) {
		t.Fatal("a skipped range has nothing to attest")
	}
}

// TestAttestSpecDetailed attests this repository's own spec over its
// DetailedRange, the range implementation-readiness reviews.
func TestAttestSpecDetailed(t *testing.T) {
	b, err := os.ReadFile("../../docs/specs/egm-nativo.md")
	if err != nil {
		t.Fatal(err)
	}
	d := ParseDoc(string(b), config.DefaultEgmConfig().DetailHeadings)
	if !d.HasDetailed() {
		t.Fatal("spec has no DetailedRange")
	}
	q, h, skip := AttestRequirement(d, d.Detailed, 3)
	if skip || q != 3 || h < 1 {
		t.Fatalf("requirement %d %d %v", q, h, skip)
	}
	checked := []Quote{
		{"## Detailed Implementation Plan", "## Detailed Implementation Plan"},
		{"## Detailed Implementation Plan", "| `internal/egm/attest.go` | Create | Attestation (R17) |"},
		{"### Verification Plan", "script in `bench/egm-spike/`"},
	}
	if !CheckAttestation(checked, d, d.Detailed, 3) {
		t.Fatal("valid attestation over the spec's DetailedRange rejected")
	}
	if CheckAttestation(checked[:1], d, d.Detailed, 3) {
		t.Fatal("headings-only attestation accepted")
	}
	// The same quotes are outside the FeatureRange.
	if CheckAttestation(checked, d, d.Feature, 3) {
		t.Fatal("DetailedRange quotes accepted for the FeatureRange")
	}
}
