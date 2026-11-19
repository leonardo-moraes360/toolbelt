package egm

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

// validateDoc: headings at lines 1, 3 and 5.
const validateDoc = `# Title
The introduction explains what this document is about.
## Alpha
Alpha has a body line that is long enough to be quoted.
## Beta
Beta also carries a body line long enough to quote.
`

func validateInput() *ValidateInput {
	return &ValidateInput{
		Reviewer: "design", Nonce: "n0nce", MaxSeverity: SevBlocker,
		PriorIDs: []string{"F-1"},
		Doc:      ParseDoc(validateDoc, nil), DocPath: "docs/a.md",
		Range: [2]int{1, 6}, MinQuotes: 3,
		Sources: map[string]*Doc{"internal/x.go": ParseDoc("package x\n\nfunc X() {}\n", nil)},
	}
}

func validPayload() map[string]any {
	return map[string]any{
		"reviewer": "design",
		"nonce":    "n0nce",
		"checked": []any{
			map[string]any{"heading": "# Title", "quote": "The introduction explains what"},
			map[string]any{"heading": "## Alpha", "quote": "## Alpha"},
			map[string]any{"heading": "## Beta", "quote": "Beta also carries a body line"},
		},
		"findings": []any{map[string]any{
			"severity":  "major",
			"scope":     map[string]any{"heading": "## Alpha", "lines": []any{3, 4}},
			"message":   "Alpha is vague.",
			"rationale": "Readers cannot tell what it does.",
			"evidence":  []any{map[string]any{"path": "internal/x.go", "line": 3}, map[string]any{"path": "docs/a.md", "line": 4}},
		}},
		"prior": []any{map[string]any{"lineage": "F-1", "still_holds": true}},
	}
}

