package egm

import "sort"

// Verdicts (R25).
const (
	VerdictNoState     = "NO_STATE"
	VerdictPass        = "PASS"
	VerdictPassPartial = "PASS_PARTIAL"
	VerdictFail        = "FAIL"
	VerdictStale       = "STALE"
	VerdictInProgress  = "IN_PROGRESS"
)

// roundClean is R23: no blocking lineage, no required reviewer not_run, and
// no unstaged source (R7).
func roundClean(st *State, rd *Round, unstaged bool) bool {
	if unstaged {
		return false
	}
	for _, l := range st.Lineages {
		if l.blocking() {
			return false
		}
	}
	for _, rv := range rd.Reviewers {
		if rv.Status == "not_run" && containsStr(rd.Required, rv.Name) {
			return false
		}
	}
	return true
}

// roundDowngrades are the cycle downgrades a round contributes (R24).
func roundDowngrades(rd *Round, docType string, hasDetailed, maxExcluded bool) []string {
	var out []string
	for _, rv := range rd.Reviewers {
		if rv.Status == "completed" || (rv.Status == "skipped" && structuralSkip(rv.Reason)) {
			continue // a spec without DetailedRange is counted once below
		}
		out = append(out, "reviewer_"+rv.Status+":"+rv.Name)
	}
	if docType == "spec" && !hasDetailed {
		out = append(out, "no_detailed_section")
	}
	if maxExcluded {
		out = append(out, "max_exclusion")
	}
	sort.Strings(out)
	return out
}

// codeReviewerRan reports whether a reviewer with code sources completed, the
// case in which a max_* exclusion reduces coverage (R7 effects, R24).
func codeReviewerRan(rd *Round) bool {
	for _, rv := range rd.Reviewers {
		if spec, ok := presetSpec(rv.Name); ok && spec.CodeSources && rv.Status == "completed" {
			return true
		}
	}
	return false
}

// advanceCycle updates convergence and the cap after round rd (R23). It
// returns true when the round produced a convergence record.
func advanceCycle(st *State, set *Settings) bool {
	cy, rd := st.cycle(), st.lastRound()
	rounds := st.cycleRounds(cy.N)
	count := 0
	for i := len(rounds) - 1; i >= 0 && rounds[i].Clean && rounds[i].Content == rd.Content && rounds[i].PolicySHA == rd.PolicySHA; i-- {
		count++
	}
	if count >= set.CleanRounds {
		counted := rounds[len(rounds)-count:]
		rec := Convergence{Content: rd.Content, PolicySHA: rd.PolicySHA, Cycle: cy.N}
		seen := map[string]bool{}
		if cy.Restart {
			seen["restart"] = true
		}
		for _, r := range counted {
			rec.Rounds = append(rec.Rounds, r.N)
			for _, d := range r.Downgrades {
				seen[d] = true
			}
		}
		for d := range seen {
			rec.Downgrades = append(rec.Downgrades, d)
		}
		sort.Strings(rec.Downgrades)
		st.Convergences = append(st.Convergences, rec)
		cy.Converged = true
		return true
	}
	if len(rounds) >= set.MaxRounds {
		cy.Capped = true
	}
	return false
}

// lineageDowngrades lists the lineages that carry a lineage downgrade (R24).
func lineageDowngrades(st *State) []string {
	var out []string
	for _, l := range st.Lineages {
		if l.Downgrade != "" {
			out = append(out, l.ID+":"+l.Downgrade)
		}
	}
	return out
}

func anyBlocking(st *State) bool {
	for _, l := range st.Lineages {
		if l.blocking() {
			return true
		}
	}
	return false
}

// LoopState is running, capped or converged (R25).
func LoopState(st *State) string {
	cy := st.cycle()
	switch {
	case cy == nil:
		return "running"
	case cy.Converged:
		return "converged"
	case cy.Capped:
		return "capped"
	}
	return "running"
}

// Verdict evaluates R25 over the target content and the current policy.
// downgrades lists the record's and lineages' downgrades when the verdict is
// PASS or PASS_PARTIAL.
func Verdict(st *State, content, policy string) (verdict string, downgrades []string) {
	if st == nil {
		return VerdictNoState, nil
	}
	ld := lineageDowngrades(st)
	blocking := anyBlocking(st)
	var rec *Convergence
	other := false
	for i := len(st.Convergences) - 1; i >= 0; i-- {
		c := &st.Convergences[i]
		if c.Content == content && c.PolicySHA == policy && rec == nil {
			rec = c
		} else {
			other = true
		}
	}
	if rec != nil && !blocking {
		d := append(append([]string(nil), rec.Downgrades...), ld...)
		if len(d) == 0 {
			return VerdictPass, nil
		}
		return VerdictPassPartial, d
	}
	if cy := st.cycle(); cy != nil && cy.Capped {
		return VerdictFail, nil
	}
	if rec != nil || other {
		return VerdictStale, nil
	}
	return VerdictInProgress, nil
}
