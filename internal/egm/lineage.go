package egm

import (
	"fmt"
	"sort"
)

// reconcile folds a round's findings into the lineages (R19–R22). mapper maps
// a lineage's scope to the current snapshot. Only lineages whose owner ran in
// this round are re-evaluated; the others keep their status (fixed owner,
// R20). Checker lineages are re-evaluated by the re-run.
type reconciler struct {
	st       *State
	round    int
	mapper   func(l *Lineage) Scope
	ran      map[string]bool // owners that ran this round
	warnings []Warning
}

func (r *reconciler) transition(l *Lineage, to, by, reason string) {
	if l.Status == to {
		return
	}
	r.st.Transitions = append(r.st.Transitions, Transition{
		Command: "round", Lineage: l.ID, From: l.Status, To: to, Round: r.round, Reason: reason, By: by,
	})
	l.Status = to
	l.RefRound = r.round
	if to == StOpen || to == StReopened {
		l.OpenedRound = r.round
	}
}

// fold applies the round's findings. Findings get their occurrence IDs here.
func (r *reconciler) fold(findings []Finding) {
	for _, l := range r.st.Lineages {
		l.Scope = r.mapper(l)
		l.ScopeRound = r.round
	}
	byKey := map[string]*Finding{}
	counter := map[string]int{}
	for i := range findings {
		f := &findings[i]
		counter[f.Reviewer]++
		f.Occurrence = fmt.Sprintf("%s-r%d-%d", f.Reviewer, r.round, counter[f.Reviewer])
		if f.Key != "" {
			byKey[f.Reviewer+"\x00"+f.Key] = f
		}
	}
	matched := map[*Finding]bool{}
	for _, l := range r.st.Lineages {
		if !isChecker(l.Owner) || !r.ran[l.Owner] {
			continue
		}
		f := byKey[l.Owner+"\x00"+l.Key]
		if f == nil {
			r.notReproduced(l)
			continue
		}
		if r.reproduced(l, f) {
			matched[f] = true
		}
	}
	for i := range findings {
		if f := &findings[i]; !matched[f] {
			r.newLineage(f)
		}
	}
}

func (r *reconciler) notReproduced(l *Lineage) {
	switch l.Status {
	case StOpen, StReopened:
		r.transition(l, StFixed, "rerun", "not reproduced by the checker")
		l.Confirmed = true
	case StFixed:
		if !l.Confirmed {
			l.Confirmed = true
			r.st.Transitions = append(r.st.Transitions, Transition{Command: "round", Lineage: l.ID, From: StFixed, To: StFixed, Round: r.round, Reason: "fix confirmed by the re-run", By: "rerun"})
		}
		if l.Downgrade != "" && l.Confirmed {
			l.Downgrade = "" // --fix from settled/declined, then confirmed (R24)
		}
	}
}

// reproduced records the occurrence on l, or returns false when the finding
// must open a new lineage (higher severity over a closed status, R20/R22).
func (r *reconciler) reproduced(l *Lineage, f *Finding) bool {
	higher := sevRank(f.Severity) > sevRank(l.Severity)
	switch l.Status {
	case StSettled, StDeclined, StContestPending, StEscalated:
		if higher {
			return false
		}
		if f.Reviewer == injectionChecker && l.Status == StSettled && f.MatchSHA != l.MatchSHA {
			r.transition(l, StReopened, "rerun", "injection match changed")
		}
	case StFixed:
		r.transition(l, StReopened, "rerun", "reproduced by the checker")
		l.Confirmed = false
	}
	if higher {
		l.Severity = f.Severity
	}
	r.record(l, f)
	return true
}

func (r *reconciler) record(l *Lineage, f *Finding) {
	l.Occurrences = append(l.Occurrences, f.Occurrence)
	r.st.Occurrences[f.Occurrence] = l.ID
	l.Scope, l.ScopeRound = f.Scope, r.round
	l.Message, l.Rationale, l.Evidence = f.Message, f.Rationale, f.Evidence
	if f.MatchSHA != "" {
		l.MatchSHA, l.Context = f.MatchSHA, f.Context
	}
	if !containsStr(l.Reviewers, f.Reviewer) {
		l.Reviewers = append(l.Reviewers, f.Reviewer)
		sort.Strings(l.Reviewers)
	}
}

func (r *reconciler) newLineage(f *Finding) {
	l := &Lineage{
		ID: r.st.newLineageID(), Severity: f.Severity, Owner: f.Reviewer, Persona: personaOf(f.Reviewer),
		Key: f.Key, Status: StOpen, OriginRound: r.round, OpenedRound: r.round, RefRound: r.round,
		Unverified: f.Unverified,
	}
	l.Cited = citedOf(f)
	r.st.Lineages = append(r.st.Lineages, l)
	r.record(l, f)
}

func personaOf(reviewer string) string {
	if s, ok := presetSpec(reviewer); ok {
		return s.Persona
	}
	return ""
}

// citedOf returns the verified evidence paths of a finding (glossary).
func citedOf(f *Finding) []string {
	if f.Unverified {
		return nil
	}
	var out []string
	for _, e := range f.Evidence {
		if !containsStr(out, e.Path) {
			out = append(out, e.Path)
		}
	}
	sort.Strings(out)
	return out
}