func encode(t *testing.T, m map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func finding0(m map[string]any) map[string]any {
	return m["findings"].([]any)[0].(map[string]any)
}

func TestValidateValid(t *testing.T) {
	out, reason := ParseReviewerOutput(encode(t, validPayload()), validateInput())
	if reason != "" {
		t.Fatalf("reason %q", reason)
	}
	f := out.Findings[0]
	if f.Reviewer != "design" || f.Unverified || f.OutOfRange || f.Severity != SevMajor || len(f.Evidence) != 2 {
		t.Fatalf("finding %+v", f)
	}
	if len(out.Prior) != 1 || !out.Prior[0].StillHolds || len(out.Checked) != 3 {
		t.Fatalf("output %+v", out)
	}
}

func TestValidateFence(t *testing.T) {
	body := string(encode(t, validPayload()))
	for _, p := range []string{"```json\n" + body + "\n```", "  ```JSON\n" + body + "\n```\n", "```\n" + body + "```"} {
		if _, reason := ParseReviewerOutput([]byte(p), validateInput()); reason != "" {
			t.Errorf("%.12q: %s", p, reason)
		}
	}
	for _, p := range []string{"```yaml\n" + body + "\n```", "```json\n" + body, "Here it is:\n" + body, body + "\nDone.", body + body} {
		if _, reason := ParseReviewerOutput([]byte(p), validateInput()); reason != ReasonInvalidSchema {
			t.Errorf("%.20q: got %q", p, reason)
		}
	}
}

func TestValidateSchemaErrors(t *testing.T) {
	cases := map[string]func(m map[string]any){
		"extra top-level key": func(m map[string]any) { m["notes"] = "x" },
		"missing prior":       func(m map[string]any) { delete(m, "prior") },
		"missing checked":     func(m map[string]any) { delete(m, "checked") },
		"reviewer mismatch":   func(m map[string]any) { m["reviewer"] = "style" },
		"bad severity":        func(m map[string]any) { finding0(m)["severity"] = "critical" },
		"extra finding key":   func(m map[string]any) { finding0(m)["same_as"] = "F-1" },
		"empty message":       func(m map[string]any) { finding0(m)["message"] = "" },
		"one line": func(m map[string]any) {
			finding0(m)["scope"] = map[string]any{"heading": "## Alpha", "lines": []any{3}}
		},
		"reversed lines": func(m map[string]any) {
			finding0(m)["scope"] = map[string]any{"heading": "## Alpha", "lines": []any{4, 3}}
		},
		"zero line": func(m map[string]any) {
			finding0(m)["scope"] = map[string]any{"heading": "## Alpha", "lines": []any{0, 3}}
		},
		"float line": func(m map[string]any) {
			finding0(m)["scope"] = map[string]any{"heading": "## Alpha", "lines": []any{1.5, 3}}
		},
		"string still_holds": func(m map[string]any) { m["prior"] = []any{map[string]any{"lineage": "F-1", "still_holds": "yes"}} },
		"findings not array": func(m map[string]any) { m["findings"] = map[string]any{} },
		"evidence no line":   func(m map[string]any) { finding0(m)["evidence"] = []any{map[string]any{"path": "docs/a.md"}} },
	}
	for name, mut := range cases {
		m := validPayload()
		mut(m)
		if _, reason := ParseReviewerOutput(encode(t, m), validateInput()); reason != ReasonInvalidSchema {
			t.Errorf("%s: got %q", name, reason)
		}
	}
	if _, reason := ParseReviewerOutput([]byte(`[]`), validateInput()); reason != ReasonInvalidSchema {
		t.Errorf("array payload: %q", reason)
	}
}

func TestValidateReasons(t *testing.T) {
	cases := []struct {
		name   string
		mut    func(m map[string]any, in *ValidateInput)
		reason string
	}{
		{"wrong nonce", func(m map[string]any, _ *ValidateInput) { m["nonce"] = "other" }, ReasonWrongNonce},
		{"over cap major", func(m map[string]any, in *ValidateInput) {
			in.MaxSeverity = SevMajor
			finding0(m)["severity"] = SevBlocker
		}, ReasonOverSeverity},
		{"cap none", func(_ map[string]any, in *ValidateInput) { in.MaxSeverity = "none" }, ReasonOverSeverity},
		{"prior missing", func(m map[string]any, _ *ValidateInput) { m["prior"] = []any{} }, ReasonIncompletePrior},
		{"prior unknown extra", func(m map[string]any, _ *ValidateInput) {
			m["prior"] = append(m["prior"].([]any), map[string]any{"lineage": "F-9", "still_holds": false})
		}, ReasonIncompletePrior},
		{"prior duplicate", func(m map[string]any, in *ValidateInput) {
			in.PriorIDs = []string{"F-1", "F-2"}
			m["prior"] = append(m["prior"].([]any), map[string]any{"lineage": "F-1", "still_holds": false})
		}, ReasonIncompletePrior},
		{"no attestation", func(m map[string]any, _ *ValidateInput) { m["checked"] = []any{} }, ReasonBadAttestation},
		{"fake quote", func(m map[string]any, _ *ValidateInput) {
			m["checked"].([]any)[0] = map[string]any{"heading": "# Title", "quote": "this sentence is not in the doc"}
		}, ReasonBadAttestation},
	}
	for _, c := range cases {
		m, in := validPayload(), validateInput()
		c.mut(m, in)
		if _, reason := ParseReviewerOutput(encode(t, m), in); reason != c.reason {
			t.Errorf("%s: got %q, want %q", c.name, reason, c.reason)
		}
	}
	// Nit under a major cap and no prior when none is owned are valid.
	m, in := validPayload(), validateInput()
	in.MaxSeverity, in.PriorIDs = SevMajor, nil
	finding0(m)["severity"] = SevNit
	m["prior"] = []any{}
	if _, reason := ParseReviewerOutput(encode(t, m), in); reason != "" {
		t.Errorf("valid variant: %q", reason)
	}
}

func TestValidateUnverifiedAndOutOfRange(t *testing.T) {
	cases := []struct {
		name                   string
		mut                    func(f map[string]any, in *ValidateInput)
		unverified, outOfRange bool
	}{
		{"unknown path", func(f map[string]any, _ *ValidateInput) {
			f["evidence"] = []any{map[string]any{"path": "internal/y.go", "line": 1}}
		}, true, false},
		{"line past end", func(f map[string]any, _ *ValidateInput) {
			f["evidence"] = []any{map[string]any{"path": "internal/x.go", "line": 4}}
		}, true, false},
		{"doc line past end", func(f map[string]any, _ *ValidateInput) {
			f["evidence"] = []any{map[string]any{"path": "docs/a.md", "line": 7}}
		}, true, false},
		{"missing heading", func(f map[string]any, _ *ValidateInput) {
			f["scope"] = map[string]any{"heading": "## Gamma", "lines": []any{3, 4}}
		}, true, false},
		{"empty heading inside a section", func(f map[string]any, _ *ValidateInput) {
			f["scope"] = map[string]any{"heading": "", "lines": []any{4, 4}}
		}, true, false},
		{"no evidence", func(f map[string]any, _ *ValidateInput) { delete(f, "evidence") }, false, false},
		{"scope before range", func(_ map[string]any, in *ValidateInput) { in.Range = [2]int{5, 6} }, false, true},
		{"scope after range", func(_ map[string]any, in *ValidateInput) { in.Range = [2]int{1, 3} }, false, true},
	}
	for _, c := range cases {
		m, in := validPayload(), validateInput()
		c.mut(finding0(m), in)
		switch in.Range {
		case [2]int{5, 6}:
			m["checked"] = []any{map[string]any{"heading": "## Beta", "quote": "Beta also carries a body line"}}
		case [2]int{1, 3}:
			m["checked"] = []any{map[string]any{"heading": "# Title", "quote": "The introduction explains what"}}
		}
		out, reason := ParseReviewerOutput(encode(t, m), in)
		if reason != "" {
			t.Fatalf("%s: %q", c.name, reason)
		}
		f := out.Findings[0]
		if f.Unverified != c.unverified || f.OutOfRange != c.outOfRange {
			t.Errorf("%s: unverified %v out_of_range %v", c.name, f.Unverified, f.OutOfRange)
		}
	}
}

func arbiterPayload() map[string]any {
	return map[string]any{
		"nonce": "n0nce",
		"checked": []any{
			map[string]any{"heading": "## Alpha", "quote": "## Alpha"},
			map[string]any{"heading": "## Alpha", "quote": "Alpha has a body line that is long"},
		},
		"verdict":   "withdraw",
		"rationale": "Line 4 already defines it.",
	}
}

func TestValidateArbiter(t *testing.T) {
	d, rng := ParseDoc(validateDoc, nil), [2]int{3, 4}
	out, reason := ParseArbiterOutput(encode(t, arbiterPayload()), "n0nce", d, rng, 3)
	if reason != "" || out.Verdict != VerdictWithdraw {
		t.Fatalf("valid arbiter: %q %+v", reason, out)
	}
	cases := map[string]struct {
		mut    func(m map[string]any)
		reason string
	}{
		"bad verdict":     {func(m map[string]any) { m["verdict"] = "approve" }, ReasonInvalidSchema},
		"no rationale":    {func(m map[string]any) { delete(m, "rationale") }, ReasonInvalidSchema},
		"empty rationale": {func(m map[string]any) { m["rationale"] = "" }, ReasonInvalidSchema},
		"extra key":       {func(m map[string]any) { m["findings"] = []any{} }, ReasonInvalidSchema},
		"wrong nonce":     {func(m map[string]any) { m["nonce"] = "x" }, ReasonWrongNonce},
		"headings only":   {func(m map[string]any) { m["checked"] = m["checked"].([]any)[:1] }, ReasonBadAttestation},
	}
	for name, c := range cases {
		m := arbiterPayload()
		c.mut(m)
		if _, reason := ParseArbiterOutput(encode(t, m), "n0nce", d, rng, 3); reason != c.reason {
			t.Errorf("%s: got %q, want %q", name, reason, c.reason)
		}
	}
	fenced := "```json\n" + string(encode(t, arbiterPayload())) + "\n```"
	if _, reason := ParseArbiterOutput([]byte(fenced), "n0nce", d, rng, 3); reason != "" {
		t.Errorf("fenced arbiter: %q", reason)
	}
}

// TestSchemaInSync keeps the published schemas and the hand-written
// validator identical: same types, required keys, properties and bounds.
func TestSchemaInSync(t *testing.T) {
	for file, s := range map[string]*shape{
		"schema/finding.schema.json": reviewerShape,
		"schema/arbiter.schema.json": arbiterShape,
	} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var node map[string]any
		if err := json.Unmarshal(b, &node); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		compareShape(t, file, node, s)
	}
}

