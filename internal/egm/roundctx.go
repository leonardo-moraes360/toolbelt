package egm

import (
	"context"
	"fmt"
)

// roundCtx runs one review or loop next under the state lock.
type roundCtx struct {
	s          *snapshot
	o          *Options
	st         *State
	engine     string
	reason     string
	env        *llmEnv
	isReview   bool
	arbiterRan bool
	// llmFindings are phase 2's findings, merged after the checkers (R19).
	llmFindings []*Finding
}

func (rc *roundCtx) run(ctx context.Context) (*Outcome, error) {
	s, st := rc.s, rc.st
	if done, err := s.gateEntry(st, rc.o, rc.isReview); err != nil {
		return nil, err
	} else if !done {
		if out, err := rc.gateLoop(ctx); out != nil || err != nil {
			return out, err
		}
	}
	if rc.st == nil {
		rc.st = newState(s.rel, Slug(s.rel))
	}
	planned := rc.plan()
	if err := rc.ownersRun(planned); err != nil {
		return nil, rc.keepArbiters(err)
	}
	pending := (*State)(nil)
	if rc.o.DryRun {
		pending = rc.st // arbiters did not run: count the pending contests (R27)
	}
	need, left := worstCase(planned, pending), rc.budgetLeft()
	if rc.o.DryRun {
		return &Outcome{Report: s.report(rc.st, rc.st.lastRound(), rc.engine, rc.reason),
			Message: fmt.Sprintf("dry run: worst case %d agent runs; %d left in the cycle budget", need, left)}, nil
	}
	if need > left {
		return nil, rc.keepArbiters(errorf(ExitError, "budget", "the round needs up to %d agent runs and the cycle has %d left (egm.max_agent_runs)", need, left))
	}
	rd, err := rc.round(ctx, planned)
	if err != nil {
		return nil, err // the state was changed in memory only: nothing is saved
	}
	rep := s.report(rc.st, rd, rc.engine, rc.reason)
	if err := s.persist(rc.st, rd, rep); err != nil {
		return nil, err
	}
	return &Outcome{Report: rep, ExitCode: exitFor(rep.Verdict)}, nil
}

// gateLoop is loop next's order: cap (R23), engine fallback (R26), the
// arbiters of pending contests, then R26's no-change refusal.
func (rc *roundCtx) gateLoop(ctx context.Context) (*Outcome, error) {
	s, st := rc.s, rc.st
	last := st.lastRound()
	if cy := st.cycle(); cy != nil && cy.Capped {
		return &Outcome{Report: s.report(st, last, rc.engine, ""), ExitCode: ExitFail,
			Message: fmt.Sprintf("cycle %d is capped at %d rounds: edit and run `tb egm review --restart`", cy.N, s.set.MaxRounds)}, nil
	}
	if rc.engine == EngineDeterministic && rc.o.Engine != EngineDeterministic && llmOwned(st) {
		return nil, errorf(ExitError, "owner_unavailable", "the llm engine is unavailable (%s) and the state has lineages owned by LLM reviewers: pass --engine deterministic to fall back", rc.reason)
	}
	if rc.env != nil && !rc.o.DryRun {
		ran, err := rc.arbiters(ctx)
		if err != nil {
			return nil, err
		}
		rc.arbiterRan = ran
	}
	if last != nil && last.Content == s.content && stuck(st, rc.env != nil) {
		msg := fmt.Sprintf("documento e fontes não mudaram desde a rodada %d; há findings bloqueantes abertos (document and sources have not changed since round %d; there are open blocking findings)", last.N, last.N)
		if rc.arbiterRan {
			if err := saveState(s.stateDir, st); err != nil {
				return nil, err
			}
			return &Outcome{Report: s.report(st, last, rc.engine, rc.reason), Message: msg}, nil
		}
		return nil, errorf(ExitError, "no_change", "%s", msg)
	}
	return nil, nil
}

// keepArbiters saves the arbiters' outcomes when the round is then refused,
// so their runs are not paid twice.
func (rc *roundCtx) keepArbiters(err error) error {
	if rc.arbiterRan {
		if serr := saveState(rc.s.stateDir, rc.st); serr != nil {
			return serr
		}
	}
	return err
}

func llmOwned(st *State) bool {
	for _, l := range st.Lineages {
		if !isChecker(l.Owner) && l.Status != StSettled && l.Status != StDeclined {
			return true
		}
	}
	return false
}

func (rc *roundCtx) plan() []ReviewerRun {
	if rc.env == nil {
		return planReviewers(rc.s.docType, rc.s.hasDetailed, rc.o.Reviewers)
	}
	return rc.s.planReviewersLLM(rc.o.Reviewers)
}

// ownersRun is R26's owner_unavailable: a blocking lineage whose owner does
// not run in this round; the attempt does not count toward the cap.
func (rc *roundCtx) ownersRun(planned []ReviewerRun) error {
	if rc.st == nil {
		return nil
	}
	runs := map[string]bool{}
	for _, r := range planned {
		runs[r.Name] = r.Status == "pending"
	}
	for _, l := range rc.st.Lineages {
		if l.blocking() && !isChecker(l.Owner) && !runs[l.Owner] {
			return errorf(ExitError, "owner_unavailable", "%s is owned by %s, which does not run in this round (%s engine)", l.ID, l.Owner, rc.engine)
		}
	}
	return nil
}

func (rc *roundCtx) budgetLeft() int {
	used := 0
	if cy := rc.st.cycle(); cy != nil && !cy.Converged && !rc.o.Restart {
		used = cy.AgentRuns
	}
	return rc.s.set.MaxAgentRuns - used
}

// round opens round n, runs phase 2 (llm) and phase 3 (match, lineage,
// convergence).
func (rc *roundCtx) round(ctx context.Context, planned []ReviewerRun) (*Round, error) {
	s, st := rc.s, rc.st
	n := len(st.Rounds) + 1
	if cy := st.cycle(); cy == nil || cy.Converged || rc.o.Restart {
		st.Cycles = append(st.Cycles, Cycle{N: len(st.Cycles) + 1, FirstRound: n, Restart: rc.o.Restart})
	}
	rd := Round{
		N: n, Cycle: st.cycle().N, Content: s.content, PolicySHA: s.policy, DocSHA256: s.docSHA, HeadCommit: s.head,
		Sources: map[string]string{}, Engine: rc.engine, EngineReason: rc.reason, Required: RequiredReviewers(s.hasDetailed),
		Reviewers: planned,
	}
	for _, src := range s.sources.Included {
		rd.Sources[src.Path] = src.SHA256
	}
	r := &reconciler{st: st, round: n, docRel: s.rel, ran: map[string]bool{
		checkRefs: true, checkSections: true, checkPlan: true, injectionChecker: true,
	}}
	if err := rc.reviewersAndPrior(ctx, &rd, r); err != nil {
		return nil, err
	}
	mapper, err := s.mapper(ctx, st)
	if err != nil {
		return nil, err
	}
	r.mapper = mapper
	r.fold(s.checks.Findings)
	r.mergeLLM(rc.llmFindings, &s.set)
	rd.Warnings = append(rd.Warnings, r.warnings...)
	rd.Clean = roundClean(st, &rd, s.sources.Unstaged)
	rd.Downgrades = roundDowngrades(&rd, s.docType, s.hasDetailed, s.sources.MaxExcluded && codeReviewerRan(&rd))
	if rc.env != nil && !rc.env.prof.ToolsCheck {
		rd.Downgrades = append(rd.Downgrades, "tools_check_none")
	}
	st.Rounds = append(st.Rounds, rd)
	advanceCycle(st, &s.set)
	return st.lastRound(), nil
}
