package egm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveOptions are the R28 flags.
type ResolveOptions struct {
	Options
	Fix         []string
	Decline     []string
	ForceSettle string
	Reason      string
	Contest     string
	Evidence    string
}

type resolveOp struct {
	kind, id string
}

func (r *ResolveOptions) ops() []resolveOp {
	var ops []resolveOp
	for _, id := range r.Fix {
		ops = append(ops, resolveOp{"fix", id})
	}
	for _, id := range r.Decline {
		ops = append(ops, resolveOp{"decline", id})
	}
	if r.ForceSettle != "" {
		ops = append(ops, resolveOp{"force-settle", r.ForceSettle})
	}
	return ops
}

// Resolve runs `tb egm resolve` (R28). Every transition is validated before
// any is applied: an invalid one changes nothing.
func Resolve(ctx context.Context, o *ResolveOptions) (*Outcome, error) {
	s, err := prepare(ctx, &o.Options)
	if err != nil {
		return nil, err
	}
	if o.Contest != "" {
		return nil, errorf(ExitError, "contest_refused", "--contest needs the llm engine with an agent (arbiter, R29); not available in this build")
	}
	ops := o.ops()
	if len(ops) == 0 {
		return nil, errorf(ExitError, "usage", "nothing to resolve: pass --fix, --decline or --force-settle")
	}
	if o.ForceSettle != "" && strings.TrimSpace(o.Reason) == "" {
		return nil, errorf(ExitError, "usage", "--force-settle needs --reason")
	}
	unlock, err := lockState(s.stateDir)
	if err != nil {
		return nil, err
	}
	defer unlock()
	st, err := loadState(s.stateDir)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, errorf(ExitError, "no_state", "%s has no state: run `tb egm review` first", s.rel)
	}
	rv := &resolver{s: s, st: st, ctx: ctx, round: len(st.Rounds)}
	var applies []func()
	for _, op := range ops {
		f, err := rv.validate(op, o.Reason)
		if err != nil {
			return nil, err
		}
		applies = append(applies, f)
	}
	if rv.needSnapshot {
		st.NextResolve++
		if err := s.writeSnapshot(filepath.Join(s.stateDir, fmt.Sprintf("resolve-%d", st.NextResolve))); err != nil {
			return nil, err
		}
	}
	for _, f := range applies {
		f()
	}
	if err := saveState(s.stateDir, st); err != nil {
		return nil, err
	}
	rep := s.report(st, st.lastRound(), "", "")
	return &Outcome{Report: rep, Message: strings.Join(rv.msgs, "; ")}, nil
}

type resolver struct {
	s            *snapshot
	st           *State
	ctx          context.Context
	round        int
	needSnapshot bool
	msgs         []string
}

func (rv *resolver) move(l *Lineage, cmd, to, reason string) {
	rv.st.Transitions = append(rv.st.Transitions, Transition{
		Command: cmd, Lineage: l.ID, From: l.Status, To: to, Round: rv.round, Reason: reason, By: "elephant",
	})
	l.Status, l.RefRound = to, rv.round
	rv.msgs = append(rv.msgs, fmt.Sprintf("%s → %s", l.ID, to))
}

func (rv *resolver) validate(op resolveOp, reason string) (func(), error) {
	l := rv.st.lineage(op.id)
	if l == nil {
		return nil, errorf(ExitError, "invalid_transition", "unknown lineage %s", op.id)
	}
	switch op.kind {
	case "fix":
		return rv.validateFix(l)
	case "decline":
		return rv.validateDecline(l)
	default:
		return rv.validateForce(l, reason)
	}
}

func invalid(l *Lineage, op, why string) error {
	return errorf(ExitError, "invalid_transition", "%s %s from %s: %s", op, l.ID, l.Status, why)
}

func (rv *resolver) validateFix(l *Lineage) (func(), error) {
	if l.Status == StFixed {
		return func() { rv.msgs = append(rv.msgs, l.ID+" already fixed (no-op)") }, nil
	}
	ok, err := rv.touched(l)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, invalid(l, "--fix", fmt.Sprintf("no change since round %d touches its lines or the evidence ±5 lines", l.RefRound))
	}
	rv.needSnapshot = true
	return func() {
		l.Confirmed = false
		rv.move(l, "fix", StFixed, "")
	}, nil
}