func compareShape(t *testing.T, at string, node map[string]any, s *shape) {
	t.Helper()
	if node["type"] != s.Type {
		t.Errorf("%s: type %v, validator %s", at, node["type"], s.Type)
		return
	}
	switch s.Type {
	case "object":
		compareObject(t, at, node, s)
	case "array":
		compareNum(t, at+".minItems", node["minItems"], s.MinItems)
		compareNum(t, at+".maxItems", node["maxItems"], s.MaxItems)
		items, _ := node["items"].(map[string]any)
		compareShape(t, at+"[]", items, s.Items)
	case "string":
		compareNum(t, at+".minLength", node["minLength"], s.MinLength)
		var enum []string
		for _, e := range asSlice(node["enum"]) {
			enum = append(enum, e.(string))
		}
		if !sameSet(enum, s.Enum) {
			t.Errorf("%s: enum %v, validator %v", at, enum, s.Enum)
		}
	case "integer":
		lo := 0
		if s.Minimum != nil {
			lo = *s.Minimum
		}
		compareNum(t, at+".minimum", node["minimum"], lo)
	}
}

func compareObject(t *testing.T, at string, node map[string]any, s *shape) {
	t.Helper()
	if node["additionalProperties"] != false {
		t.Errorf("%s: additionalProperties must be false", at)
	}
	var req []string
	for _, r := range asSlice(node["required"]) {
		req = append(req, r.(string))
	}
	if !sameSet(req, s.Required) {
		t.Errorf("%s: required %v, validator %v", at, req, s.Required)
	}
	props, _ := node["properties"].(map[string]any)
	if len(props) != len(s.Props) {
		t.Errorf("%s: %d properties, validator %d", at, len(props), len(s.Props))
	}
	for k, p := range s.Props {
		pn, ok := props[k].(map[string]any)
		if !ok {
			t.Errorf("%s.%s: missing in schema", at, k)
			continue
		}
		compareShape(t, at+"."+k, pn, p)
	}
}

func compareNum(t *testing.T, at string, v any, want int) {
	t.Helper()
	got := 0
	if f, ok := v.(float64); ok {
		got = int(f)
	}
	if got != want {
		t.Errorf("%s: schema %d, validator %d", at, got, want)
	}
}

func asSlice(v any) []any {
	a, _ := v.([]any)
	return a
}

func sameSet(a, b []string) bool {
	a, b = append([]string{}, a...), append([]string{}, b...)
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
