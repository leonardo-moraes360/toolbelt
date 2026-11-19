package egm

import (
	"context"
	"fmt"
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
	if r.Contest != "" {
		ops = append(ops, resolveOp{"contest", r.Contest})
	}
	return ops
}

func (r *ResolveOptions) checkOps() ([]resolveOp, error) {
	if r.Contest != "" && r.Evidence == "" {
		return nil, errorf(ExitError, "usage", "--contest needs --evidence path:line")
	}
	if r.ForceSettle != "" && strings.TrimSpace(r.Reason) == "" {
		return nil, errorf(ExitError, "usage", "--force-settle needs --reason")
	}
	ops := r.ops()
	if len(ops) == 0 {
		return nil, errorf(ExitError, "usage", "nothing to resolve: pass --fix, --decline, --force-settle or --contest")
	}
	return ops, nil
}

// Resolve runs `tb egm resolve` (R28). Every transition is validated before
// any is applied: an invalid one changes nothing.
func Resolve(ctx context.Context, o *ResolveOptions) (*Outcome, error) {
	s, err := prepare(ctx, &o.Options)
	if err != nil {
		return nil, err
	}
	ops, err := o.checkOps()
	if err != nil {
		return nil, err
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
		arg := o.Reason
		if op.kind == "contest" {
			arg = o.Evidence
		}
		f, err := rv.validate(op, arg)
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
	case "contest":
		return rv.validateContest(rv.ctx, l, reason)
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
	ok, err := rv.s.touchedSince(rv.ctx, l, l.RefRound)
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
