package stack

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/gitref"
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
		return nil, fmt.Errorf("informe ao menos uma operação")
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
	MaxIter       int
	Timeout       time.Duration
	Reviewers     int
	Standards     []string
	LoadGraph     bool
	Loaders       *Loaders
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
	Op         string `json:"op"`
	OK         bool   `json:"ok"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Result     any    `json:"result"`
}

type Result struct {
	OK              bool         `json:"ok"`
	Steps           []StepResult `json:"steps"`
	TotalDurationMs int64        `json:"total_duration_ms"`
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

	shared, err := Prepare(opts)
	if err != nil {
		return nil, err
	}

	out := &Result{OK: true, Steps: make([]StepResult, 0, len(opts.Ops))}
	start := time.Now()
	for _, op := range opts.Ops {
		stepStart := time.Now()
		step, runErr := runOp(ctx, op, opts, shared)
		if runErr != nil {
			step = StepResult{
				Op:       string(op),
				OK:       false,
				ExitCode: 1,
				Result:   map[string]any{"error": runErr.Error()},
			}
		}
		step.DurationMs = time.Since(stepStart).Milliseconds()
		out.Steps = append(out.Steps, step)
		if !step.OK {
			out.OK = false
			if opts.FailFast {
				break
			}
		}
	}
	out.TotalDurationMs = time.Since(start).Milliseconds()
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
	base, head, err := resolveStackRefs(repo, opts)
	if err != nil {
		return nil, err
	}
	loaders := opts.Loaders
	if loaders == nil {
		loaders = defaultLoaders()
	}
	shared := &Shared{RepoDir: repo, Base: base, Head: head}
	if fileCfg, err := config.Load(); err == nil {
		shared.FileConfig = fileCfg
	}
	if err := loadSharedGit(shared, loaders); err != nil {
		return nil, err
	}
	if err := loadSharedExtras(opts, shared, loaders); err != nil {
		return nil, err
	}
	return shared, nil
}

func resolveStackRefs(repo string, opts *Options) (base, head string, err error) {
	base = opts.Base
	if base == "" {
		base = "HEAD~1"
	}
	head = opts.Head
	if head == "" {
		head = "HEAD"
	}
	base = resolveEffectiveBase(repo, base, opts.Ticket)
	base, head, err = gitref.ResolvePair(repo, base, head)
	if err != nil {
		return "", "", err
	}
	if opts.ApprovedSHA != "" {
		approved, resolveErr := gitref.Resolve(repo, opts.ApprovedSHA)
		if resolveErr != nil {
			return "", "", fmt.Errorf("approved-sha: %w", resolveErr)
		}
		opts.ApprovedSHA = approved
	}
	return base, head, nil
}

func loadSharedGit(shared *Shared, loaders *Loaders) error {
	diff, err := loaders.UnifiedDiff(shared.RepoDir, shared.Base, shared.Head)
	if err != nil {
		return err
	}
	shared.Diff = diff
	shared.DiffLoads++

	nameStatus, err := loaders.NameStatus(shared.RepoDir, shared.Base, shared.Head)
	if err != nil {
		return err
	}
	shared.NameStatus = nameStatus
	shared.NameStatusLoads++

	changed, err := loaders.ChangedFiles(shared.RepoDir, shared.Base, shared.Head)
	if err != nil {
		return err
	}
	shared.Changed = changed
	shared.IndexLoads++
	return nil
}

func loadSharedExtras(opts *Options, shared *Shared, loaders *Loaders) error {
	invariants, err := loaders.Invariants(shared.RepoDir)
	if err != nil {
		return err
	}
	shared.Invariants = invariants
	shared.MemLoads++

	if opts.LoadGraph || graph.Enabled() {
		idx, gErr := loaders.GraphIndex(shared.RepoDir)
		if gErr != nil {
			return gErr
		}
		shared.GraphIndex = idx
		shared.GraphLoads++
	}
	return nil
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
	cmd := exec.Command("git", "diff", "--end-of-options", base+".."+head) //nolint:gosec // refs resolvidas
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

func gitNameStatus(repo, base, head string) (string, error) {
	cmd := exec.Command("git", "diff", "--name-status", "--end-of-options", base+".."+head) //nolint:gosec // refs resolvidas
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff --name-status: %w", err)
	}
	return string(out), nil
}

func gitChangedFiles(repo, base, head string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--end-of-options", base+".."+head) //nolint:gosec // refs resolvidas
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only: %w", err)
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
	cfg := verify.DefaultConfig(shared.RepoDir)
	cfg.BaseRef = shared.Base
	cfg.HeadRef = shared.Head
	cfg.NameStatusDiff = shared.NameStatus
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
	if opts.MaxIter > 0 {
		cfg.MaxIterations = opts.MaxIter
	}
	if opts.Timeout > 0 {
		cfg.Timeout = opts.Timeout
	}
	if opts.Ticket != "" {
		cfg.TicketPath = resolveTicket(shared.RepoDir, opts.Ticket)
	}
	res, err := verify.Run(ctx, &cfg)
	if err != nil {
		return StepResult{}, err
	}
	ok := res.Verdict == verify.VerdictPass || res.Verdict == verify.VerdictSkipped
	exit := 0
	if !ok {
		exit = 1
	}
	return StepResult{Op: string(OpVerify), OK: ok, ExitCode: exit, Result: res}, nil
}

func runReview(opts *Options, shared *Shared) (StepResult, error) {
	cfg := review.DefaultConfig(shared.RepoDir)
	cfg.BaseRef = shared.Base
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
	for _, f := range rep.Findings {
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
