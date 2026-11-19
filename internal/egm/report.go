package egm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Report is report.json (R33b). Ordering is stable and there are no
// timestamps, so the deterministic engine is byte-for-byte reproducible.
type Report struct {
	Slug           string          `json:"slug"`
	Doc            string          `json:"doc"`
	DocType        string          `json:"doc_type"`
	Cycle          int             `json:"cycle"`
	Round          int             `json:"round"`
	Verdict        string          `json:"verdict"`
	LoopState      string          `json:"loop_state"`
	Content        string          `json:"content"`
	Target         string          `json:"target"`
	Coverage       Coverage        `json:"coverage"`
	Engine         string          `json:"engine"`
	EngineReason   string          `json:"engine_reason,omitempty"`
	Findings       []ReportFinding `json:"findings"`
	SkippedSources []SkippedSource `json:"skipped_sources"`
	Exemptions     []Exemption     `json:"exemptions"`
	Warnings       []Warning       `json:"warnings"`
	Downgrades     []string        `json:"downgrades"`
	Budget         Budget          `json:"budget"`
}

// Coverage lists the required set and each reviewer's status.
type Coverage struct {
	Required  []string      `json:"required"`
	Reviewers []ReviewerRun `json:"reviewers"`
	Partial   bool          `json:"partial"`
}

// Budget is the cycle's agent-run budget (R27).
type Budget struct {
	MaxAgentRuns int `json:"max_agent_runs"`
	Used         int `json:"used"`
	Remaining    int `json:"remaining"`
}

// ReportFinding is one lineage in the report.
type ReportFinding struct {
	ID          string     `json:"id"`
	Severity    string     `json:"severity"`
	Status      string     `json:"status"`
	Confirmed   bool       `json:"confirmed"`
	Blocking    bool       `json:"blocking"`
	Owner       string     `json:"owner"`
	Reviewers   []string   `json:"reviewers"`
	Scope       Scope      `json:"scope"`
	Message     string     `json:"message"`
	Rationale   string     `json:"rationale,omitempty"`
	Evidence    []Evidence `json:"evidence,omitempty"`
	Cited       []string   `json:"cited,omitempty"`
	Context     string     `json:"context,omitempty"`
	MatchSHA    string     `json:"match_sha,omitempty"`
	Downgrade   string     `json:"downgrade,omitempty"`
	Unverified  bool       `json:"unverified,omitempty"`
	Occurrences int        `json:"occurrences"`
	OriginRound int        `json:"origin_round"`
	OpenedRound int        `json:"opened_round"`
}

func reportFindings(st *State) []ReportFinding {
	out := []ReportFinding{}
	if st == nil {
		return out
	}
	for _, l := range st.Lineages {
		out = append(out, ReportFinding{
			ID: l.ID, Severity: l.Severity, Status: l.Status, Confirmed: l.Confirmed, Blocking: l.blocking(),
			Owner: l.Owner, Reviewers: l.Reviewers, Scope: l.Scope, Message: l.Message, Rationale: l.Rationale,
			Evidence: l.Evidence, Cited: l.Cited, Context: l.Context, MatchSHA: l.MatchSHA, Downgrade: l.Downgrade,
			Unverified: l.Unverified, Occurrences: len(l.Occurrences), OriginRound: l.OriginRound, OpenedRound: l.OpenedRound,
		})
	}
	return out
}

func sortWarnings(ws []Warning) []Warning {
	out := []Warning{}
	seen := map[Warning]bool{}
	for _, w := range ws {
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func coverageOf(rd *Round) Coverage {
	c := Coverage{Required: []string{}, Reviewers: []ReviewerRun{}}
	if rd == nil {
		return c
	}
	c.Required, c.Reviewers = rd.Required, rd.Reviewers
	for _, rv := range rd.Reviewers {
		if rv.Status != "completed" && !(rv.Status == "skipped" && structuralSkip(rv.Reason)) {
			c.Partial = true
		}
	}
	return c
}

// JSON renders the report with a trailing newline.
func (r *Report) JSON() []byte {
	b, _ := json.MarshalIndent(r, "", "  ")
	return append(b, '\n')
}

// Markdown renders report.md from the report.
func (r *Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# tb egm: %s\n\n", r.Doc)
	fmt.Fprintf(&b, "- Verdict: **%s** (loop %s)\n- Cycle %d, round %d, engine %s\n", r.Verdict, r.LoopState, r.Cycle, r.Round, r.Engine)
	if r.EngineReason != "" {
		fmt.Fprintf(&b, "- Engine reason: %s\n", r.EngineReason)
	}
	fmt.Fprintf(&b, "- Content: `%s` (%s)\n", short(r.Content), r.Target)
	b.WriteString("\n## Findings\n\n")
	if len(r.Findings) == 0 {
		b.WriteString("None.\n")
	}
	for i := range r.Findings {
		f := &r.Findings[i]
		mark := ""
		if f.Blocking {
			mark = " (blocking)"
		}
		fmt.Fprintf(&b, "- **%s** [%s, %s%s] %s `%s` %v: %s\n", f.ID, f.Severity, f.Status, mark, f.Owner, f.Scope.Heading, f.Scope.Lines, f.Message)
	}
	b.WriteString("\n## Coverage\n\n")
	for _, rv := range r.Coverage.Reviewers {
		fmt.Fprintf(&b, "- %s: %s %s\n", rv.Name, rv.Status, rv.Reason)
	}
	writeList(&b, "Downgrades", r.Downgrades)
	ws := make([]string, 0, len(r.Warnings))
	for _, w := range r.Warnings {
		ws = append(ws, strings.TrimSpace(w.Code+" "+w.Path+" "+w.Message))
	}
	writeList(&b, "Warnings", ws)
	return b.String()
}

func writeList(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## %s\n\n", title)
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", it)
	}
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
