package egm

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// Invalid-output reasons (R16). All of them are retryable.
const (
	ReasonInvalidSchema   = "invalid_schema"
	ReasonOverSeverity    = "over_severity"
	ReasonWrongNonce      = "wrong_nonce"
	ReasonBadAttestation  = "bad_attestation"
	ReasonIncompletePrior = "incomplete_prior"
)

// Arbiter verdicts (R29).
const (
	VerdictWithdraw = "withdraw"
	VerdictUphold   = "uphold"
)

// PriorAnswer is the reviewer's re-evaluation of one owned lineage (R20).
type PriorAnswer struct {
	Lineage    string `json:"lineage"`
	StillHolds bool   `json:"still_holds"`
}

// ReviewerOutput is a validated reviewer payload (R15). Findings carry the
// reviewer name and the unverified / out_of_range marks (R16a, R9); the
// occurrence ID is assigned by the round.
type ReviewerOutput struct {
	Reviewer string        `json:"reviewer"`
	Nonce    string        `json:"nonce"`
	Checked  []Quote       `json:"checked"`
	Findings []Finding     `json:"findings"`
	Prior    []PriorAnswer `json:"prior"`
}

// ValidateInput is what a reviewer payload is checked against.
type ValidateInput struct {
	Reviewer, Nonce, MaxSeverity string
	PriorIDs                     []string // every one must be answered (R20)
	Doc                          *Doc     // redacted document snapshot
	DocPath                      string
	Range                        [2]int // reviewer range: attestation and out_of_range
	MinQuotes                    int
	Sources                      map[string]*Doc // packet sources; the document counts too
}

// ArbiterOutput is a validated arbiter payload (R29).
type ArbiterOutput struct {
	Nonce     string  `json:"nonce"`
	Checked   []Quote `json:"checked"`
	Verdict   string  `json:"verdict"`
	Rationale string  `json:"rationale"`
}

// ParseReviewerOutput validates a reviewer payload (envelope already removed).
// It returns the output and "" when valid, or nil and one of the Reason*
// codes. The payload may be wrapped in a ```json fence; any other text
// around the JSON object is rejected.
func ParseReviewerOutput(payload []byte, in *ValidateInput) (out *ReviewerOutput, reason string) {
	out = &ReviewerOutput{}
	if !decodeChecked(payload, reviewerShape, out) || out.Reviewer != in.Reviewer || !scopesOrdered(out.Findings) {
		return nil, ReasonInvalidSchema
	}
	if out.Nonce != in.Nonce {
		return nil, ReasonWrongNonce
	}
	if !withinCap(out.Findings, in.MaxSeverity) {
		return nil, ReasonOverSeverity
	}
	if !priorComplete(out.Prior, in.PriorIDs) {
		return nil, ReasonIncompletePrior
	}
	if !CheckAttestation(out.Checked, in.Doc, in.Range, in.MinQuotes) {
		return nil, ReasonBadAttestation
	}
	for i := range out.Findings {
		annotateFinding(&out.Findings[i], in)
	}
	return out, ""
}

// ParseArbiterOutput validates an arbiter payload; the attestation is over
// the range rng of d (the scope and evidence excerpt, R29).
func ParseArbiterOutput(payload []byte, nonce string, d *Doc, rng [2]int, minQuotes int) (out *ArbiterOutput, reason string) {
	out = &ArbiterOutput{}
	if !decodeChecked(payload, arbiterShape, out) {
		return nil, ReasonInvalidSchema
	}
	if out.Nonce != nonce {
		return nil, ReasonWrongNonce
	}
	if !CheckAttestation(out.Checked, d, rng, minQuotes) {
		return nil, ReasonBadAttestation
	}
	return out, ""
}

