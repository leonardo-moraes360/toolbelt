package egm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// CI mode of the egm gate (docs/specs/egm-trust.md, T-R2 to T-R6): the
// receipt committed at head is checked against the commit, then CI runs one
// fresh round of its own and matches the new findings against the claims.

const (
	GateModeLocal = "local"
	GateModeCI    = "ci"

	maxReceiptBytes = 1 << 20
)

// CIFinding is a finding of CI's fresh round that decided or warned.
type CIFinding struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Heading  string `json:"heading,omitempty"`
	Lines    []int  `json:"lines,omitempty"`
	Message  string `json:"message"`
	Claim    string `json:"claim,omitempty"`   // the matched receipt claim
	Result   string `json:"result"`            // ci_finding, ci_downgrade, ci_upheld, withdrawn, warn
	Arbiter  string `json:"arbiter,omitempty"` // CI arbiter verdict on an arbiter_withdrawn match
}

// readReceiptAt reads and validates rel's receipt blob at oid. code is
// no_receipt or receipt_invalid when it fails.
func readReceiptAt(ctx context.Context, repo *Repo, oid, rel string, docLines int) (rc *Receipt, code string, err error) {
	b, mode, err := readBlobFile(ctx, repo, oid, ReceiptPath(rel), maxReceiptBytes)
	var e *Error
	switch {
	case errors.As(err, &e) && e.Reason == "blob_too_large":
		return nil, "receipt_invalid", nil
	case err != nil:
		return nil, "", err
	case mode == "":
		return nil, "no_receipt", nil
	case b == nil:
		return nil, "receipt_invalid", nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	rc = &Receipt{}
	if err := dec.Decode(rc); err != nil || !rc.valid(rel, docLines) {
		return nil, "receipt_invalid", nil
	}
	return rc, "", nil
}

func (r *Receipt) valid(rel string, docLines int) bool {
	if r.Version != receiptVersion || r.Doc != rel || r.DocSHA256 == "" || r.Sources == nil {
		return false
	}
	for i := range r.Claims {
		if !r.Claims[i].valid(docLines) {
			return false
		}
	}
	return true
}

func (c *Claim) valid(docLines int) bool {
	switch c.Kind {
	case ClaimArbiterWithdrawn, ClaimDeclined, ClaimForced:
	default:
		return false
	}
	return c.ID != "" && c.Message != "" && sevRank(c.Severity) >= sevRank(SevMajor) &&
		len(c.Lines) == 2 && c.Lines[0] >= 1 && c.Lines[0] <= c.Lines[1] && c.Lines[1] <= docLines
}

// fresh reports whether the receipt's hashes are the snapshot's (R7a at head).
func (r *Receipt) fresh(s *snapshot) bool {
	if r.DocSHA256 != s.docSHA || len(r.Sources) != len(s.sources.Included) {
		return false
	}
	for _, src := range s.sources.Included {
		if r.Sources[src.Path] != src.SHA256 {
			return false
		}
	}
	return true
}

// receiptCheck is T-R2 steps 1 and 2 for rel at head: "" when the receipt
// is present, valid and fresh; otherwise no_receipt, receipt_invalid or
// receipt_stale.
func receiptCheck(ctx context.Context, root, rel, head, cfg string) (*Receipt, *snapshot, string, error) {
	s, err := prepare(ctx, &Options{Doc: rel, Cwd: root, ConfigFile: cfg, At: head})
	if err != nil {
		return nil, nil, "", err
	}
	rc, code, err := readReceiptAt(ctx, s.repo, s.at, s.rel, len(s.doc.Lines))
	if err != nil || code != "" {
		return nil, s, code, err
	}
	if !rc.fresh(s) {
		return rc, s, "receipt_stale", nil
	}
	return rc, s, "", nil
}

// ciEngine reports why the llm engine cannot run with set, or "" (T-R4).
func ciEngine(set *Settings) string {
	_, _, env, err := resolveEngine(&Options{Engine: EngineLLM}, set)
	if err != nil {
		return err.Error()
	}
	env.close()
	return ""
}

// ciRun carries the step-wide state of a CI gate.
type ciRun struct {
	o        *GateOptions
	root     string
	head     string
	offline  string // why the llm engine cannot run; "" when it can
	runsLeft int    // egm.max_agent_runs across the whole step (T-R6)
}

// doc is T-R2 for one document.
func (c *ciRun) doc(ctx context.Context, rel string) GateDoc {
	gd := GateDoc{Doc: rel, Verdict: VerdictFail, ExitCode: ExitFail}
	rc, s, code, err := receiptCheck(ctx, c.root, rel, c.head, c.o.ConfigFile)
	if err != nil {
		return GateDoc{Doc: rel, Verdict: "ERROR", ExitCode: ExitError, Reason: err.Error()}
	}
	if code != "" {
		gd.Reason = code
		return gd
	}
	engine := EngineLLM
	if c.offline != "" {
		if !c.o.AllowOffline {
			gd.Reason = "ci_offline: " + c.offline
			return gd
		}
		engine = EngineDeterministic
	}
	if c.runsLeft <= 0 && engine == EngineLLM {
		gd.Reason = "ci_budget"
		return gd
	}
	tmp, err := os.MkdirTemp("", "tb-egm-ci-")
	if err != nil {
		return GateDoc{Doc: rel, Verdict: "ERROR", ExitCode: ExitError, Reason: err.Error()}
	}
	defer os.RemoveAll(tmp)
	o := &Options{Doc: rel, Cwd: c.root, ConfigFile: c.o.ConfigFile, At: c.head, Engine: engine, stateRoot: tmp, maxRuns: c.runsLeft}
	out, err := Review(ctx, o)
	var e *Error
	switch {
	case errors.As(err, &e) && e.Reason == "budget":
		gd.Reason = "ci_budget"
		return gd
	case err != nil:
		return GateDoc{Doc: rel, Verdict: "ERROR", ExitCode: ExitError, Reason: err.Error()}
	}
	c.runsLeft -= out.Report.Budget.Used
	if why := notRun(out.Report.Coverage); why != "" && engine == EngineLLM {
		gd.Reason = "ci_not_run: " + why
		return gd
	}
	return c.classify(ctx, o, s, rc, out.Report, engine)
}

// notRun names a required reviewer that did not complete.
func notRun(cv Coverage) string {
	for _, r := range cv.Reviewers {
		if containsStr(cv.Required, r.Name) && r.Status != "completed" {
			return r.Name + " " + r.Status
		}
	}
	return ""
}

// classify is T-R3 over the fresh round's findings.
func (c *ciRun) classify(ctx context.Context, o *Options, s *snapshot, rc *Receipt, rep *Report, engine string) GateDoc {
	gd := GateDoc{Doc: rc.Doc, Verdict: VerdictPass, OK: true, ExitCode: ExitOK, CIFindings: []CIFinding{}}
	var arbitrate []string
	for i := range rep.Findings {
		f := &rep.Findings[i]
		cf := CIFinding{ID: f.ID, Severity: f.Severity, Heading: f.Scope.Heading, Lines: f.Scope.Lines, Message: f.Message}
		if sevRank(f.Severity) < sevRank(SevMajor) {
			cf.Result = "warn"
			gd.Warn = true
			gd.CIFindings = append(gd.CIFindings, cf)
			continue
		}
		claim := matchClaim(f, rc.Claims, &s.set)
		switch {
		case claim == nil:
			cf.Result = "ci_finding"
		case claim.Kind == ClaimArbiterWithdrawn:
			cf.Claim, cf.Result = claim.ID, "arbiter"
			arbitrate = append(arbitrate, f.ID)
		default:
			cf.Claim, cf.Result = claim.ID, "ci_downgrade"
		}
		gd.CIFindings = append(gd.CIFindings, cf)
	}
	if len(arbitrate) > 0 {
		c.arbitrate(ctx, o, engine, arbitrate, gd.CIFindings)
	}
	for _, cf := range gd.CIFindings {
		if cf.Result != "warn" && cf.Result != "withdrawn" && gd.OK {
			gd.OK, gd.Verdict, gd.ExitCode, gd.Reason = false, VerdictFail, ExitFail, cf.Result+": "+cf.ID
		}
	}
	if gd.OK && (gd.Warn || engine != EngineLLM) {
		gd.Verdict, gd.Warn = VerdictPassPartial, true
		if engine != EngineLLM {
			gd.Downgrades = append(gd.Downgrades, "ci_offline")
		}
	}
	return gd
}

// matchClaim is T-R3's match: R18's region and sim, owner and persona
// ignored; a finding more severe than the claim is unmatched (R20).
func matchClaim(f *ReportFinding, claims []Claim, set *Settings) *Claim {
	for i := range claims {
		cl := &claims[i]
		if sevRank(f.Severity) > sevRank(cl.Severity) {
			continue
		}
		if Region(f.Scope, Scope{Heading: cl.Heading, Lines: cl.Lines}, set.Match.Gap) && Sim(f.Message, cl.Message, set.Match.Stem) >= set.Match.Sim {
			return cl
		}
	}
	return nil
}

// arbitrate runs CI's own arbiter on each CI lineage matched to an
// arbiter_withdrawn claim (T-R3), in the same throwaway state, and records
// the verdict in cfs.
func (c *ciRun) arbitrate(ctx context.Context, o *Options, engine string, ids []string, cfs []CIFinding) {
	verdicts := map[string]string{}
	if engine == EngineLLM {
		var err error
		if verdicts, err = ciArbiter(ctx, o, ids, &c.runsLeft); err != nil {
			verdicts = map[string]string{}
		}
	}
	for i := range cfs {
		if cfs[i].Result != "arbiter" {
			continue
		}
		v := verdicts[cfs[i].ID]
		cfs[i].Arbiter = v
		switch v {
		case "withdraw":
			cfs[i].Result = "withdrawn"
		case "":
			cfs[i].Arbiter, cfs[i].Result = "not_run", "ci_upheld"
		default:
			cfs[i].Result = "ci_upheld"
		}
	}
}

// ciArbiter runs the arbiter on each lineage id of the throwaway state:
// "withdraw", "uphold" or "not_run".
func ciArbiter(ctx context.Context, o *Options, ids []string, runsLeft *int) (map[string]string, error) {
	s, err := prepare(ctx, o)
	if err != nil {
		return nil, err
	}
	_, _, env, err := resolveEngine(&Options{Engine: EngineLLM}, &s.set)
	if err != nil {
		return nil, err
	}
	defer env.close()
	unlock, err := lockState(s.stateDir)
	if err != nil {
		return nil, err
	}
	defer unlock()
	st, err := loadState(s.stateDir)
	if err != nil || st == nil {
		return nil, fmt.Errorf("ci state: %v", err)
	}
	rc := &roundCtx{s: s, o: o, st: st, engine: EngineLLM, env: env}
	mapper, err := s.mapper(ctx, st)
	if err != nil {
		return nil, err
	}
	before := st.cycle().AgentRuns
	out := map[string]string{}
	for _, id := range ids {
		l := st.lineage(id)
		if l == nil {
			continue
		}
		notRun := l.ArbiterNotRun
		if _, err := rc.arbiter(ctx, l, mapper(l)); err != nil {
			return out, err
		}
		switch {
		case l.ArbiterNotRun > notRun:
			out[id] = "not_run"
		case arbiterWithdrew(st, id):
			out[id] = "withdraw"
		default:
			out[id] = "uphold"
		}
	}
	*runsLeft -= st.cycle().AgentRuns - before
	return out, nil
}

// ciPreflight is the CI mode's usage check: -c is required (T-R2).
func ciPreflight(o *GateOptions) error {
	switch o.Mode {
	case "", GateModeLocal:
		return nil
	case GateModeCI:
		if strings.TrimSpace(o.ConfigFile) == "" {
			return errorf(ExitError, "usage", "--egm-mode ci needs -c <config>: CI's own user config (T-R5)")
		}
		return nil
	}
	return errorf(ExitError, "usage", "--egm-mode must be local or ci")
}
