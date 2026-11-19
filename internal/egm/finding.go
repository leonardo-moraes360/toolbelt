package egm

// Severities (R15).
const (
	SevBlocker = "blocker"
	SevMajor   = "major"
	SevNit     = "nit"
)

func sevRank(s string) int {
	switch s {
	case SevBlocker:
		return 3
	case SevMajor:
		return 2
	case SevNit:
		return 1
	}
	return 0
}

// Lineage statuses (R21).
const (
	StOpen           = "open"
	StFixed          = "fixed"
	StReopened       = "reopened"
	StContestPending = "contest_pending"
	StSettled        = "settled"
	StEscalated      = "escalated"
	StDeclined       = "declined"
)

// Scope locates a finding: heading (whole line, trimmed) and 1-based
// inclusive lines.
type Scope struct {
	Heading string `json:"heading"`
	Lines   []int  `json:"lines,omitempty"`
}

// Evidence points at a line of the document or of a source.
type Evidence struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

// Finding is one occurrence produced by a reviewer or a checker in a round
// (egm.Finding, G2).
type Finding struct {
	Occurrence string     `json:"occurrence"`
	Reviewer   string     `json:"reviewer"`
	Severity   string     `json:"severity"`
	Scope      Scope      `json:"scope"`
	Message    string     `json:"message"`
	Rationale  string     `json:"rationale,omitempty"`
	Evidence   []Evidence `json:"evidence,omitempty"`
	// Key is the stable identity of a checker finding: a checker lineage is
	// reproduced when the re-run emits the same key (R20).
	Key        string `json:"key,omitempty"`
	Context    string `json:"context,omitempty"`   // injection-scan (R17a)
	MatchSHA   string `json:"match_sha,omitempty"` // injection-scan (R17a)
	Unverified bool   `json:"unverified,omitempty"`
	OutOfRange bool   `json:"out_of_range,omitempty"`
}

// blocking reports whether a lineage blocks convergence and PASS (glossary).
func (l *Lineage) blocking() bool {
	if sevRank(l.Severity) < sevRank(SevMajor) {
		return false
	}
	switch l.Status {
	case StOpen, StReopened, StContestPending, StEscalated:
		return true
	case StFixed:
		return !l.Confirmed
	}
	return false
}
