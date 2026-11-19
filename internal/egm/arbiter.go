package egm

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

const arbiterWindow = 20

// moveLineage records a status change of l (R21).
func (s *State) moveLineage(l *Lineage, cmd, to, by, reason string, round int) {
	s.Transitions = append(s.Transitions, Transition{Command: cmd, Lineage: l.ID, From: l.Status, To: to, Round: round, Reason: reason, By: by})
	l.Status, l.RefRound = to, round
	if to == StOpen || to == StReopened {
		l.OpenedRound = round
	}
}

// arbiters runs the arbiter of each contest_pending lineage before the round
// (R26, R29). It reports whether any arbiter ran.
func (rc *roundCtx) arbiters(ctx context.Context) (bool, error) {
	pending := false
	for _, l := range rc.st.Lineages {
		pending = pending || (l.Status == StContestPending && l.ArbiterNotRun < 2)
	}
	if !pending {
		return false, nil
	}
	mapper, err := rc.s.mapper(ctx, rc.st)
	if err != nil {
		return false, err
	}
	ran := false
	for _, l := range rc.st.Lineages {
		if l.Status != StContestPending || l.ArbiterNotRun >= 2 {
			continue
		}
		did, err := rc.arbiter(ctx, l, mapper(l))
		if err != nil {
			return ran, err
		}
		ran = ran || did
	}
	return ran, nil
}

type arbiterJob struct {
	owner, packet, prompt, nonce string
	rng                          [2]int
}

// arbiterEvidence is the contest evidence in the current snapshot: its file
// text and its mapped line.
func (rc *roundCtx) arbiterEvidence(l *Lineage, lm *LineMap) (path, text string, line int, ok bool) {
	e := l.ContestEvidence
	if e == nil || e.Path == rc.s.rel {
		ln := 1
		if e != nil {
			ln = lm.Map(e.Line)
		} else if len(l.Scope.Lines) == 2 {
			ln = l.Scope.Lines[0]
		}
		return rc.s.rel, rc.s.text, ln, true
	}
	for _, src := range rc.s.sources.Included {
		if src.Path == e.Path {
			return src.Path, src.Text, e.Line, true
		}
	}
	return "", "", 0, false
}

func (rc *roundCtx) arbiter(ctx context.Context, l *Lineage, lm *LineMap) (bool, error) {
	round := len(rc.st.Rounds)
	scope := lm.MapScope(l.Scope)
	path, text, line, ok := rc.arbiterEvidence(l, lm)
	// R17a holds for the text the arbiter will see now, not only for the
	// text at contest time: an edit after --contest may add a directive.
	guardErr := error(nil)
	if ok {
		guardErr = injectionGuard(rc.s, rc.st, text, Evidence{Path: path, Line: line})
	}
	if !ok || guardErr != nil {
		why := "the contest evidence is no longer a source"
		if guardErr != nil {
			why = guardErr.Error()
		}
		rc.st.moveLineage(l, "arbiter", StOpen, "arbiter", "contest dropped: "+why, round)
		return false, nil
	}
	j, err := rc.prepareArbiter(l, scope, Block{Path: path, Text: excerpt(text, line, line), Start: max(1, line-arbiterWindow)})
	if err != nil {
		return false, err
	}
	out, reason, ran, err := rc.runArbiter(ctx, j, &budgetGate{cy: rc.st.cycle(), max: rc.s.set.MaxAgentRuns})
	if err != nil {
		return ran, err
	}
	if out == nil {
		l.ArbiterNotRun++
		if l.ArbiterNotRun >= 2 {
			rc.st.moveLineage(l, "arbiter", StEscalated, "arbiter", "two consecutive arbiter not_run: "+reason, round)
		}
		return ran, nil
	}
	l.ArbiterNotRun = 0
	rc.applyVerdict(l, out, round)
	return true, nil
}

// applyVerdict is R29's outcomes.
func (rc *roundCtx) applyVerdict(l *Lineage, out *ArbiterOutput, round int) {
	switch {
	case out.Verdict == "uphold":
		rc.st.moveLineage(l, "arbiter", StOpen, "arbiter", "upheld: "+out.Rationale, round)
	case l.Severity == SevNit || isChecker(l.Owner):
		rc.st.moveLineage(l, "arbiter", StSettled, "arbiter", "withdrawn: "+out.Rationale, round)
	default:
		// A withdrawn blocker or major goes back to its owner, who confirms
		// in the next round (still_holds: false) or reopens it.
		l.Confirmed = false
		rc.st.moveLineage(l, "arbiter", StFixed, "arbiter", "withdrawn: "+out.Rationale, round)
	}
}