// decodeChecked parses exactly one JSON value, checks it against the shape
// and then decodes it into dst. The shape check runs on the generic tree so
// that unknown keys and wrong types are rejected, which encoding/json alone
// would silently accept.
func decodeChecked(payload []byte, s *shape, dst any) bool {
	b := stripFence(payload)
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return false
	}
	if len(bytes.TrimSpace(b[dec.InputOffset():])) != 0 {
		return false
	}
	return s.check(v) && json.Unmarshal(b, dst) == nil
}

// stripFence removes a surrounding ```json (or bare ```) fence, which models
// add often enough that rejecting it would only burn retries.
func stripFence(payload []byte) []byte {
	t := bytes.TrimSpace(payload)
	if !bytes.HasPrefix(t, []byte("```")) {
		return t
	}
	first, rest, ok := bytes.Cut(t, []byte("\n"))
	if !ok {
		return t
	}
	if tag := strings.TrimSpace(string(first[3:])); tag != "" && !strings.EqualFold(tag, "json") {
		return t
	}
	rest = bytes.TrimSpace(rest)
	inner, ok := bytes.CutSuffix(rest, []byte("```"))
	if !ok {
		return t
	}
	return bytes.TrimSpace(inner)
}

// scopesOrdered checks what the schema cannot express: lines[0] <= lines[1].
func scopesOrdered(fs []Finding) bool {
	for i := range fs {
		if fs[i].Scope.Lines[0] > fs[i].Scope.Lines[1] {
			return false
		}
	}
	return true
}

// withinCap enforces the persona's max_severity (R11). "none" (arbiter)
// admits no finding; "" means no cap.
func withinCap(fs []Finding, maxSeverity string) bool {
	limit := sevRank(SevBlocker)
	if maxSeverity != "" {
		limit = sevRank(maxSeverity)
	}
	for i := range fs {
		if sevRank(fs[i].Severity) > limit {
			return false
		}
	}
	return true
}

// priorComplete requires exactly one answer per owned lineage (R20): a
// missing, duplicated or unknown lineage invalidates the output.
func priorComplete(answers []PriorAnswer, ids []string) bool {
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	got := make(map[string]bool, len(answers))
	for _, a := range answers {
		if !want[a.Lineage] || got[a.Lineage] {
			return false
		}
		got[a.Lineage] = true
	}
	return len(got) == len(want)
}

func annotateFinding(f *Finding, in *ValidateInput) {
	f.Reviewer = in.Reviewer
	// Lines past the snapshot are clamped (and the finding unverified): a
	// model-chosen [5, MaxInt64] must not drive loops over line ranges.
	beyond := clampLines(f.Scope.Lines, len(in.Doc.Lines))
	f.Unverified = beyond || !scopeHeadingExists(in.Doc, &f.Scope) || !evidenceVerified(f.Evidence, in)
	// R9: the range is only a focus; a scope not contained in it still
	// counts but is marked.
	f.OutOfRange = f.Scope.Lines[0] < in.Range[0] || f.Scope.Lines[1] > in.Range[1]
}

// scopeHeadingExists implements the heading half of R16a. An empty heading
// is real only for text before the document's first heading.
func scopeHeadingExists(d *Doc, s *Scope) bool {
	h := strings.TrimSpace(s.Heading)
	if h == "" {
		return d.HeadingAt(s.Lines[0]) == ""
	}
	return d.HasHeading(h)
}

// evidenceVerified implements the evidence half of R16a: each path:line
// must exist in the packet (the document or a source).
func evidenceVerified(ev []Evidence, in *ValidateInput) bool {
	for _, e := range ev {
		d := in.Sources[e.Path]
		if e.Path == in.DocPath {
			d = in.Doc
		}
		if d == nil || e.Line < 1 || e.Line > len(d.Lines) {
			return false
		}
	}
	return true
}

// shape is the subset of JSON Schema that schema/*.json uses. Objects never
// admit extra properties. TestSchemaInSync keeps these declarations and the
// published schema files identical.
type shape struct {
	Type      string
	Props     map[string]*shape
	Required  []string
	Items     *shape
	Enum      []string
	MinLength int
	Minimum   *int
	MinItems  int
	MaxItems  int // 0: unbounded
}

