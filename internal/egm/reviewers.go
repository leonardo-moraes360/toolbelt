package egm

import (
	"encoding/json"
	"sort"
)

// Reviewer ranges (R10).
const (
	RangeWhole    = "whole"
	RangeFeature  = "feature"
	RangeDetailed = "detailed"
)

// ReviewerSpec is one slot of the consolidated preset (R10).
type ReviewerSpec struct {
	Name, Persona, Standard, Range string
	CodeSources                    bool
}

// Preset is the consolidated preset, in R10 order (also the owner tie-break
// order, R19). Standard "structural" means the type's structural standard.
var Preset = []ReviewerSpec{
	{"zoom-out", "zoom-out", "zoom-out", RangeWhole, false},
	{"inventive-catalyst", "inventive-catalyst", "", RangeWhole, false},
	{"comprehension", "comprehension", "", RangeWhole, false},
	{"design", "design", "structural", RangeFeature, false},
	{"structural", "structural", "structural", RangeWhole, false},
	{"style-feature", "style", "", RangeFeature, false},
	{"implementation-readiness", "implementation-readiness", "implementation-readiness", RangeDetailed, true},
	{"codebase-details", "codebase-details", "implementation-readiness", RangeWhole, true},
	{"style-detailed", "style", "", RangeDetailed, false},
}

func presetSpec(name string) (ReviewerSpec, bool) {
	for _, r := range Preset {
		if r.Name == name {
			return r, true
		}
	}
	return ReviewerSpec{}, false
}

// DocType maps the document through egm.doc_types (default spec, R6).
func DocType(rel string, types map[string]string) string {
	globs := make([]string, 0, len(types))
	for g := range types {
		globs = append(globs, g)
	}
	sort.Strings(globs) // deterministic when several globs match
	for _, g := range globs {
		if t := types[g]; MatchGlob(g, rel) && (t == "adr" || t == "spec") {
			return t
		}
	}
	return "spec"
}

// RequiredReviewers is the full required set (R6): all 9 with a
// DetailedRange; otherwise the 6 not on the DetailedRange.
func RequiredReviewers(hasDetailed bool) []string {
	var out []string
	for _, r := range Preset {
		if hasDetailed || r.Range != RangeDetailed {
			out = append(out, r.Name)
		}
	}
	return out
}

// structuralSkip is a skipped reason that is not a coverage reduction (R25).
func structuralSkip(reason string) bool {
	return reason == "no_detailed_section" || reason == "range_too_small"
}

// planReviewers builds the round's reviewer statuses for the deterministic
// engine: LLM personas are unavailable; the detail reviewers of a spec
// without DetailedRange are skipped; --reviewers narrows the list.
func planReviewers(docType string, hasDetailed bool, only []string) []ReviewerRun {
	var out []ReviewerRun
	for _, r := range Preset {
		switch {
		case r.Range == RangeDetailed && !hasDetailed && docType == "adr":
			continue // not required (R6)
		case r.Range == RangeDetailed && !hasDetailed:
			out = append(out, ReviewerRun{Name: r.Name, Status: "skipped", Reason: "no_detailed_section"})
		case len(only) > 0 && !containsStr(only, r.Name):
			out = append(out, ReviewerRun{Name: r.Name, Status: "skipped", Reason: "not_selected"})
		default:
			out = append(out, ReviewerRun{Name: r.Name, Status: "unavailable", Reason: "deterministic_engine"})
		}
	}
	return out
}

// PolicySHA is the sha256 of the canonical JSON of the policy (glossary).
func PolicySHA(set *Settings, hasDetailed bool) string {
	p := struct {
		Required       []string          `json:"required"`
		CleanRounds    int               `json:"clean_rounds"`
		Match          any               `json:"match"`
		Attest         any               `json:"attest"`
		DetailHeadings []string          `json:"detail_headings"`
		DocTypes       map[string]string `json:"doc_types"`
		RefsIgnore     []string          `json:"refs_ignore"`
		MaxSourceBytes int64             `json:"max_source_bytes"`
		MaxPacketBytes int64             `json:"max_packet_bytes"`
	}{
		RequiredReviewers(hasDetailed), set.CleanRounds, set.Match, set.Attest, set.DetailHeadings,
		set.DocTypes, set.RefsIgnore, set.MaxSourceBytes, set.MaxPacketBytes,
	}
	b, _ := json.Marshal(p) // maps marshal with sorted keys: canonical
	return sha256Hex(b)
}