func (rc *roundCtx) runArbiter(ctx context.Context, j *arbiterJob, bud *budgetGate) (out *ArbiterOutput, reason string, ran bool, err error) {
	reason = "budget"
	rc.env.runner.ResetUnattributed()
	for attempt := 1; attempt <= 2; attempt++ {
		if !bud.take() {
			return nil, "budget", ran, nil
		}
		ran = true
		res := rc.env.runner.Run(ctx, &GoldfishRequest{Owner: j.owner, Attempt: attempt, Packet: j.packet, Prompt: j.prompt})
		switch {
		case ctx.Err() != nil:
			return nil, "", ran, errorf(ExitError, "interrupted", "interrupted while the arbiter ran; nothing was recorded")
		case rc.env.runner.Escaped(j.owner):
			return nil, "escaped_process", ran, nil
		case rc.env.runner.Unattributed():
			rc.env.runner.ResetUnattributed()
			return nil, "unattributed_orphan", ran, nil
		case !res.Completed:
			reason = res.Reason
			if noRetry(reason) {
				return nil, reason, ran, nil
			}
			continue
		}
		o, why := ParseArbiterOutput(res.Payload, j.nonce, rc.s.doc, j.rng, rc.s.set.Attest.MinQuotes)
		if why == "" {
			return o, "", ran, nil
		}
		reason = why
	}
	return nil, reason, ran, nil
}

// prepareArbiter writes the arbiter packet (R1) with only the scope ±20
// lines of the document and the evidence ±20 lines (R29). The flagged lines
// of an injection-scan lineage go only in TRECHO-SINALIZADO.
func (rc *roundCtx) prepareArbiter(l *Lineage, scope Scope, evidence Block) (*arbiterJob, error) {
	s := rc.s
	persona, err := LoadPersona("arbiter")
	if err != nil {
		return nil, err
	}
	rc.st.NextArbiter++
	m := rc.st.NextArbiter
	rng := window(scope, len(s.doc.Lines))
	j := &arbiterJob{
		owner:  fmt.Sprintf("egm:%s:a%d:%s", Slug(s.rel), m, l.ID),
		packet: filepath.Join(s.stateDir, fmt.Sprintf("arbiter-%d", m), l.ID),
		nonce:  newNonce(),
		rng:    rng,
	}
	flagged, mask := "", Scope{}
	if l.Owner == injectionChecker {
		flagged, mask = flaggedExcerpt(s.doc, scope), scope
	}
	blocks := []Block{{Path: s.rel, Text: docExcerpt(s.doc, rng, mask), Start: rng[0]}}
	if evidence.Path != s.rel || evidence.Start < rng[0] || evidence.Start > rng[1] {
		blocks = append(blocks, evidence)
	}
	item := PriorItem{Lineage: l.ID, Severity: l.Severity, Status: l.Status, Scope: scope, Message: l.Message, Rationale: l.Rationale, Evidence: l.Evidence}
	j.prompt = BuildArbiterPrompt(&ArbiterInput{Nonce: j.nonce, Persona: persona, Flagged: flagged, Blocks: blocks, MinQuotes: s.set.Attest.MinQuotes, Lineage: item})
	return j, writeAtomic(j.packet, "prompt.txt", []byte(j.prompt))
}

// excerpt returns the lines lo-20..hi+20 of text.
func excerpt(text string, lo, hi int) string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	a, b := max(1, lo-arbiterWindow), min(len(lines), hi+arbiterWindow)
	if a > b {
		return ""
	}
	return strings.Join(lines[a-1:b], "\n") + "\n"
}

// docExcerpt returns the document lines of rng, with the masked scope lines
// replaced by a marker.
func docExcerpt(d *Doc, rng [2]int, mask Scope) string {
	var b strings.Builder
	for n := rng[0]; n <= rng[1] && n <= len(d.Lines); n++ {
		if len(mask.Lines) == 2 && n >= mask.Lines[0] && n <= mask.Lines[1] {
			b.WriteString("[see TRECHO-SINALIZADO]\n")
			continue
		}
		b.WriteString(d.Lines[n-1] + "\n")
	}
	return b.String()
}

// window is the scope ±20 lines, clipped to the document.
func window(sc Scope, n int) [2]int {
	if len(sc.Lines) != 2 {
		return [2]int{1, n}
	}
	return [2]int{max(1, sc.Lines[0]-arbiterWindow), min(n, sc.Lines[1]+arbiterWindow)}
}

// flaggedExcerpt quotes the scope lines of an injection-scan lineage and
// escapes the block delimiters, so the arbiter sees them only as data
// (TRECHO-SINALIZADO, R17a).
func flaggedExcerpt(d *Doc, sc Scope) string {
	if len(sc.Lines) != 2 {
		return ""
	}
	var b strings.Builder
	for n := sc.Lines[0]; n <= sc.Lines[1] && n <= len(d.Lines); n++ {
		line := strings.ReplaceAll(d.Lines[n-1], "<<<", "‹‹‹")
		line = strings.ReplaceAll(line, ">>>", "›››")
		fmt.Fprintf(&b, "> %d: %s\n", n, line)
	}
	return b.String()
}
