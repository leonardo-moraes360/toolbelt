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
	// At is gate mode (R7a, R33): status over the commit's tree.
	At string
}

// Outcome is a command's result: the report (nil for a refused command), the
// exit code (R32) and a human message.
type Outcome struct {
	Report   *Report `json:"report,omitempty"`
	ExitCode int     `json:"exit_code"`
	Message  string  `json:"message,omitempty"`
	// Receipt is the repository-relative path tb egm receipt wrote (T-R1).
	Receipt string `json:"receipt,omitempty"`
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
	at          string // gate mode: commit oid; "" in interactive mode
	target      string // report target: "worktree" or the --at rev
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

// prepare runs phase 1 without touching the state (R7, R13, R34). With
// o.At it reads everything from the commit (R7a).
func prepare(ctx context.Context, o *Options) (*snapshot, error) {
	user, err := LoadUserConfig(o.ConfigFile)
	if err != nil {
		return nil, errorf(ExitError, "config", "%v", err)
	}
	var repo *Repo
	var docPath string
	if o.At != "" {
		var rel string
		if repo, rel, err = gateLocate(o.Cwd, o.Doc); err != nil {
			return nil, err
		}
		docPath = filepath.Join(repo.Root, filepath.FromSlash(rel))
	} else {
		if docPath, err = docPathIn(o.Cwd, o.Doc); err != nil {
			return nil, err
		}
		if repo, err = DiscoverRepo(docPath); err != nil {
			return nil, err
		}
	}
	s := &snapshot{repo: repo, target: "worktree"}
	var raw []byte
	var w *Warning
	if o.At != "" {
		if s.at, err = repo.RevParse(ctx, o.At); err != nil {
			return nil, err
		}
		s.target = o.At
		if raw, w, err = readRepoConfigAt(ctx, repo, s.at); err != nil {
			return nil, err
		}
	} else {
		raw, w = readRepoConfig(repo.Root)
	}
	s.set = ApplyRepoConfig(&user, raw)
	if w != nil {
		s.set.Warnings = append(s.set.Warnings, *w)
	}
	if o.At != "" {
		err = s.readDocAt(ctx, docPath)
	} else {
		err = s.readDoc(docPath)
	}
	if err != nil {
		return nil, err
	}
	if err := s.phase1(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *snapshot) setRel(docPath string) error {
	rel, err := filepath.Rel(s.repo.Root, docPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errorf(ExitError, "usage", "document %s is outside the repository root %s", docPath, s.repo.Root)
	}
	s.rel = filepath.ToSlash(rel)
	return nil
}

func (s *snapshot) readDoc(docPath string) error {
	if err := s.setRel(docPath); err != nil {
		return err
	}
	b, err := readRegular(docPath, s.set.MaxPacketBytes)
	if errors.Is(err, errTooBig) {
		return errorf(ExitError, "max_packet_bytes", "document %s is larger than egm.max_packet_bytes (%d)", s.rel, s.set.MaxPacketBytes)
	}
	if err != nil {
		return errorf(ExitError, "usage", "document %s: %v", s.rel, err)
	}
	s.setDoc(b)
	return nil
}

func (s *snapshot) setDoc(b []byte) {
	s.docSHA = sha256Hex(b)
	s.text, _ = Redact(string(b))
	s.doc = ParseDoc(s.text, s.set.DetailHeadings)
	s.hasDetailed = s.doc.HasDetailed()
	s.docType = DocType(s.rel, s.set.DocTypes)
}

func (s *snapshot) phase1(ctx context.Context) error {
	var err error
	var idx indexView = worktreeIndex{s.repo}
	if s.at != "" {
		idx = newTreeIndex(s.repo, s.at)
		s.sources, err = CollectSourcesAt(ctx, s.repo, &s.set, s.at, s.rel, s.text)
	} else {
		s.sources, err = CollectSources(ctx, s.repo, &s.set, s.rel, s.text)
	}
	if err != nil {
		return err
	}
	s.checks, err = RunCheckers(ctx, s.doc, s.rel, s.docType, &s.set, idx, nil)
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
	if s.head = s.at; s.head == "" {
		s.head, _ = s.repo.RevParse(ctx, "HEAD")
	}
	s.warnings = append(append(append([]Warning(nil), s.set.Warnings...), s.repo.Warnings...), s.sources.Warnings...)
	return nil
}

// resolveEngine applies R12 and opens the llm engine when it runs.
func resolveEngine(o *Options, set *Settings) (engine, reason string, env *llmEnv, err error) {
	switch o.Engine {
	case "", EngineAuto:
		if len(set.AgentCmd) == 0 {
			return EngineDeterministic, "agent_cmd ausente na config de usuário (agent_cmd missing from user config)", nil, nil
		}
		env, fallback, err := openLLM(set)
		if fallback {
			return EngineDeterministic, "llm engine unavailable: " + err.Error(), nil, nil
		}
		if err != nil {
			return "", "", nil, err
		}
		return EngineLLM, "", env, nil
	case EngineDeterministic:
		return EngineDeterministic, "", nil, nil
	case EngineLLM:
		if len(set.AgentCmd) == 0 {
			return "", "", nil, errorf(ExitError, "usage", "--engine llm needs egm.agent_cmd in the user config")
		}
		env, _, err := openLLM(set)
		if err != nil {
			return "", "", nil, err
		}
		return EngineLLM, "", env, nil
	}
	return "", "", nil, errorf(ExitError, "usage", "--engine must be auto, llm or deterministic")
}

func (s *snapshot) report(st *State, rd *Round, engine, reason string) *Report {
	verdict, downs := Verdict(st, s.content, s.policy)
	r := &Report{
		Slug: Slug(s.rel), Doc: s.rel, DocType: s.docType, Verdict: verdict, Content: s.content, Target: s.target,
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
	if err := checkReviewers(o.Reviewers); err != nil {
		return nil, err
	}
	engine, reason, env, err := resolveEngine(o, &s.set)
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
	if err != nil {
		return nil, err
	}
	rc := &roundCtx{s: s, o: o, st: st, engine: engine, reason: reason, env: env, isReview: isReview}
	return rc.run(ctx)
}

func checkReviewers(names []string) error {
	for _, n := range names {
		if _, ok := presetSpec(n); !ok {
			return errorf(ExitError, "usage", "unknown reviewer %q", n)
		}
	}
	return nil
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
func stuck(st *State, arbiter bool) bool {
	blocked := false
	for _, l := range st.Lineages {
		if l.Status == StFixed && !l.Confirmed {
			return false
		}
		if l.Status == StContestPending && arbiter && l.ArbiterNotRun < 2 {
			continue // an eligible arbiter can still change it
		}
		if l.blocking() {
			blocked = true
		}
	}
	return blocked
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

// Status runs `tb egm status` over the worktree, or over the commit with
// o.At (R7a, R25, R32). It reads the
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
