package egm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// llmEnv is the llm engine for one command: the resolved profile and the
// shared runner (one Registry and subreaper, R3).
type llmEnv struct {
	prof    *Profile
	runner  *Runner
	argvSHA string
}

func (e *llmEnv) close() {
	if e != nil && e.runner != nil {
		e.runner.Close()
	}
}

// openLLM resolves the profile and the runner. fallback is true when auto
// may fall back to deterministic: the llm engine cannot run here (R12).
func openLLM(set *Settings) (env *llmEnv, fallback bool, err error) {
	prof, err := ResolveProfile(set)
	if err != nil {
		return nil, false, err
	}
	runner, err := NewRunner(set, prof)
	if err != nil {
		var e *Error
		if asError(err, &e) && (e.Reason == "no_landlock" || e.Reason == "no_subreaper") {
			return nil, true, err
		}
		return nil, false, err
	}
	return &llmEnv{prof: prof, runner: runner, argvSHA: ArgvSHA(prof)}, false, nil
}

func newNonce() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return hex.EncodeToString(b[:])
}

// reviewerRange is the reviewer's range in the snapshot (R6, R10).
func (s *snapshot) reviewerRange(r ReviewerSpec) [2]int {
	switch r.Range {
	case RangeFeature:
		return s.doc.Feature
	case RangeDetailed:
		return s.doc.Detailed
	}
	return [2]int{1, len(s.doc.Lines)}
}

// planReviewersLLM gives each preset reviewer its initial status; "pending"
// ones run in phase 2.
func (s *snapshot) planReviewersLLM(only []string) []ReviewerRun {
	var out []ReviewerRun
	for _, r := range Preset {
		switch {
		case r.Range == RangeDetailed && !s.hasDetailed && s.docType == "adr":
			continue
		case r.Range == RangeDetailed && !s.hasDetailed:
			out = append(out, ReviewerRun{Name: r.Name, Status: "skipped", Reason: "no_detailed_section"})
		case len(only) > 0 && !containsStr(only, r.Name):
			out = append(out, ReviewerRun{Name: r.Name, Status: "skipped", Reason: "not_selected"})
		default:
			if _, _, skip := AttestRequirement(s.doc, s.reviewerRange(r), s.set.Attest.MinQuotes); skip {
				out = append(out, ReviewerRun{Name: r.Name, Status: "skipped", Reason: "range_too_small"})
				continue
			}
			out = append(out, ReviewerRun{Name: r.Name, Status: "pending"})
		}
	}
	return out
}

var removedPattern = "[PADRÃO-REMOVIDO]"

// scrubPrior replaces injection matches in a PRIOR text (R17a): a match in
// PRIOR opens no lineage, it is removed and reported.
func scrubPrior(text string) (string, bool) {
	hit := false
	for _, re := range injectionPatterns {
		if re.MatchString(text) {
			hit = true
			text = re.ReplaceAllString(text, removedPattern)
		}
	}
	return text, hit
}

// priorFor is R9's PRIOR set for a reviewer: lineages it owns that are open,
// reopened, unconfirmed fixed or fixed confirmed in the current cycle.
func priorFor(st *State, reviewer string, cycleStart int) (items []PriorItem, injected bool) {
	for _, l := range st.Lineages {
		if l.Owner != reviewer {
			continue
		}
		in := l.Status == StOpen || l.Status == StReopened ||
			(l.Status == StFixed && (!l.Confirmed || l.ConfirmedRound >= cycleStart))
		if !in {
			continue
		}
		msg, h1 := scrubPrior(l.Message)
		rat, h2 := scrubPrior(l.Rationale)
		injected = injected || h1 || h2
		items = append(items, PriorItem{Lineage: l.ID, Severity: l.Severity, Status: l.Status, Scope: l.Scope,
			Message: msg, Rationale: rat, Evidence: l.Evidence})
	}
	return items, injected
}

// job is one reviewer's phase-2 work.
type job struct {
	spec   ReviewerSpec
	run    *ReviewerRun
	owner  string
	packet string
	prompt string
	vin    *ValidateInput
	out    *ReviewerOutput
	tools  bool
}

func (s *snapshot) blocksFor(r ReviewerSpec) (blocks []Block, docs map[string]*Doc) {
	blocks = []Block{{Path: s.rel, Text: s.text}}
	docs = map[string]*Doc{s.rel: s.doc}
	if r.CodeSources {
		for _, src := range s.sources.Included {
			blocks = append(blocks, Block{Path: src.Path, Text: src.Text})
			docs[src.Path] = ParseDoc(src.Text, nil)
		}
	}
	return blocks, docs
}

