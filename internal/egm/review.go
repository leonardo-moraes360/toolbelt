package egm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Engines (R12).
const (
	EngineAuto          = "auto"
	EngineLLM           = "llm"
	EngineDeterministic = "deterministic"
)

// Options are the inputs shared by the tb egm commands.
type Options struct {
	Doc        string
	Cwd        string // absolute; relative Doc paths resolve against it
	ConfigFile string // -c/--config
	Engine     string
	Reviewers  []string
	DryRun     bool
	Restart    bool
	Strict     bool
}

// Outcome is a command's result: the report (nil for a refused command), the
// exit code (R32) and a human message.
type Outcome struct {
	Report   *Report `json:"report,omitempty"`
	ExitCode int     `json:"exit_code"`
	Message  string  `json:"message,omitempty"`
}

// snapshot is everything phase 1 computes for the target content.
type snapshot struct {
	repo        *Repo
	set         Settings
	rel         string
	docType     string
	doc         *Doc
	text        string // redacted document
	docSHA      string
	sources     *SourceSet
	checks      *CheckResult
	content     string
	policy      string
	stateDir    string
	head        string
	hasDetailed bool
	warnings    []Warning
}

func docPathIn(cwd, doc string) (string, error) {
	p := doc
	if !filepath.IsAbs(p) {
		if !filepath.IsAbs(cwd) {
			return "", errorf(ExitError, "usage", "cwd must be absolute")
		}
		p = filepath.Join(cwd, p)
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", errorf(ExitError, "usage", "document %s: %v", doc, err)
	}
	return resolved, nil
}