func (rv *resolver) validateDecline(l *Lineage) (func(), error) {
	switch {
	case l.Status == StDeclined:
		return func() { rv.msgs = append(rv.msgs, l.ID+" already declined (no-op)") }, nil
	case l.Status != StOpen && l.Status != StReopened:
		return nil, invalid(l, "--decline", "only open or reopened lineages can be declined")
	case l.Contests != 0:
		return nil, invalid(l, "--decline", "a contested lineage cannot be declined")
	case l.Severity == SevBlocker:
		return nil, invalid(l, "--decline", "a blocker is never declined")
	}
	return func() {
		if l.Severity == SevMajor {
			l.Downgrade = "declined_major"
		}
		rv.move(l, "decline", StDeclined, "")
	}, nil
}

func (rv *resolver) validateForce(l *Lineage, reason string) (func(), error) {
	switch l.Status {
	case StSettled:
		return func() { rv.msgs = append(rv.msgs, l.ID+" already settled (no-op)") }, nil
	case StOpen, StReopened, StContestPending, StEscalated:
	default:
		return nil, invalid(l, "--force-settle", "only open, reopened, contest_pending or escalated lineages can be force-settled")
	}
	return func() {
		l.Downgrade = "forced"
		if l.Severity == SevBlocker {
			l.Downgrade = "forced_blocker"
		}
		rv.move(l, "force-settle", StSettled, reason)
	}, nil
}

func (rv *resolver) readRound(n int, rel string) (string, error) {
	p := filepath.Join(rv.s.stateDir, fmt.Sprintf("round-%d", n), "target.md")
	if rel != rv.s.rel {
		p = filepath.Join(rv.s.stateDir, fmt.Sprintf("round-%d", n), "sources", filepath.FromSlash(rel))
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

// touched is R28's change test: since the reference round, a non-whitespace
// change touched the mapped scope or the evidence ±5 lines of a cited source.
func (rv *resolver) touched(l *Lineage) (bool, error) {
	ref, err := rv.readRound(l.RefRound, rv.s.rel)
	if err != nil {
		return false, err
	}
	lm, err := DiffSnapshots(rv.ctx, rv.s.stateDir, ref, rv.s.text)
	if err != nil {
		return false, err
	}
	changed := lm.Changed()
	scope := l.Scope
	if l.ScopeRound != l.RefRound {
		old, err := rv.readRound(l.ScopeRound, rv.s.rel)
		if err != nil {
			return false, err
		}
		if sm, err := DiffSnapshots(rv.ctx, rv.s.stateDir, old, rv.s.text); err == nil {
			scope = sm.MapScope(scope)
		}
	} else {
		scope = lm.MapScope(scope)
	}
	if Touches(scope.Lines, changed, 0) {
		return true, nil
	}
	return rv.evidenceTouched(l, lm)
}

func (rv *resolver) evidenceTouched(l *Lineage, docMap *LineMap) (bool, error) {
	for _, e := range l.Evidence {
		if !containsStr(l.Cited, e.Path) {
			continue
		}
		if e.Path == rv.s.rel {
			if Touches([]int{docMap.Map(e.Line), docMap.Map(e.Line)}, docMap.Changed(), 5) {
				return true, nil
			}
			continue
		}
		ok, err := rv.sourceTouched(l.RefRound, e)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

func (rv *resolver) sourceTouched(ref int, e Evidence) (bool, error) {
	old, err := rv.readRound(ref, e.Path)
	if err != nil {
		return false, err
	}
	cur := ""
	for _, src := range rv.s.sources.Included {
		if src.Path == e.Path {
			cur = src.Text
		}
	}
	sm, err := DiffSnapshots(rv.ctx, rv.s.stateDir, old, cur)
	if err != nil {
		return false, err
	}
	return Touches([]int{sm.Map(e.Line), sm.Map(e.Line)}, sm.Changed(), 5), nil
}
