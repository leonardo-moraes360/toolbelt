//go:build linux

package egm

import (
	"context"
	"testing"
)

// llmConverged reviews lf's spec under modes until it converges, writes the
// receipt and commits it.
func (lf *llmFlow) llmConverged(ctx context.Context, resolve func(*Report)) {
	lf.t.Helper()
	r := lf.must(Review(ctx, lf.optsP())).Report
	if resolve != nil {
		resolve(r)
	}
	for i := 0; i < 6 && r.LoopState != "converged"; i++ {
		r = lf.must(LoopNext(ctx, lf.optsP())).Report
	}
	if r.LoopState != "converged" {
		lf.t.Fatalf("did not converge: %s %+v", r.Verdict, r.Findings)
	}
	lf.commitReceipt(ctx)
}

func ciResult(r *GateResult) string {
	if len(r.Docs) == 0 || len(r.Docs[0].CIFindings) == 0 {
		return ""
	}
	return r.Docs[0].CIFindings[0].Result
}

// TestCIFreshRound is T-R2 step 3 and T-R3: a clean fresh round passes; a
// new major fails as ci_finding.
func TestCIFreshRound(t *testing.T) {
	lf := newLLMFlow(t, "")
	ctx := context.Background()
	lf.llmConverged(ctx, nil)
	r := lf.ciGate(ctx, "HEAD~2", false)
	if !r.OK || r.Warn || r.Docs[0].Verdict != VerdictPass {
		t.Fatalf("clean: %+v", r.Docs)
	}
	lf.setModes(map[string][]string{"*": {"clean"}, "design": {"findings"}})
	r = lf.ciGate(ctx, "HEAD~2", false)
	if r.OK || ciResult(r) != "ci_finding" {
		t.Fatalf("new major: %+v", r.Docs)
	}
}

// TestCIDeclinedClaim: a CI finding matching a declined claim fails as
// ci_downgrade.
func TestCIDeclinedClaim(t *testing.T) {
	lf := newLLMFlow(t, "")
	ctx := context.Background()
	lf.setModes(map[string][]string{"*": {"clean"}, "design": {"findings"}})
	lf.llmConverged(ctx, func(r *Report) {
		lf.must(Resolve(ctx, &ResolveOptions{Options: lf.opts(), Decline: []string{r.Findings[0].ID}}))
	})
	r := lf.ciGate(ctx, "HEAD~2", false)
	if r.OK || ciResult(r) != "ci_downgrade" || r.Docs[0].CIFindings[0].Claim == "" {
		t.Fatalf("declined: %+v", r.Docs)
	}
}

// TestCIArbiterClaim: a CI finding matching an arbiter_withdrawn claim is
// re-arbitrated: withdraw passes, uphold fails as ci_upheld.
func TestCIArbiterClaim(t *testing.T) {
	lf := newLLMFlow(t, "")
	ctx := context.Background()
	lf.setModes(map[string][]string{"*": {"clean"}, "design": {"findings"}})
	r := lf.must(Review(ctx, lf.optsP())).Report
	ev := r.Findings[0].Evidence[0]
	lf.must(Resolve(ctx, &ResolveOptions{Options: lf.opts(), Contest: "F-1", Evidence: ev.Path + ":" + itoa(int64(ev.Line))}))
	lf.setModes(map[string][]string{"*": {"clean"}, "arbiter": {"withdraw"}})
	for i := 0; i < 6 && r.LoopState != "converged"; i++ {
		r = lf.must(LoopNext(ctx, lf.optsP())).Report
	}
	lf.commitReceipt(ctx)

	lf.setModes(map[string][]string{"*": {"clean"}, "design": {"findings"}, "arbiter": {"withdraw"}})
	g := lf.ciGate(ctx, "HEAD~2", false)
	if !g.OK || ciResult(g) != "withdrawn" || g.Docs[0].CIFindings[0].Arbiter != "withdraw" {
		t.Fatalf("withdraw: %+v", g.Docs)
	}
	lf.setModes(map[string][]string{"*": {"clean"}, "design": {"findings"}, "arbiter": {"uphold"}})
	g = lf.ciGate(ctx, "HEAD~2", false)
	if g.OK || ciResult(g) != "ci_upheld" {
		t.Fatalf("uphold: %+v", g.Docs)
	}
}

// TestCIForcedClaim: a CI finding matching a forced claim fails as
// ci_downgrade, like a declined one.
func TestCIForcedClaim(t *testing.T) {
	lf := newLLMFlow(t, "")
	ctx := context.Background()
	lf.setModes(map[string][]string{"*": {"clean"}, "design": {"findings"}})
	lf.llmConverged(ctx, func(r *Report) {
		lf.must(Resolve(ctx, &ResolveOptions{Options: lf.opts(), ForceSettle: r.Findings[0].ID, Reason: "accepted risk"}))
	})
	if r := lf.ciGate(ctx, "HEAD~2", false); r.OK || ciResult(r) != "ci_downgrade" {
		t.Fatalf("forced: %+v", r.Docs)
	}
}
