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
	if r.Doc != rel || r.DocSHA256 == "" || r.Sources == nil || !r.versionValid() {
		return false
	}
	for p := range r.Sources {
		if validRelPath(p) != nil {
			return false
		}
	}
	seen := map[string]bool{}
	for i := range r.Claims {
		if !r.Claims[i].valid(docLines) || seen[r.Claims[i].ID] {
			return false
		}
		seen[r.Claims[i].ID] = true
	}
	return true
}

// versionValid checks the fields each version defines: the commit from v2,
// symbols only in v3 (present, possibly empty, with well-formed entries).
func (r *Receipt) versionValid() bool {
	switch {
	case r.Version == receiptVersion && r.Commit != "":
		return false
	case r.Version >= receiptVersionCommit && !isHexCommit(r.Commit):
		return false
	case r.Version < receiptVersion || r.Version > receiptVersionSymbols:
		return false
	case (r.Version == receiptVersionSymbols) != (r.Symbols != nil):
		return false
	}
	for k, h := range r.Symbols {
		p, _, ok := splitSymbolKey(k)
		if !ok || validRelPath(p) != nil || !isHex64(h) {
			return false
		}
	}
	return true
}

// isHexCommit: a 40- or 64-hex lowercase object id (SHA-1 or SHA-256).
func isHexCommit(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
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

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// symbolsFit reports whether a v3 receipt's symbol keys are all citations of
// the document it certifies; a key the document never cites is forged.
// Keys the head no longer resolves are staleness, checked by fresh.
func (r *Receipt) symbolsFit(text string) bool {
	if len(r.Symbols) == 0 {
		return true
	}
	cited := citedSymbolKeys(r.Doc, text)
	for k := range r.Symbols {
		if !cited[k] {
			return false
		}
	}
	return true
}

// fresh reports whether the receipt's hashes are the snapshot's (R7a at
// head): the document, every source and, in v3, every cited symbol.
func (r *Receipt) fresh(s *snapshot) bool {
	if r.DocSHA256 != s.docSHA || len(r.Sources) != len(s.sources.Included) {
		return false
	}
	for _, src := range s.sources.Included {
		if r.Sources[src.Path] != src.SHA256 {
			return false
		}
	}
	if r.Version != receiptVersionSymbols {
		return true
	}
	if len(r.Symbols) != len(s.symbols) {
		return false
	}
	for k, h := range s.symbols {
		if r.Symbols[k] != h {
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
	if rc.DocSHA256 == s.docSHA && !rc.symbolsFit(s.text) {
		return nil, s, "receipt_invalid", nil
	}
	if !rc.fresh(s) {
		return rc, s, "receipt_stale", nil
	}
	return rc, s, "", nil
}

// ciEngine is T-R4: offline is a missing credential (an egm.env.pass
// variable unset, or an unusable claude credential); any other reason the
// llm engine cannot run is a broken CI setup, reported as an error.
func ciEngine(set *Settings) (offline string, err error) {
	// The claude auth variables count as a group: one of them set is enough.
	auth := false
	for _, k := range set.Env.Pass {
		switch {
		case containsStr(claudeAuthVars, k):
			auth = auth || os.Getenv(k) != ""
		case os.Getenv(k) == "":
			return k + " is not set (egm.env.pass)", nil
		}
	}
	if !auth && passesAuthVar(set.Env.Pass) {
		return strings.Join(claudeAuthVars, " or ") + " is not set (egm.env.pass)", nil
	}
	_, _, env, err := resolveEngine(&Options{Engine: EngineLLM}, set)
	var e *Error
	switch {
	case errors.As(err, &e) && e.Reason == "auth_expired":
		return err.Error(), nil
	case err != nil:
		return "", err
	}
	env.close()
	return "", nil
}

// ciRun carries the step-wide state of a CI gate.
type ciRun struct {
	o        *GateOptions
	root     string
	head     string
	offline  string // why there is no credential; "" when there is one
	runsLeft int    // egm.max_agent_runs across the whole step (T-R6)
}

func errDoc(rel string, err error) GateDoc {
	return GateDoc{Doc: rel, Verdict: "ERROR", ExitCode: ExitError, Reason: err.Error()}
}

// doc is T-R2 for one document.
func (c *ciRun) doc(ctx context.Context, rel string) GateDoc {
	gd := GateDoc{Doc: rel, Verdict: VerdictFail, ExitCode: ExitFail}
	rc, s, code, err := receiptCheck(ctx, c.root, rel, c.head, c.o.ConfigFile)
	if err != nil {
		return errDoc(rel, err)
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
		return errDoc(rel, err)
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
		return errDoc(rel, err)
	}
	c.runsLeft -= out.Report.Budget.Used
	if engine == EngineLLM {
		if gd.Reason = roundFailure(out.Report); gd.Reason != "" {
			gd.Downgrades = out.Report.Downgrades
			return gd
		}
	}
	return c.classify(ctx, o, s, rc, out.Report, engine)
}

// roundFailure is T-R2 step 3 before T-R3: a required reviewer that did not
// complete (ci_not_run, or ci_budget when the budget stopped it), or a
// downgraded round (ci_partial: ci implies --strict, and a partial round,
// e.g. max_exclusion or no_detailed_section, is not an independent review).
func roundFailure(rep *Report) string {
	if why := notRun(rep.Coverage); why != "" {
		if strings.HasSuffix(why, "(budget)") {
			return "ci_budget: " + why
		}
		return "ci_not_run: " + why
	}
	if len(rep.Downgrades) > 0 {
		return "ci_partial: " + strings.Join(rep.Downgrades, ",")
	}
	return ""
}

// notRun names a required reviewer that did not complete.
func notRun(cv Coverage) string {
	for _, r := range cv.Reviewers {
		if containsStr(cv.Required, r.Name) && r.Status != "completed" {
			return r.Name + " " + r.Status + " (" + r.Reason + ")"
		}
	}
	return ""
}

// classify is T-R3 over the fresh round's findings.
func (c *ciRun) classify(ctx context.Context, o *Options, s *snapshot, rc *Receipt, rep *Report, engine string) GateDoc {
	gd := GateDoc{Doc: rc.Doc, Verdict: VerdictPass, OK: true, ExitCode: ExitOK, CIFindings: []CIFinding{}}
	var arbitrate []string
	for i := range rep.Findings {
		cf := sortFinding(&rep.Findings[i], rc.Claims, &s.set)
		if cf.Result == "arbiter" {
			arbitrate = append(arbitrate, cf.ID)
		}
		gd.CIFindings = append(gd.CIFindings, cf)
	}
	if len(arbitrate) > 0 {
		if err := c.arbitrate(ctx, o, engine, arbitrate, gd.CIFindings); err != nil {
			return errDoc(rc.Doc, err)
		}
	}
	finalize(&gd, engine)
	return gd
}

// sortFinding is one row of T-R3: warn below major, then the matched claim.
func sortFinding(f *ReportFinding, claims []Claim, set *Settings) CIFinding {
	cf := CIFinding{ID: f.ID, Severity: f.Severity, Heading: f.Scope.Heading, Lines: f.Scope.Lines, Message: f.Message}
	if sevRank(f.Severity) < sevRank(SevMajor) {
		cf.Result = "warn"
		return cf
	}
	claim := matchClaim(f, claims, set)
	switch {
	case claim == nil:
		cf.Result = "ci_finding"
	case claim.Kind == ClaimArbiterWithdrawn && !contestable(f):
		// Local mode could never have contested it (R28): the claim is
		// made up, and CI does not re-roll the finding.
		cf.Claim, cf.Result = claim.ID, "ci_finding"
	case claim.Kind == ClaimArbiterWithdrawn:
		cf.Claim, cf.Result = claim.ID, "arbiter"
	default:
		cf.Claim, cf.Result = claim.ID, "ci_downgrade"
	}
	return cf
}

// finalize sets the document's verdict from its CI findings: the first
// failing one decides; minor findings or the offline opt-in give
// PASS_PARTIAL with warn.
func finalize(gd *GateDoc, engine string) {
	for i := range gd.CIFindings {
		cf := &gd.CIFindings[i]
		switch cf.Result {
		case "warn":
			gd.Warn = true
		case "withdrawn":
		default:
			if gd.OK {
				gd.OK, gd.Verdict, gd.ExitCode, gd.Reason = false, VerdictFail, ExitFail, cf.Result+": "+cf.ID
			}
		}
	}
	if gd.OK && (gd.Warn || engine != EngineLLM) {
		gd.Verdict, gd.Warn = VerdictPassPartial, true
		if engine != EngineLLM {
			gd.Downgrades = append(gd.Downgrades, "ci_offline")
		}
	}
}

// matchClaim is T-R3's match: same heading, both sides with lines within
// egm.match.gap, and sim ≥ egm.match.sim; owner and persona ignored; a
// finding more severe than the claim is unmatched (R20). A declined or
// forced claim wins over an arbiter_withdrawn one, so the claims' order
// cannot buy an arbiter run (fail-closed).
func matchClaim(f *ReportFinding, claims []Claim, set *Settings) *Claim {
	if len(f.Scope.Lines) != 2 {
		return nil
	}
	var arb *Claim
	for i := range claims {
		cl := &claims[i]
		if sevRank(f.Severity) > sevRank(cl.Severity) {
			continue
		}
		if !Region(f.Scope, Scope{Heading: cl.Heading, Lines: cl.Lines}, set.Match.Gap) || Sim(f.Message, cl.Message, set.Match.Stem) < set.Match.Sim {
			continue
		}
		if cl.Kind != ClaimArbiterWithdrawn {
			return cl
		}
		if arb == nil {
			arb = cl
		}
	}
	return arb
}

// contestable is R28's contest rule for a fresh CI finding: an LLM
// reviewer's finding, not comprehension. Checker and injection-scan
// findings never reach an arbiter.
func contestable(f *ReportFinding) bool {
	return !isChecker(f.Owner) && personaOf(f.Owner) != "comprehension"
}

// arbitrate runs CI's own arbiter on each CI lineage matched to an
// arbiter_withdrawn claim (T-R3), in the same throwaway state, and records
// the verdict in cfs.
func (c *ciRun) arbitrate(ctx context.Context, o *Options, engine string, ids []string, cfs []CIFinding) error {
	verdicts := map[string]string{}
	if engine == EngineLLM {
		var err error
		if verdicts, err = ciArbiter(ctx, o, ids, &c.runsLeft); err != nil {
			return err
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
	return nil
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
	defer func() { *runsLeft -= st.cycle().AgentRuns - before }()
	out := map[string]string{}
	for _, id := range ids {
		l := st.lineage(id)
		if l == nil {
			continue
		}
		// The arbiter sees the finding's own first evidence, as a local
		// contest would (R28), never a text the receipt chose.
		if len(l.Evidence) > 0 {
			e := l.Evidence[0]
			l.ContestEvidence = &e
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

func passesAuthVar(pass []string) bool {
	for _, k := range claudeAuthVars {
		if containsStr(pass, k) {
			return true
		}
	}
	return false
}