// prepareJob writes the reviewer's packet (R1) and builds its prompt (R9).
func (s *snapshot) prepareJob(st *State, rd *Round, run *ReviewerRun, cycleStart int) (j *job, ws []Warning, err error) {
	spec, _ := presetSpec(run.Name)
	persona, err := LoadPersona(spec.Persona)
	if err != nil {
		return nil, nil, err
	}
	var standard string
	if name := s.standardName(spec); name != "" {
		if standard, err = LoadStandard(name); err != nil {
			return nil, nil, err
		}
	}
	nonce := newNonce()
	prior, injected := priorFor(st, run.Name, cycleStart)
	if injected {
		ws = append(ws, Warning{Code: "prior_injection", Message: run.Name + ": an injection pattern was removed from PRIOR"})
	}
	blocks, docs := s.blocksFor(spec)
	rng := s.reviewerRange(spec)
	prompt := BuildPrompt(&PromptInput{Reviewer: run.Name, Persona: persona, Standard: standard, Range: rng, Nonce: nonce, Prior: prior, Blocks: blocks, MinQuotes: s.set.Attest.MinQuotes})
	ids := make([]string, 0, len(prior))
	for i := range prior {
		ids = append(ids, prior[i].Lineage)
	}
	j = &job{spec: spec, run: run, owner: fmt.Sprintf("egm:%s:r%d:%s", Slug(s.rel), rd.N, run.Name), prompt: prompt,
		vin: &ValidateInput{Reviewer: run.Name, Nonce: nonce, MaxSeverity: persona.MaxSeverity, PriorIDs: ids, Doc: s.doc,
			DocPath: s.rel, Range: rng, MinQuotes: s.set.Attest.MinQuotes, Sources: docs}}
	j.packet = filepath.Join(s.stateDir, fmt.Sprintf("round-%d", rd.N), "packets", run.Name)
	return j, ws, s.writePacket(j, blocks, persona.Name, standard != "", nonce, rd.ArgvSHA256)
}

func (s *snapshot) standardName(r ReviewerSpec) string {
	if r.Standard == "structural" {
		return structuralStandard(s.docType)
	}
	return r.Standard
}

func (s *snapshot) writePacket(j *job, blocks []Block, persona string, hasStandard bool, nonce, argvSHA string) error {
	if err := writeAtomic(j.packet, "prompt.txt", []byte(j.prompt)); err != nil {
		return err
	}
	type srcMeta struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
		Lines  [2]int `json:"lines"`
	}
	var srcs []srcMeta
	for i := range blocks {
		b := &blocks[i]
		srcs = append(srcs, srcMeta{Path: b.Path, SHA256: sha256Hex([]byte(b.Text)), Lines: [2]int{1, strings.Count(b.Text, "\n") + 1}})
	}
	sb, _ := json.MarshalIndent(srcs, "", "  ")
	if err := writeAtomic(j.packet, "sources.json", append(sb, '\n')); err != nil {
		return err
	}
	meta := map[string]any{"persona": persona, "standard": hasStandard, "range": j.vin.Range, "nonce": nonce, "argv_sha256": argvSHA}
	mb, _ := json.MarshalIndent(meta, "", "  ")
	return writeAtomic(j.packet, "meta.json", append(mb, '\n'))
}

// noRetry reasons are not retried (R16).
func noRetry(reason string) bool {
	switch reason {
	case "tools_used", "escaped_process", "unattributed_orphan", "budget", "interrupted":
		return true
	}
	return false
}

// budgetGate counts agent runs for the cycle (R27).
type budgetGate struct {
	mu   sync.Mutex
	cy   *Cycle
	max  int
	runs int
}

func (b *budgetGate) take() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cy.AgentRuns >= b.max {
		return false
	}
	b.cy.AgentRuns++
	b.runs++
	return true
}

// runJob runs one reviewer with one retry (R16).
func (e *llmEnv) runJob(ctx context.Context, j *job, bud *budgetGate) {
	reason := "missing_output"
	for attempt := 1; attempt <= 2; attempt++ {
		if !bud.take() {
			reason = "budget"
			break
		}
		res := e.runner.Run(ctx, &GoldfishRequest{Owner: j.owner, Attempt: attempt, Packet: j.packet, Prompt: j.prompt})
		j.tools = j.tools || res.ToolsUsed
		if !res.Completed {
			reason = res.Reason
			if noRetry(reason) {
				break
			}
			continue
		}
		out, why := ParseReviewerOutput(res.Payload, j.vin)
		if why == "" {
			j.out = out
			j.run.Status, j.run.Reason = "completed", ""
			return
		}
		reason = why
	}
	j.run.Status, j.run.Reason = "not_run", reason
}

