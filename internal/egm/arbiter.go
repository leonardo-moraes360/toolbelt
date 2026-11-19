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
	ran := false
	for _, l := range rc.st.Lineages {
		if l.Status != StContestPending || l.ArbiterNotRun >= 2 {
			continue
		}
		did, err := rc.arbiter(ctx, l)
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

func (rc *roundCtx) arbiter(ctx context.Context, l *Lineage) (bool, error) {
	cy := rc.st.cycle()
	bud := &budgetGate{cy: cy, max: rc.s.set.MaxAgentRuns}
	j, err := rc.prepareArbiter(l)
	if err != nil {
		return false, err
	}
	out, reason, ran := rc.runArbiter(ctx, j, bud)
	round := len(rc.st.Rounds)
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

func (rc *roundCtx) runArbiter(ctx context.Context, j *arbiterJob, bud *budgetGate) (*ArbiterOutput, string, bool) {
	reason, ran := "budget", false
	for attempt := 1; attempt <= 2; attempt++ {
		if !bud.take() {
			reason = "budget"
			break
		}
		ran = true
		res := rc.env.runner.Run(ctx, &GoldfishRequest{Owner: j.owner, Attempt: attempt, Packet: j.packet, Prompt: j.prompt})
		if rc.env.runner.Escaped(j.owner) {
			return nil, "escaped_process", ran
		}
		if !res.Completed {
			reason = res.Reason
			if noRetry(reason) {
				break
			}
			continue
		}
		out, why := ParseArbiterOutput(res.Payload, j.nonce, rc.s.doc, j.rng, rc.s.set.Attest.MinQuotes)
		if why == "" {
			return out, "", ran
		}
		reason = why
	}
	return nil, reason, ran
}

func (rc *roundCtx) prepareArbiter(l *Lineage) (*arbiterJob, error) {
	s := rc.s
	persona, err := LoadPersona("arbiter")
	if err != nil {
		return nil, err
	}
	rc.st.NextArbiter++
	m := rc.st.NextArbiter
	j := &arbiterJob{
		owner:  fmt.Sprintf("egm:%s:a%d:%s", Slug(s.rel), m, l.ID),
		packet: filepath.Join(s.stateDir, fmt.Sprintf("arbiter-%d", m), l.ID),
		nonce:  newNonce(),
		rng:    window(l.Scope, len(s.doc.Lines)),
	}
	blocks := []Block{{Path: s.rel, Text: s.text}}
	if e := l.ContestEvidence; e != nil && e.Path != s.rel {
		for _, src := range s.sources.Included {
			if src.Path == e.Path {
				blocks = append(blocks, Block{Path: src.Path, Text: src.Text})
			}
		}
	}
	flagged := ""
	if l.Owner == injectionChecker {
		flagged = flaggedExcerpt(s.doc, l.Scope)
	}
	j.prompt = BuildArbiterPrompt(&ArbiterInput{Nonce: j.nonce, Persona: persona, Flagged: flagged, Blocks: blocks, MinQuotes: s.set.Attest.MinQuotes,
		Lineage: PriorItem{Lineage: l.ID, Severity: l.Severity, Status: l.Status, Scope: l.Scope, Message: l.Message, Rationale: l.Rationale, Evidence: l.Evidence}})
	return j, writeAtomic(j.packet, "prompt.txt", []byte(j.prompt))
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
