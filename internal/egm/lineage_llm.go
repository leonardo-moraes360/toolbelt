package egm

import (
	"sort"
)

// fixCheck answers R20's "a fix needs a change" for a lineage: whether a
// change since opened_round touched its mapped scope or the evidence ±5 lines
// of a cited source, and whether the content changed at all since then.
type fixCheck func(l *Lineage) (touched, contentChanged bool, err error)

// applyPrior applies the owners' still_holds answers (R20). answers maps the
// owning reviewer to its answers; only reviewers that completed appear.
func (r *reconciler) applyPrior(answers map[string][]PriorAnswer, check fixCheck) error {
	for owner, list := range answers {
		for _, a := range list {
			l := r.st.lineage(a.Lineage)
			if l == nil || l.Owner != owner {
				continue
			}
			if err := r.applyAnswer(l, a.StillHolds, check); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *reconciler) applyAnswer(l *Lineage, holds bool, check fixCheck) error {
	switch {
	case l.Status == StFixed && holds:
		r.transition(l, StReopened, "reviewer", "owner says it still holds")
		l.Confirmed = false
	case l.Status == StFixed:
		r.confirm(l, "reviewer")
	case (l.Status == StOpen || l.Status == StReopened) && !holds:
		return r.ownerFix(l, check)
	}
	return nil
}

func (r *reconciler) confirm(l *Lineage, by string) {
	if !l.Confirmed {
		r.st.Transitions = append(r.st.Transitions, Transition{Command: "round", Lineage: l.ID, From: StFixed, To: StFixed, Round: r.round, Reason: "fix confirmed", By: by})
	}
	l.Confirmed, l.ConfirmedRound = true, r.round
	if l.Downgrade != "" {
		l.Downgrade = "" // --fix from settled/declined, then the owner confirmed (R24)
	}
}

// ownerFix is R20's "a fix needs a change" for still_holds: false.
func (r *reconciler) ownerFix(l *Lineage, check fixCheck) error {
	touched, changed, err := check(l)
	if err != nil {
		return err
	}
	if !changed {
		r.warnings = append(r.warnings, Warning{Code: "fixed_without_edit", Message: l.ID + ": the owner says it no longer holds, but nothing changed; ignored"})
		return nil
	}
	if !touched {
		r.warnings = append(r.warnings, Warning{Code: "fixed_elsewhere", Message: l.ID + ": closed by its owner with no change on its lines"})
	}
	r.transition(l, StFixed, "reviewer", "owner says it no longer holds")
	r.confirm(l, "reviewer")
	return nil
}

// mergeLLM folds the reviewers' new findings into the lineages (R19, R20 new
// findings, R22). Checker lineages match by key only and are not candidates.
func (r *reconciler) mergeLLM(findings []*Finding, set *Settings) {
	var fresh []*Lineage
	for _, f := range findings {
		if l := r.candidate(f, set); l != nil {
			r.matched(l, f)
			continue
		}
		if l := firstMatch(fresh, f, set); l != nil {
			r.record(l, f)
			r.mergeOwner(l, f)
			continue
		}
		r.newLineage(f)
		fresh = append(fresh, r.st.Lineages[len(r.st.Lineages)-1])
	}
}

// candidate returns the first existing lineage, in F-<k> order, that the
// finding matches. Another owner's lineage with a lower severity than the
// finding is not a candidate, so a blocker is never absorbed by another
// owner's major (R19).
func (r *reconciler) candidate(f *Finding, set *Settings) *Lineage {
	for _, l := range r.st.Lineages {
		if isChecker(l.Owner) || l.OriginRound == r.round {
			continue
		}
		if l.Owner != f.Reviewer && sevRank(l.Severity) < sevRank(f.Severity) {
			continue
		}
		if matches(l, f, set) {
			return l
		}
	}
	return nil
}

func firstMatch(list []*Lineage, f *Finding, set *Settings) *Lineage {
	for _, l := range list {
		if l.Owner != f.Reviewer && sevRank(l.Severity) < sevRank(f.Severity) {
			continue
		}
		if matches(l, f, set) {
			return l
		}
	}
	return nil
}

func matches(l *Lineage, f *Finding, set *Settings) bool {
	return Sim(l.Message, f.Message, set.Match.Stem) >= set.Match.Sim && Region(l.Scope, f.Scope, set.Match.Gap)
}

// matched applies R20's rules for a new finding that matched lineage l.
func (r *reconciler) matched(l *Lineage, f *Finding) {
	higher := sevRank(f.Severity) > sevRank(l.Severity)
	switch l.Status {
	case StFixed:
		r.transition(l, StReopened, "reviewer", "a new finding matches it")
		l.Confirmed = false
	case StSettled, StDeclined, StContestPending, StEscalated:
		if higher {
			r.newLineage(f)
			return
		}
	}
	if higher {
		l.Severity = f.Severity
	}
	r.record(l, f)
	l.Cited = unionSorted(l.Cited, citedOf(f))
}

// mergeOwner keeps the owner of the highest-severity occurrence, with ties
// broken by code sources, then R10 order (R19).
func (r *reconciler) mergeOwner(l *Lineage, f *Finding) {
	l.Cited = unionSorted(l.Cited, citedOf(f))
	switch {
	case sevRank(f.Severity) > sevRank(l.Severity):
		l.Severity, l.Owner, l.Persona = f.Severity, f.Reviewer, personaOf(f.Reviewer)
	case sevRank(f.Severity) == sevRank(l.Severity) && ownerRank(f.Reviewer) < ownerRank(l.Owner):
		l.Owner, l.Persona = f.Reviewer, personaOf(f.Reviewer)
	}
}

// ownerRank orders owners for the R19 tie-break: code-source reviewers,
// then R10 order, then checkers.
func ownerRank(name string) int {
	for i, rv := range Preset {
		if rv.Name == name {
			if rv.CodeSources {
				return i
			}
			return 100 + i
		}
	}
	return 1000
}

func unionSorted(a, b []string) []string {
	out := append([]string(nil), a...)
	for _, x := range b {
		if !containsStr(out, x) {
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