// prepare runs phase 1 without touching the state (R7, R13, R34).
func prepare(ctx context.Context, o *Options) (*snapshot, error) {
	user, err := LoadUserConfig(o.ConfigFile)
	if err != nil {
		return nil, errorf(ExitError, "config", "%v", err)
	}
	docPath, err := docPathIn(o.Cwd, o.Doc)
	if err != nil {
		return nil, err
	}
	repo, err := DiscoverRepo(docPath)
	if err != nil {
		return nil, err
	}
	s := &snapshot{repo: repo}
	raw, w := readRepoConfig(repo.Root)
	s.set = ApplyRepoConfig(&user, raw)
	if w != nil {
		s.set.Warnings = append(s.set.Warnings, *w)
	}
	if err := s.readDoc(docPath); err != nil {
		return nil, err
	}
	if err := s.phase1(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *snapshot) readDoc(docPath string) error {
	rel, err := filepath.Rel(s.repo.Root, docPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errorf(ExitError, "usage", "document %s is outside the repository root %s", docPath, s.repo.Root)
	}
	s.rel = filepath.ToSlash(rel)
	b, err := readRegular(docPath, s.set.MaxPacketBytes)
	if errors.Is(err, errTooBig) {
		return errorf(ExitError, "max_packet_bytes", "document %s is larger than egm.max_packet_bytes (%d)", s.rel, s.set.MaxPacketBytes)
	}
	if err != nil {
		return errorf(ExitError, "usage", "document %s: %v", s.rel, err)
	}
	s.docSHA = sha256Hex(b)
	s.text, _ = Redact(string(b))
	s.doc = ParseDoc(s.text, s.set.DetailHeadings)
	s.hasDetailed = s.doc.HasDetailed()
	s.docType = DocType(s.rel, s.set.DocTypes)
	return nil
}

func (s *snapshot) phase1(ctx context.Context) error {
	var err error
	s.sources, err = CollectSources(ctx, s.repo, &s.set, s.rel, s.text)
	if err != nil {
		return err
	}
	s.checks, err = RunCheckers(ctx, s.doc, s.rel, s.docType, s.repo.Root, &s.set, worktreeIndex{s.repo}, nil)
	if err != nil {
		return err
	}
	pairs := []string{s.rel + "\t" + s.docSHA}
	for _, src := range s.sources.Included {
		pairs = append(pairs, src.Path+"\t"+src.SHA256)
	}
	sort.Strings(pairs)
	pairs = append(pairs, "checks\t"+s.checks.checksSHA())
	s.content = sha256Hex([]byte(strings.Join(pairs, "\n")))
	s.policy = PolicySHA(&s.set, s.hasDetailed)
	if s.stateDir, err = StateDir(s.repo.ID, Slug(s.rel)); err != nil {
		return err
	}
	s.head, _ = s.repo.RevParse(ctx, "HEAD")
	s.warnings = append(append(append([]Warning(nil), s.set.Warnings...), s.repo.Warnings...), s.sources.Warnings...)
	return nil
}

// resolveEngine applies R12. Phase A slice 1 has no llm engine yet.
func resolveEngine(o *Options, set *Settings) (engine, reason string, err error) {
	switch o.Engine {
	case "", EngineAuto:
		if len(set.AgentCmd) == 0 {
			return EngineDeterministic, "agent_cmd ausente na config de usuário (agent_cmd missing from user config)", nil
		}
		return EngineDeterministic, "llm engine not available in this build", nil
	case EngineDeterministic:
		return EngineDeterministic, "", nil
	case EngineLLM:
		if len(set.AgentCmd) == 0 {
			return "", "", errorf(ExitError, "usage", "--engine llm needs egm.agent_cmd in the user config")
		}
		return "", "", errorf(ExitError, "engine_unavailable", "the llm engine is not available in this build")
	}
	return "", "", errorf(ExitError, "usage", "--engine must be auto, llm or deterministic")
}

func (s *snapshot) report(st *State, rd *Round, engine, reason string) *Report {
	verdict, downs := Verdict(st, s.content, s.policy)
	r := &Report{
		Slug: Slug(s.rel), Doc: s.rel, DocType: s.docType, Verdict: verdict, Content: s.content, Target: "worktree",
		Engine: engine, EngineReason: reason, Findings: reportFindings(st), Coverage: coverageOf(rd),
		SkippedSources: append([]SkippedSource{}, s.sources.Skipped...), Exemptions: append([]Exemption{}, s.checks.Exemptions...),
		Warnings: sortWarnings(s.warnings), Downgrades: append([]string{}, downs...), LoopState: "running",
	}
	if rd != nil {
		r.Round, r.Cycle = rd.N, rd.Cycle
		r.Warnings = sortWarnings(append(r.Warnings, rd.Warnings...))
	}
	if st != nil {
		r.LoopState = LoopState(st)
		if cy := st.cycle(); cy != nil {
			r.Budget = Budget{MaxAgentRuns: s.set.MaxAgentRuns, Used: cy.AgentRuns, Remaining: s.set.MaxAgentRuns - cy.AgentRuns}
		}
	}
	if r.Budget.MaxAgentRuns == 0 {
		r.Budget = Budget{MaxAgentRuns: s.set.MaxAgentRuns, Remaining: s.set.MaxAgentRuns}
	}
	return r
}

func exitFor(verdict string) int {
	if verdict == VerdictFail {
		return ExitFail
	}
	return ExitOK
}

// Review runs `tb egm review`: round 1 of the first cycle, or a new cycle
// with --restart (R31).
func Review(ctx context.Context, o *Options) (*Outcome, error) {
	return runRound(ctx, o, true)
}

// LoopNext runs `tb egm loop next` (R26).
func LoopNext(ctx context.Context, o *Options) (*Outcome, error) {
	c := *o
	c.Restart = false
	return runRound(ctx, &c, false)
}

func runRound(ctx context.Context, o *Options, isReview bool) (*Outcome, error) {
	s, err := prepare(ctx, o)
	if err != nil {
		return nil, err
	}
	engine, reason, err := resolveEngine(o, &s.set)
	if err != nil {
		return nil, err
	}
	if err := checkReviewers(o.Reviewers); err != nil {
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
	if out, err := s.gate(st, o, isReview, engine); out != nil || err != nil {
		return out, err
	}
	if st == nil {
		st = newState(s.rel, Slug(s.rel))
	}
	if o.DryRun {
		return &Outcome{Report: s.report(st, st.lastRound(), engine, reason), Message: "dry run: worst case 0 agent runs (deterministic engine)"}, nil
	}
	rd, err := s.round(ctx, st, o, engine, reason)
	if err != nil {
		return nil, err
	}
	rep := s.report(st, rd, engine, reason)
	if err := s.persist(st, rd, rep); err != nil {
		return nil, err
	}
	return &Outcome{Report: rep, ExitCode: exitFor(rep.Verdict)}, nil
}

func checkReviewers(names []string) error {
	for _, n := range names {
		if _, ok := presetSpec(n); !ok {
			return errorf(ExitError, "usage", "unknown reviewer %q", n)
		}
	}
	return nil
}

// gate refuses a round per R23 (cap), R26 (no change), R31 (review vs loop
// next, --restart) before anything is written.
func (s *snapshot) gate(st *State, o *Options, isReview bool, engine string) (*Outcome, error) {
	if done, err := s.gateEntry(st, o, isReview); done || err != nil {
		return nil, err
	}
	last := st.lastRound()
	if cy := st.cycle(); cy != nil && cy.Capped {
		return &Outcome{Report: s.report(st, last, engine, ""), ExitCode: ExitFail,
			Message: fmt.Sprintf("cycle %d is capped at %d rounds: edit and run `tb egm review --restart`", cy.N, s.set.MaxRounds)}, nil
	}
	if err := ownersAvailable(st, engine); err != nil {
		return nil, err
	}
	if last != nil && last.Content == s.content && stuck(st) {
		return nil, errorf(ExitError, "no_change", "documento e fontes não mudaram desde a rodada %d; há findings bloqueantes abertos (document and sources have not changed since round %d; there are open blocking findings)", last.N, last.N)
	}
	return nil, nil
}

// gateEntry checks review vs loop next and --restart; done means the round
// may open without the R23/R26 checks (a new state or a restart).
func (s *snapshot) gateEntry(st *State, o *Options, isReview bool) (done bool, err error) {
	switch {
	case isReview && st != nil && !o.Restart:
		return true, errorf(ExitError, "state_exists", "%s already has state: use `tb egm loop next` or `tb egm review --restart`", s.rel)
	case !isReview && st == nil:
		return true, errorf(ExitError, "no_state", "%s has no state: run `tb egm review` first", s.rel)
	case st == nil:
		return true, nil
	}
	if !o.Restart {
		return false, nil
	}
	if last := st.lastRound(); last != nil && last.Content == s.content {
		return true, errorf(ExitError, "restart_needs_change", "--restart needs the document or its sources to change since round %d", last.N)
	}
	return true, nil
}

// stuck is R26: a blocking lineage that cannot change without an edit or a
// resolve, and no unconfirmed fixed lineage.
func stuck(st *State) bool {
	blocked := false
	for _, l := range st.Lineages {
		if l.Status == StFixed && !l.Confirmed {
			return false
		}
		if l.blocking() {
			blocked = true
		}
	}
	return blocked
}

// ownersAvailable is R26's owner_unavailable: a blocking lineage whose owner
// cannot run in the effective engine.
func ownersAvailable(st *State, engine string) error {
	for _, l := range st.Lineages {
		if l.blocking() && !isChecker(l.Owner) && engine == EngineDeterministic {
			return errorf(ExitError, "owner_unavailable", "%s is owned by %s, which does not run in the %s engine", l.ID, l.Owner, engine)
		}
	}
	return nil
}

func (s *snapshot) round(ctx context.Context, st *State, o *Options, engine, reason string) (*Round, error) {
	n := len(st.Rounds) + 1
	if cy := st.cycle(); cy == nil || cy.Converged || o.Restart {
		st.Cycles = append(st.Cycles, Cycle{N: len(st.Cycles) + 1, FirstRound: n, Restart: o.Restart})
	}
	rd := Round{
		N: n, Cycle: st.cycle().N, Content: s.content, PolicySHA: s.policy, DocSHA256: s.docSHA, HeadCommit: s.head,
		Sources: map[string]string{}, Engine: engine, EngineReason: reason, Required: RequiredReviewers(s.hasDetailed),
		Reviewers: planReviewers(s.docType, s.hasDetailed, o.Reviewers),
	}
	for _, src := range s.sources.Included {
		rd.Sources[src.Path] = src.SHA256
	}
	mapper, err := s.mapper(ctx, st)
	if err != nil {
		return nil, err
	}
	r := &reconciler{st: st, round: n, mapper: mapper, docRel: s.rel, ran: map[string]bool{
		checkRefs: true, checkSections: true, checkPlan: true, injectionChecker: true,
	}}
	r.fold(s.checks.Findings)
	rd.Warnings = r.warnings
	rd.Clean = roundClean(st, &rd, s.sources.Unstaged)
	rd.Downgrades = roundDowngrades(&rd, s.docType, s.hasDetailed, s.sources.MaxExcluded && codeReviewerRan(&rd))
	st.Rounds = append(st.Rounds, rd)
	advanceCycle(st, &s.set)
	return st.lastRound(), nil
}

// mapper maps each lineage's scope from its round's snapshot to the current
// one (R20), diffing each old snapshot once.
func (s *snapshot) mapper(ctx context.Context, st *State) (func(*Lineage) *LineMap, error) {
	maps := map[int]*LineMap{}
	for _, l := range st.Lineages {
		if _, ok := maps[l.ScopeRound]; ok {
			continue
		}
		old, err := os.ReadFile(filepath.Join(s.stateDir, fmt.Sprintf("round-%d", l.ScopeRound), "target.md"))
		if err != nil {
			return nil, fmt.Errorf("snapshot of round %d: %w", l.ScopeRound, err)
		}
		lm, err := DiffSnapshots(ctx, s.stateDir, string(old), s.text)
		if err != nil {
			return nil, err
		}
		maps[l.ScopeRound] = lm
	}
	return func(l *Lineage) *LineMap { return maps[l.ScopeRound] }, nil
}

// writeSnapshot writes target.md and sources/ under dir (R1).
func (s *snapshot) writeSnapshot(dir string) error {
	if err := writeAtomic(dir, "target.md", []byte(s.text)); err != nil {
		return err
	}
	for _, src := range s.sources.Included {
		p := filepath.Join(dir, "sources", filepath.FromSlash(src.Path))
		if err := writeAtomic(filepath.Dir(p), filepath.Base(p), []byte(src.Text)); err != nil {
			return err
		}
	}
	return nil
}

func (s *snapshot) persist(st *State, rd *Round, rep *Report) error {
	dir := filepath.Join(s.stateDir, fmt.Sprintf("round-%d", rd.N))
	if err := s.writeSnapshot(dir); err != nil {
		return err
	}
	if err := writeAtomic(dir, "report.json", rep.JSON()); err != nil {
		return err
	}
	if err := writeAtomic(dir, "report.md", []byte(rep.Markdown())); err != nil {
		return err
	}
	return saveState(s.stateDir, st)
}

// Status runs `tb egm status` over the worktree (R25, R32). It reads the
// state without the lock: writes are atomic (R21).
func Status(ctx context.Context, o *Options) (*Outcome, error) {
	s, err := prepare(ctx, o)
	if err != nil {
		return nil, err
	}
	st, err := loadState(s.stateDir)
	if err != nil {
		return nil, err
	}
	var last *Round
	engine := ""
	if st != nil {
		last = st.lastRound()
		if last != nil {
			engine = last.Engine
		}
	}
	rep := s.report(st, last, engine, "")
	return &Outcome{Report: rep, ExitCode: statusExit(rep.Verdict, o.Strict || s.set.RequireFullCoverage)}, nil
}

func statusExit(verdict string, strict bool) int {
	switch verdict {
	case VerdictPass:
		return ExitOK
	case VerdictPassPartial:
		if strict {
			return ExitFail
		}
		return ExitOK
	case VerdictFail:
		return ExitFail
	}
	return ExitNotPassing
}