// runPhase2 runs the pending reviewers in parallel up to max_parallel and
// applies R3's orphan attribution at the end.
func (s *snapshot) runPhase2(ctx context.Context, e *llmEnv, jobs []*job, bud *budgetGate) {
	e.runner.ResetUnattributed()
	sem := make(chan struct{}, s.set.MaxParallel)
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j *job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			e.runJob(ctx, j, bud)
		}(j)
	}
	wg.Wait()
	unattributed := e.runner.Unattributed()
	for _, j := range jobs {
		switch {
		case unattributed && j.run.Status == "completed":
			j.run.Status, j.run.Reason, j.out = "not_run", "unattributed_orphan", nil
		case e.runner.Escaped(j.owner):
			j.run.Status, j.run.Reason, j.out = "not_run", "escaped_process", nil
		}
	}
}

// llmPhase prepares and runs the reviewers of round rd. It returns the
// findings of the completed reviewers and their PRIOR answers.
func (s *snapshot) llmPhase(ctx context.Context, e *llmEnv, st *State, rd *Round) ([]*Finding, map[string][]PriorAnswer, error) {
	cy := st.cycle()
	var jobs []*job
	for i := range rd.Reviewers {
		if rd.Reviewers[i].Status != "pending" {
			continue
		}
		j, ws, err := s.prepareJob(st, rd, &rd.Reviewers[i], cy.FirstRound)
		if err != nil {
			return nil, nil, err
		}
		rd.Warnings = append(rd.Warnings, ws...)
		jobs = append(jobs, j)
	}
	s.runPhase2(ctx, e, jobs, &budgetGate{cy: cy, max: s.set.MaxAgentRuns})
	if ctx.Err() != nil {
		// Interrupted (SIGINT/SIGTERM): nothing of this round is kept.
		return nil, nil, errorf(ExitError, "interrupted", "interrupted during phase 2; the round was not recorded")
	}
	var findings []*Finding
	answers := map[string][]PriorAnswer{}
	for _, j := range jobs {
		if j.tools {
			rd.Warnings = append(rd.Warnings, Warning{Code: "tools_used", Message: j.run.Name})
		}
		if j.out == nil {
			continue
		}
		answers[j.run.Name] = j.out.Prior
		for i := range j.out.Findings {
			f := j.out.Findings[i]
			f.Reviewer = j.run.Name
			findings = append(findings, &f)
		}
	}
	sort.SliceStable(findings, func(a, b int) bool { return ownerRank(findings[a].Reviewer) < ownerRank(findings[b].Reviewer) })
	return findings, answers, nil
}

// worstCase is R27's worst case for the next round.
func worstCase(reviewers []ReviewerRun, st *State) int {
	n := 0
	for _, r := range reviewers {
		if r.Status == "pending" {
			n += 2
		}
	}
	if st != nil {
		for _, l := range st.Lineages {
			if l.Status == StContestPending {
				n += 2
			}
		}
	}
	return n
}

var reArbiterTerm = regexp.MustCompile(`(?i)\b(arbiter|árbitro|withdraw|uphold|verdict|veredito)\b`)

func asError(err error, target **Error) bool {
	return errors.As(err, target)
}

// reviewersAndPrior runs phase 2 and applies the owners' PRIOR answers
// before the scopes are remapped: the fix test reads the snapshots the
// scopes are numbered in (R20).
func (rc *roundCtx) reviewersAndPrior(ctx context.Context, rd *Round, r *reconciler) error {
	if rc.env == nil {
		return nil
	}
	rd.ArgvSHA256 = rc.env.argvSHA
	findings, answers, err := rc.s.llmPhase(ctx, rc.env, rc.st, rd)
	if err != nil {
		return err
	}
	for i := range rd.Reviewers {
		if rd.Reviewers[i].Status == "completed" {
			r.ran[rd.Reviewers[i].Name] = true
		}
	}
	counter := map[string]int{}
	for _, f := range findings {
		counter[f.Reviewer]++
		f.Occurrence = fmt.Sprintf("%s-r%d-%d", f.Reviewer, rd.N, counter[f.Reviewer])
	}
	rc.llmFindings = findings
	return r.applyPrior(answers, rc.fixCheck(ctx))
}

func (rc *roundCtx) fixCheck(ctx context.Context) fixCheck {
	return func(l *Lineage) (touched, changed bool, err error) {
		opened := rc.st.roundN(l.OpenedRound)
		changed = opened == nil || opened.Content != rc.s.content
		if !changed {
			return false, false, nil
		}
		touched, err = rc.s.touchedSince(ctx, l, l.OpenedRound)
		return touched, true, err
	}
}