func (s *shape) check(v any) bool {
	switch s.Type {
	case "object":
		return s.checkObject(v)
	case "array":
		return s.checkArray(v)
	case "string":
		str, ok := v.(string)
		return ok && utf8.RuneCountInString(str) >= s.MinLength && (len(s.Enum) == 0 || containsName(s.Enum, str))
	case "integer":
		n, ok := v.(json.Number)
		if !ok {
			return false
		}
		i, err := n.Int64()
		return err == nil && (s.Minimum == nil || i >= int64(*s.Minimum))
	case "boolean":
		_, ok := v.(bool)
		return ok
	}
	return false
}

func (s *shape) checkObject(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	for _, k := range s.Required {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	for k, val := range m {
		p := s.Props[k]
		if p == nil || !p.check(val) {
			return false
		}
	}
	return true
}

func (s *shape) checkArray(v any) bool {
	a, ok := v.([]any)
	if !ok || len(a) < s.MinItems || (s.MaxItems > 0 && len(a) > s.MaxItems) {
		return false
	}
	for _, x := range a {
		if !s.Items.check(x) {
			return false
		}
	}
	return true
}

func intp(i int) *int { return &i }

var (
	nonEmpty    = &shape{Type: "string", MinLength: 1}
	quoteShape  = &shape{Type: "object", Required: []string{"heading", "quote"}, Props: map[string]*shape{"heading": {Type: "string"}, "quote": nonEmpty}}
	lineShape   = &shape{Type: "integer", Minimum: intp(1)}
	checkedList = &shape{Type: "array", Items: quoteShape}

	evidenceShape = &shape{Type: "object", Required: []string{"path", "line"}, Props: map[string]*shape{"path": nonEmpty, "line": lineShape}}
	scopeShape    = &shape{Type: "object", Required: []string{"heading", "lines"}, Props: map[string]*shape{
		"heading": {Type: "string"},
		"lines":   {Type: "array", Items: lineShape, MinItems: 2, MaxItems: 2},
	}}
	findingShape = &shape{Type: "object", Required: []string{"severity", "scope", "message"}, Props: map[string]*shape{
		"severity":  {Type: "string", Enum: []string{SevBlocker, SevMajor, SevNit}},
		"scope":     scopeShape,
		"message":   nonEmpty,
		"rationale": {Type: "string"},
		"evidence":  {Type: "array", Items: evidenceShape},
	}}
	priorShape = &shape{Type: "object", Required: []string{"lineage", "still_holds"}, Props: map[string]*shape{
		"lineage":     nonEmpty,
		"still_holds": {Type: "boolean"},
	}}

	// reviewerShape mirrors schema/finding.schema.json (R15).
	reviewerShape = &shape{Type: "object", Required: []string{"reviewer", "nonce", "checked", "findings", "prior"}, Props: map[string]*shape{
		"reviewer": nonEmpty,
		"nonce":    nonEmpty,
		"checked":  checkedList,
		"findings": {Type: "array", Items: findingShape},
		"prior":    {Type: "array", Items: priorShape},
	}}
	// arbiterShape mirrors schema/arbiter.schema.json (R29).
	arbiterShape = &shape{Type: "object", Required: []string{"nonce", "checked", "verdict", "rationale"}, Props: map[string]*shape{
		"nonce":     nonEmpty,
		"checked":   checkedList,
		"verdict":   {Type: "string", Enum: []string{VerdictWithdraw, VerdictUphold}},
		"rationale": nonEmpty,
	}}
)

// clampLines keeps scope lines inside [1, n]; it reports whether any was
// beyond the snapshot.
func clampLines(lines []int, n int) bool {
	beyond := false
	for i := range lines {
		if lines[i] > n {
			lines[i], beyond = max(1, n), true
		}
	}
	return beyond
}
