package stack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/git"
	"github.com/leonardo-moraes360/toolbelt/internal/graph"
	"github.com/leonardo-moraes360/toolbelt/internal/guard"
	"github.com/leonardo-moraes360/toolbelt/internal/mem"
	"github.com/leonardo-moraes360/toolbelt/internal/review"
	"github.com/leonardo-moraes360/toolbelt/internal/verify"
)

type Op string

const (
	OpVerify Op = "verify"
	OpReview Op = "review"
	OpDrift  Op = "drift"
	OpGuard  Op = "guard"
)

var knownOps = map[string]Op{
	"verify": OpVerify,
	"review": OpReview,
	"drift":  OpDrift,
	"guard":  OpGuard,
}

func ParseOp(name string) (Op, error) {
	op, ok := knownOps[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return "", fmt.Errorf("operação desconhecida: %q (use verify, review, drift ou guard)", name)
	}
	return op, nil
}

func ParseOps(names []string) ([]Op, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("informe ao menos uma operação (verify, review, drift, guard)")
	}
	ops := make([]Op, 0, len(names))
	for _, name := range names {
		op, err := ParseOp(name)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

type Options struct {
	RepoDir       string
	Base          string
	Head          string
	Ops           []Op
	FailFast      bool
	Ticket        string
	ApprovedSHA   string
	WarnOnly      bool
	TestCmd       string
	HiddenTestCmd string
	BuildCmd      string
	LintCmd       string
	BuildCmdSet   bool
	LintCmdSet    bool
	NoBuild       bool
	NoLint        bool
	DryRun        bool
	MaxIter       int
	Timeout       time.Duration
	Reviewers     int
	Standards     []string
	LoadGraph     bool
	Loaders       *Loaders
	EnvAllow      []string
	EnvPass       []string
}

type Loaders struct {
	UnifiedDiff  func(repo, base, head string) (string, error)
	NameStatus   func(repo, base, head string) (string, error)
	ChangedFiles func(repo, base, head string) ([]string, error)
	Invariants   func(repo string) ([]mem.Invariant, error)
	GraphIndex   func(repo string) (*graph.Index, error)
}

type Shared struct {
	RepoDir    string
	Base       string
	Head       string
	FileConfig *config.Config

	Diff       string
	NameStatus string
	Changed    []string
	Invariants []mem.Invariant
	GraphIndex *graph.Index

	DiffLoads       int
	NameStatusLoads int
	IndexLoads      int
	MemLoads        int
	GraphLoads      int
}

type StepResult struct {
	Op         string  `json:"op"`
	OK         bool    `json:"ok"`
	ExitCode   int     `json:"exit_code"`
	DurationMs float64 `json:"duration_ms"`
	Reason     string  `json:"reason,omitempty"`
	Error      string  `json:"error,omitempty"`
	Result     any     `json:"result"`
}

type Result struct {
	OK              bool         `json:"ok"`
	Steps           []StepResult `json:"steps"`
	TotalDurationMs float64      `json:"total_duration_ms"`
}

func durationMillis(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func failedStep(op Op, err error) StepResult {
	msg := "erro desconhecido"
	if err != nil {
		msg = err.Error()
	}
	return StepResult{
		Op:       string(op),
		OK:       false,
		ExitCode: 1,
		Reason:   msg,
		Error:    msg,
	}
}

func Run(ctx context.Context, opts *Options) (*Result, error) {
	if opts == nil {
		return nil, fmt.Errorf("options ausentes")
	}
	if len(opts.Ops) == 0 {
		return nil, fmt.Errorf("informe ao menos uma operação")
	}
	for _, op := range opts.Ops {
		if _, err := ParseOp(string(op)); err != nil {
			return nil, err
		}
	}

	out := &Result{OK: true, Steps: make([]StepResult, 0, len(opts.Ops))}
	start := time.Now()

	shared, err := Prepare(opts)
	if err != nil {
		step := failedStep(opts.Ops[0], err)
		step.DurationMs = durationMillis(time.Since(start))
		out.Steps = append(out.Steps, step)
		out.OK = false
		out.TotalDurationMs = step.DurationMs
		return out, nil
	}

	for _, op := range opts.Ops {
		stepStart := time.Now()
		step, runErr := runOp(ctx, op, opts, shared)
		if runErr != nil {
			step = failedStep(op, runErr)
		}
		step.DurationMs = durationMillis(time.Since(stepStart))
		annotateStepReason(&step)
		out.Steps = append(out.Steps, step)
		if !step.OK {
			out.OK = false
			if opts.FailFast {
				break
			}
		}
	}
	out.TotalDurationMs = durationMillis(time.Since(start))
	return out, nil
}

func Prepare(opts *Options) (*Shared, error) {
	repo := opts.RepoDir
	if repo == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		repo = cwd
	}
	base := opts.Base
	if base == "" {
		base = "HEAD~1"
	}
	head := opts.Head
	if head == "" {
		head = "HEAD"
	}
	base = resolveEffectiveBase(repo, base, opts.Ticket)

	loaders := opts.Loaders
	if loaders == nil {
		loaders = defaultLoaders()
	}

	shared := &Shared{
		RepoDir: repo,
		Base:    base,
		Head:    head,
	}
	if fileCfg, err := config.Load(); err == nil {
		shared.FileConfig = fileCfg
	}

	diff, err := loaders.UnifiedDiff(repo, base, head)
	if err != nil {
		return nil, err
	}
	shared.Diff = diff
	shared.DiffLoads++

	nameStatus, err := loaders.NameStatus(repo, base, head)
	if err != nil {
		return nil, err
	}
	shared.NameStatus = nameStatus
	shared.NameStatusLoads++

	changed, err := loaders.ChangedFiles(repo, base, head)
	if err != nil {
		return nil, err
	}
	shared.Changed = changed
	shared.IndexLoads++

	invariants, err := loaders.Invariants(repo)
	if err != nil {
		return nil, err
	}
	shared.Invariants = invariants
	shared.MemLoads++

	if opts.LoadGraph || graph.Enabled() {
		idx, gErr := loaders.GraphIndex(repo)
		if gErr != nil {
			return nil, gErr
		}
		shared.GraphIndex = idx
		shared.GraphLoads++
	}

	return shared, nil
}

func defaultLoaders() *Loaders {
	return &Loaders{
		UnifiedDiff:  gitUnifiedDiff,
		NameStatus:   gitNameStatus,
		ChangedFiles: gitChangedFiles,
		Invariants:   loadInvariants,
		GraphIndex:   loadGraphIndex,
	}
}

func gitUnifiedDiff(repo, base, head string) (string, error) {
	out, err := git.Output(repo, "diff", base+"..."+head)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func gitNameStatus(repo, base, head string) (string, error) {
	out, err := git.Output(repo, "diff", "--name-status", base+"..."+head)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func gitChangedFiles(repo, base, head string) ([]string, error) {
	out, err := git.Output(repo, "diff", "--name-only", base+"..."+head)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func loadInvariants(repo string) ([]mem.Invariant, error) {
	dir := mem.LocateRepoToolbeltDir(repo)
	list, err := mem.NewStore(dir).List("", false)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return list, nil
}

func loadGraphIndex(repo string) (*graph.Index, error) {
	return graph.BuildFromRoot(repo)
}

func runOp(ctx context.Context, op Op, opts *Options, shared *Shared) (StepResult, error) {
	switch op {
	case OpVerify:
		return runVerify(ctx, opts, shared)
	case OpReview:
		return runReview(opts, shared)
	case OpDrift:
		return runDrift(opts, shared)
	case OpGuard:
		return runGuard(opts, shared)
	default:
		return StepResult{}, fmt.Errorf("operação desconhecida: %q", op)
	}
}

func runVerify(ctx context.Context, opts *Options, shared *Shared) (StepResult, error) {
	cfg := buildVerifyConfig(opts, shared)
	res, err := verify.Run(ctx, &cfg)
	if err != nil {
		return StepResult{}, err
	}
	return verifyStepResult(res), nil
}

func buildVerifyConfig(opts *Options, shared *Shared) verify.Config {
	cfg := verify.DefaultConfig(shared.RepoDir)
	cfg.BaseRef = shared.Base
	cfg.HeadRef = shared.Head
	cfg.NameStatusDiff = shared.NameStatus
	cfg.DryRun = opts.DryRun
	if shared.FileConfig != nil {
		if shared.FileConfig.Verify.TestCmd != "" {
			cfg.VisibleTest = shared.FileConfig.Verify.TestCmd
		}
		if shared.FileConfig.Verify.HiddenTestCmd != "" {
			cfg.HiddenTest = shared.FileConfig.Verify.HiddenTestCmd
		}
		if shared.FileConfig.Verify.BuildCmd != "" {
			cfg.BuildCmd = shared.FileConfig.Verify.BuildCmd
		}
		if shared.FileConfig.Verify.LintCmd != "" {
			cfg.LintCmd = shared.FileConfig.Verify.LintCmd
		}
	}
	if opts.TestCmd != "" {
		cfg.VisibleTest = opts.TestCmd
	}
	if opts.HiddenTestCmd != "" {
		cfg.HiddenTest = opts.HiddenTestCmd
	}
	verify.ApplyCmdOverrides(&cfg, opts.BuildCmd, opts.BuildCmdSet, opts.NoBuild, opts.LintCmd, opts.LintCmdSet, opts.NoLint)
	if opts.MaxIter > 0 {
		cfg.MaxIterations = opts.MaxIter
	}
	if opts.Timeout > 0 {
		cfg.Timeout = opts.Timeout
	}
	if opts.Ticket != "" {
		cfg.TicketPath = resolveTicket(shared.RepoDir, opts.Ticket)
	}
	cfg.EnvAllow = append(cfg.EnvAllow, opts.EnvAllow...)
	cfg.EnvPass = append(cfg.EnvPass, opts.EnvPass...)
	if shared.FileConfig != nil {
		cfg.EnvAllow = append(cfg.EnvAllow, shared.FileConfig.Env.Allow...)
		cfg.EnvPass = append(cfg.EnvPass, shared.FileConfig.Env.Pass...)
	}
	return cfg
}

func verifyStepResult(res *verify.Result) StepResult {
	ok := res.Verdict == verify.VerdictPass || res.Verdict == verify.VerdictSkipped
	exit := 0
	if !ok {
		exit = 1
	}
	step := StepResult{Op: string(OpVerify), OK: ok, ExitCode: exit, Result: res}
	if res.Reason != "" {
		step.Reason = res.Reason
		step.Error = res.Error
	}
	return step
}

func annotateStepReason(step *StepResult) {
	if step == nil || step.Reason != "" {
		return
	}
	switch res := step.Result.(type) {
	case *verify.Result:
		if res != nil && res.Reason != "" {
			step.Reason = res.Reason
			step.Error = res.Error
		}
	case map[string]any:
		if errMsg, ok := res["error"].(string); ok && errMsg != "" {
			step.Reason = errMsg
			step.Error = errMsg
		}
	}
}

func runReview(opts *Options, shared *Shared) (StepResult, error) {
	cfg := review.DefaultConfig(shared.RepoDir)
	cfg.BaseRef = shared.Base
	cfg.BaseExplicit = true
	cfg.HeadRef = shared.Head
	cfg.ApprovedSHA = opts.ApprovedSHA
	cfg.TicketPath = resolveTicket(shared.RepoDir, ticketOrDefault(opts.Ticket))
	if opts.Reviewers > 0 {
		cfg.Reviewers = opts.Reviewers
	}
	if len(opts.Standards) > 0 {
		cfg.Standards = opts.Standards
	}
	cfg.Shared = reviewShared(shared)
	rep, err := review.Run(&cfg)
	if err != nil {
		return StepResult{}, err
	}
	ok := reviewOK(rep)
	exit := 0
	if !ok {
		exit = 1
	}
	return StepResult{Op: string(OpReview), OK: ok, ExitCode: exit, Result: rep}, nil
}

func runDrift(opts *Options, shared *Shared) (StepResult, error) {
	ticketPath := resolveTicket(shared.RepoDir, ticketOrDefault(opts.Ticket))
	cfg := review.DefaultConfig(shared.RepoDir)
	cfg.BaseRef = shared.Base
	cfg.BaseExplicit = true
	cfg.HeadRef = shared.Head
	cfg.ApprovedSHA = opts.ApprovedSHA
	cfg.TicketPath = ticketPath
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	cfg.Shared = reviewShared(shared)
	rep, err := review.Run(&cfg)
	if err != nil {
		return StepResult{}, err
	}
	ok := driftOK(rep.Drift)
	exit := 0
	if !ok {
		exit = 1
	}
	return StepResult{Op: string(OpDrift), OK: ok, ExitCode: exit, Result: rep.Drift}, nil
}

func runGuard(opts *Options, shared *Shared) (StepResult, error) {
	res, err := guard.NewChecker(shared.RepoDir).CheckDiff(shared.Base, shared.Head, shared.NameStatus)
	if err != nil {
		return StepResult{}, err
	}
	ok := true
	exit := 0
	if hasStrictError(res.Violations) && !opts.WarnOnly {
		ok = false
		exit = 1
	}
	return StepResult{Op: string(OpGuard), OK: ok, ExitCode: exit, Result: res}, nil
}

func reviewShared(shared *Shared) *review.Shared {
	return &review.Shared{
		Diff:          shared.Diff,
		Changed:       shared.Changed,
		Invariants:    shared.Invariants,
		HasDiff:       true,
		HasChanged:    true,
		HasInvariants: true,
	}
}

func reviewOK(rep *review.Report) bool {
	if rep == nil || !rep.FreshOK {
		return false
	}
	if rep.PinnedApproval != nil && !rep.PinnedApproval.Match {
		return false
	}
	if rep.Drift != nil && (len(rep.Drift.OutOfScope) > 0 || len(rep.Drift.ForbiddenHit) > 0 || rep.Drift.BaseMismatch) {
		return false
	}
	for i := range rep.Findings {
		f := &rep.Findings[i]
		if f.Severity == review.SeverityCritical || f.Severity == review.SeverityError {
			return false
		}
	}
	return true
}

func driftOK(drift *review.DriftReport) bool {
	if drift == nil {
		return true
	}
	if drift.PinnedApproval != nil && !drift.PinnedApproval.Match {
		return false
	}
	return len(drift.OutOfScope) == 0 && len(drift.ForbiddenHit) == 0 && !drift.BaseMismatch
}

func hasStrictError(violations []guard.Violation) bool {
	for _, v := range violations {
		if v.Severity == guard.SeverityError {
			return true
		}
	}
	return false
}

func ticketOrDefault(ticket string) string {
	if ticket == "" {
		return "rd.md"
	}
	return ticket
}

func resolveTicket(repo, ticket string) string {
	if ticket == "" {
		ticket = "rd.md"
	}
	if filepath.IsAbs(ticket) {
		return ticket
	}
	return filepath.Join(repo, ticket)
}

func resolveEffectiveBase(repo, base, ticket string) string {
	if base != "HEAD~1" {
		return base
	}
	path := resolveTicket(repo, ticketOrDefault(ticket))
	t, err := review.LoadTicket(path)
	if err != nil || t == nil || t.BaseCommit == "" {
		return base
	}
	return t.BaseCommit
}

func ExitCode(res *Result) int {
	if res == nil || res.OK {
		return 0
	}
	code := 1
	for _, step := range res.Steps {
		if step.ExitCode > code {
			code = step.ExitCode
		}
	}
	return code
}
