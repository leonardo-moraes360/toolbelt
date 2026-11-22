package stack

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/git"
	"github.com/leonardo-moraes360/toolbelt/internal/gitref"
	"github.com/leonardo-moraes360/toolbelt/internal/graph"
	"github.com/leonardo-moraes360/toolbelt/internal/guard"
	"github.com/leonardo-moraes360/toolbelt/internal/mem"
	"github.com/leonardo-moraes360/toolbelt/internal/review"
	"github.com/leonardo-moraes360/toolbelt/internal/secplay"
	"github.com/leonardo-moraes360/toolbelt/internal/verify"
)

type Op string

const (
	OpVerify   Op = "verify"
	OpReview   Op = "review"
	OpDrift    Op = "drift"
	OpGuard    Op = "guard"
	OpEgm      Op = "egm"
	OpVault    Op = "vault"
	OpSecCheck Op = "seccheck"

	maxJobs = 32
)

var knownOps = map[string]Op{
	"verify":   OpVerify,
	"review":   OpReview,
	"drift":    OpDrift,
	"guard":    OpGuard,
	"egm":      OpEgm,
	"vault":    OpVault,
	"seccheck": OpSecCheck,
}

func ParseOp(name string) (Op, error) {
	op, ok := knownOps[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return "", fmt.Errorf("unknown operation: %q (use verify, review, drift, guard, vault or egm)", name)
	}
	return op, nil
}

func ParseOps(names []string) ([]Op, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("provide at least one operation (verify, review, drift, guard, vault, egm)")
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
	RepoDir            string
	Base               string
	Head               string
	Ops                []Op
	FailFast           bool
	Jobs               int
	Ticket             string
	ApprovedSHA        string
	ApprovedBase       string
	RequireApprovedSHA bool
	WarnOnly           bool
	TestCmd            string
	HiddenTestCmd      string
	BuildCmd           string
	LintCmd            string
	BuildCmdSet        bool
	LintCmdSet         bool
	NoBuild            bool
	NoLint             bool
	// Ruler is the verify op's --ruler list (empty = tests); visual is
	// refused in a stack.
	Ruler           string
	TypecheckCmd    string
	TypecheckCmdSet bool
	NoTypecheck     bool
	// Checks adds or overrides named checks; SkipChecks drops some by name;
	// NoChecks drops them all.
	Checks     []config.VerifyCheck
	SkipChecks []string
	NoChecks   bool
	// Coverage ruler (R4): the *Set flags tell an explicit empty value from
	// an absent one; MinCoverage only raises the base threshold.
	CoverageCmd        string
	CoverageCmdSet     bool
	CoveragePattern    string
	CoveragePatternSet bool
	MinCoverage        *float64
	DryRun             bool
	MaxIter            int
	Timeout            time.Duration
	Reviewers          int
	Standards          []string
	LoadGraph          bool
	Loaders            *Loaders
	EnvAllow           []string
	EnvPass            []string
	// EgmDocs are added to the egm op's selection (R33).
	EgmDocs []string
	// Strict makes PASS_PARTIAL and an empty egm selection fail (R33).
	Strict bool
	// EgmMode is local (default) or ci; EgmConfig is the egm user config
	// (-c), required by ci; EgmAllowOffline accepts a deterministic-only
	// round in ci (docs/specs/egm-trust.md, T-R2 to T-R5).
	EgmMode         string
	EgmConfig       string
	EgmAllowOffline bool
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
	// VerifyVerdict is the verdict of a verify step that already ran in this
	// stack (verify is never parallel, so later review steps can read it).
	VerifyVerdict string

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
	// Warn is set by a step that passed with a downgrade (egm PASS_PARTIAL,
	// empty selection); it never changes the exit code.
	Warn   bool `json:"warn,omitempty"`
	Result any  `json:"result"`
}

type Result struct {
	OK              bool         `json:"ok"`
	Warn            bool         `json:"warn,omitempty"`
	Steps           []StepResult `json:"steps"`
	TotalDurationMs float64      `json:"total_duration_ms"`
}

func durationMillis(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func failedStep(op Op, err error) StepResult {
	msg := "unknown error"
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

func ResolveJobs(n int) int {
	if n == 0 {
		n = runtime.NumCPU()
	}
	if n < 1 {
		n = 1
	}
	if n > maxJobs {
		n = maxJobs
	}
	return n
}

func isParallelOp(op Op) bool {
	return op == OpGuard || op == OpDrift || op == OpReview || op == OpVault || op == OpSecCheck
}

func validateOptions(opts *Options) error {
	if opts == nil {
		return fmt.Errorf("options missing")
	}
	if len(opts.Ops) == 0 {
		return fmt.Errorf("provide at least one operation")
	}
	for _, op := range opts.Ops {
		if _, err := ParseOp(string(op)); err != nil {
			return err
		}
	}
	if _, err := verifyRulers(opts.Ruler); err != nil {
		return err
	}
	if err := verify.ValidateChecks(opts.Checks); err != nil {
		return &verify.UsageError{Msg: "--check: " + strings.TrimPrefix(err.Error(), "verify.checks: ")}
	}
	// The coverage overrides fail the same way inside the verify step; checking
	// here turns a bad pattern or floor into exit 2 before any stage runs.
	cfg := verify.DefaultConfig(opts.RepoDir)
	return verify.MergeFlags(&cfg, nil, &verify.FlagOverrides{
		CoveragePattern: verify.CmdOverride{Cmd: opts.CoveragePattern, Set: opts.CoveragePatternSet},
		MinCoverage:     opts.MinCoverage,
	})
}

// verifyRulers parses the verify op's ruler list; visual needs a browser and
// screens, not the stack's shared diff, so it is refused here.
func verifyRulers(s string) ([]string, error) {
	names, err := verify.ParseRulers(s)
	if err != nil {
		return nil, err
	}
	for _, n := range names {
		if n == verify.RulerVisual {
			return nil, &verify.UsageError{Msg: "ruler visual is not available in tb stack; run tb verify run --ruler visual"}
		}
	}
	return names, nil
}

func Run(ctx context.Context, opts *Options) (*Result, error) {
	if err := validateOptions(opts); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	opts.Ops = egmLast(opts.Ops)

	jobs := ResolveJobs(opts.Jobs)
	start := time.Now()
	shared, err := Prepare(ctx, opts)
	if err != nil {
		out := &Result{OK: false, Steps: make([]StepResult, 0, 1)}
		step := failedStep(opts.Ops[0], err)
		step.DurationMs = durationMillis(time.Since(start))
		out.Steps = append(out.Steps, step)
		out.TotalDurationMs = step.DurationMs
		return out, nil
	}

	out := runWaves(ctx, opts, shared, jobs)
	for i := range out.Steps {
		out.Warn = out.Warn || out.Steps[i].Warn
	}
	out.TotalDurationMs = durationMillis(time.Since(start))
	return out, nil
}

// egmLast moves the egm op to the end, keeping the order of the others: it
// is not parallel and gates on the final content (R33).
func egmLast(ops []Op) []Op {
	out := make([]Op, 0, len(ops))
	n := 0
	for _, op := range ops {
		if op == OpEgm {
			n++
			continue
		}
		out = append(out, op)
	}
	if n > 0 {
		out = append(out, OpEgm)
	}
	return out
}

func runWaves(ctx context.Context, opts *Options, shared *Shared, jobs int) *Result {
	out := &Result{OK: true, Steps: make([]StepResult, 0, len(opts.Ops))}
	for i := 0; i < len(opts.Ops); {
		if ctx.Err() != nil {
			out.OK = false
			return out
		}
		next, stop := advanceWave(ctx, opts, shared, jobs, i, out)
		if stop {
			return out
		}
		i = next
	}
	return out
}

func advanceWave(ctx context.Context, opts *Options, shared *Shared, jobs, i int, out *Result) (int, bool) {
	if !isParallelOp(opts.Ops[i]) {
		step := timedOp(ctx, opts.Ops[i], opts, shared)
		out.Steps = append(out.Steps, step)
		if step.OK {
			return i + 1, false
		}
		out.OK = false
		return i + 1, opts.FailFast
	}

	waveEnd := i + 1
	for waveEnd < len(opts.Ops) && isParallelOp(opts.Ops[waveEnd]) {
		waveEnd++
	}
	steps, waveOK := runParallelWave(ctx, opts.Ops[i:waveEnd], opts, shared, jobs)
	out.Steps = append(out.Steps, steps...)
	if waveOK {
		return waveEnd, false
	}
	out.OK = false
	return waveEnd, opts.FailFast
}

func timedOp(ctx context.Context, op Op, opts *Options, shared *Shared) StepResult {
	stepStart := time.Now()
	step, runErr := runOp(ctx, op, opts, shared)
	if runErr != nil {
		step = failedStep(op, runErr)
	}
	step.DurationMs = durationMillis(time.Since(stepStart))
	annotateStepReason(&step)
	return step
}

func runParallelWave(ctx context.Context, wave []Op, opts *Options, shared *Shared, jobs int) ([]StepResult, bool) {
	if len(wave) == 0 {
		return nil, true
	}
	if jobs <= 1 || len(wave) == 1 {
		steps := make([]StepResult, 0, len(wave))
		ok := true
		for _, op := range wave {
			step := timedOp(ctx, op, opts, shared)
			steps = append(steps, step)
			if !step.OK {
				ok = false
				if opts.FailFast {
					break
				}
			}
		}
		return steps, ok
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(jobs)

	var mu sync.Mutex
	slots := make([]StepResult, len(wave))
	done := make([]bool, len(wave))

	for idx, op := range wave {
		idx, op := idx, op
		g.Go(func() error {
			step := timedOp(gctx, op, opts, shared)
			mu.Lock()
			slots[idx] = step
			done[idx] = true
			mu.Unlock()
			if !step.OK && opts.FailFast {
				return fmt.Errorf("fail-fast: %s", op)
			}
			return nil
		})
	}
	_ = g.Wait()

	steps := make([]StepResult, 0, len(wave))
	ok := true
	for i := range wave {
		if !done[i] {
			steps = append(steps, failedStep(wave[i], context.Canceled))
			ok = false
			continue
		}
		steps = append(steps, slots[i])
		if !slots[i].OK {
			ok = false
		}
	}
	return steps, ok
}

type prepareLoads struct {
	diff       string
	nameStatus string
	changed    []string
	invariants []mem.Invariant
	graphIdx   *graph.Index
}

func loadSharedInputs(ctx context.Context, opts *Options, loaders *Loaders, repo, base, head string) (*prepareLoads, error) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(ResolveJobs(opts.Jobs))

	var diff, nameStatus string
	var changed []string
	var invariants []mem.Invariant
	var graphIdx *graph.Index

	g.Go(func() error {
		var err error
		diff, err = loaders.UnifiedDiff(repo, base, head)
		return err
	})
	g.Go(func() error {
		var err error
		nameStatus, err = loaders.NameStatus(repo, base, head)
		return err
	})
	g.Go(func() error {
		var err error
		changed, err = loaders.ChangedFiles(repo, base, head)
		return err
	})
	g.Go(func() error {
		var err error
		invariants, err = loaders.Invariants(repo)
		return err
	})
	if opts.LoadGraph || graph.Enabled() {
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			var err error
			graphIdx, err = loaders.GraphIndex(repo)
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return &prepareLoads{
		diff:       diff,
		nameStatus: nameStatus,
		changed:    changed,
		invariants: invariants,
		graphIdx:   graphIdx,
	}, nil
}

func Prepare(ctx context.Context, opts *Options) (*Shared, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	repo, err := resolveRepoDir(opts.RepoDir)
	if err != nil {
		return nil, err
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

	loads, err := loadSharedInputs(ctx, opts, loaders, repo, base, head)
	if err != nil {
		return nil, err
	}
	shared.Diff = loads.diff
	shared.DiffLoads++
	shared.NameStatus = loads.nameStatus
	shared.NameStatusLoads++
	shared.Changed = loads.changed
	shared.IndexLoads++
	shared.Invariants = loads.invariants
	shared.MemLoads++
	if loads.graphIdx != nil {
		shared.GraphIndex = loads.graphIdx
		shared.GraphLoads++
	}
	return shared, nil
}

func resolveRepoDir(repo string) (string, error) {
	if repo != "" {
		return repo, nil
	}
	return os.Getwd()
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
	out, err := git.Output(repo, "diff", "--end-of-options", base+"..."+head)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func gitNameStatus(repo, base, head string) (string, error) {
	out, err := git.Output(repo, "diff", "--name-status", "--end-of-options", base+"..."+head)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func gitChangedFiles(repo, base, head string) ([]string, error) {
	out, err := git.Output(repo, "diff", "--name-only", "--end-of-options", base+"..."+head)
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
	if err := ctx.Err(); err != nil {
		return StepResult{}, err
	}
	switch op {
	case OpVerify:
		return runVerify(ctx, opts, shared)
	case OpReview:
		return runReview(ctx, opts, shared)
	case OpDrift:
		return runDrift(ctx, opts, shared)
	case OpGuard:
		return runGuard(ctx, opts, shared)
	case OpEgm:
		return runEgm(ctx, opts, shared)
	case OpVault:
		return runVault(ctx, shared)
	case OpSecCheck:
		return runSecCheck(ctx, opts, shared)
	default:
		return StepResult{}, fmt.Errorf("unknown operation: %q", op)
	}
}

func runVerify(ctx context.Context, opts *Options, shared *Shared) (StepResult, error) {
	names, err := verifyRulers(opts.Ruler)
	if err != nil {
		return StepResult{}, err
	}
	cfg, err := buildVerifyConfig(opts, shared)
	if err != nil {
		return StepResult{}, err
	}
	multi, err := verify.RunRulers(ctx, &cfg, names)
	if err != nil {
		return StepResult{}, err
	}
	// A missing command stays a step error, as with verify.Run before.
	for _, rr := range multi.Results {
		if res, ok := rr.Payload.(*verify.Result); ok && rr.ExitCode == 127 {
			return StepResult{}, errors.New(res.Error)
		}
	}
	if !multi.DryRun {
		shared.VerifyVerdict = string(multi.Verdict)
	}
	return verifyStepResult(multi), nil
}

// buildVerifyConfig leaves the command merge to verify: R0 reads the verify
// section from the base ref over the trusted config, then the stack flags
// apply as overrides.
func buildVerifyConfig(opts *Options, shared *Shared) (verify.Config, error) {
	cfg := verify.DefaultConfig(shared.RepoDir)
	cfg.BaseRef = shared.Base
	cfg.HeadRef = shared.Head
	cfg.NameStatusDiff = shared.NameStatus
	cfg.DryRun = opts.DryRun
	trusted, err := config.LoadTrusted()
	if err != nil {
		return cfg, err
	}
	cfg.TrustedConfig = trusted
	cfg.Overrides = verifyOverrides(opts)
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
	// env.allow/pass from config come through R0 (base ref), never the
	// working tree, so a change cannot grant its tests secrets.
	return cfg, nil
}

// verifyOverrides maps the stack options; the hidden command only overrides
// when non-empty, as tb stack always did.
func verifyOverrides(opts *Options) *verify.FlagOverrides {
	return &verify.FlagOverrides{
		TestCmd:         verify.CmdOverride{Cmd: opts.TestCmd, Set: opts.TestCmd != ""},
		HiddenTestCmd:   verify.CmdOverride{Cmd: opts.HiddenTestCmd, Set: opts.HiddenTestCmd != ""},
		BuildCmd:        verify.CmdOverride{Cmd: opts.BuildCmd, Set: opts.BuildCmdSet},
		LintCmd:         verify.CmdOverride{Cmd: opts.LintCmd, Set: opts.LintCmdSet},
		TypecheckCmd:    verify.CmdOverride{Cmd: opts.TypecheckCmd, Set: opts.TypecheckCmdSet},
		NoBuild:         opts.NoBuild,
		NoLint:          opts.NoLint,
		NoTypecheck:     opts.NoTypecheck,
		Checks:          opts.Checks,
		SkipChecks:      opts.SkipChecks,
		NoChecks:        opts.NoChecks,
		CoverageCmd:     verify.CmdOverride{Cmd: opts.CoverageCmd, Set: opts.CoverageCmdSet},
		CoveragePattern: verify.CmdOverride{Cmd: opts.CoveragePattern, Set: opts.CoveragePatternSet},
		MinCoverage:     opts.MinCoverage,
	}
}

func verifyStepResult(multi *verify.MultiResult) StepResult {
	exit := multi.ExitCode()
	step := StepResult{Op: string(OpVerify), OK: exit == 0, ExitCode: exit, Result: multi.Payload()}
	switch p := step.Result.(type) {
	case *verify.Result:
		step.Reason, step.Error = p.Reason, p.Error
	case *verify.CoverageReport:
		if p.Verdict != verify.VerdictPass {
			step.Reason, step.Error = p.Reason, p.Reason
		}
	default:
		if !step.OK {
			step.Reason = multi.FirstReason()
			step.Error = step.Reason
		}
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

func runReview(ctx context.Context, opts *Options, shared *Shared) (StepResult, error) {
	if err := ctx.Err(); err != nil {
		return StepResult{}, err
	}
	cfg := review.DefaultConfig(shared.RepoDir)
	cfg.BaseRef = shared.Base
	cfg.BaseExplicit = true
	cfg.HeadRef = shared.Head
	cfg.ApprovedSHA = opts.ApprovedSHA
	cfg.ApprovedBase = opts.ApprovedBase
	cfg.RequireApprovedSHA = opts.RequireApprovedSHA
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
	if err := ctx.Err(); err != nil {
		return StepResult{}, err
	}
	ok := reviewOK(rep)
	exit := 0
	if !ok {
		exit = 1
	}
	return StepResult{Op: string(OpReview), OK: ok, ExitCode: exit, Result: rep}, nil
}

func runDrift(ctx context.Context, opts *Options, shared *Shared) (StepResult, error) {
	if err := ctx.Err(); err != nil {
		return StepResult{}, err
	}
	ticketPath := resolveTicket(shared.RepoDir, ticketOrDefault(opts.Ticket))
	cfg := review.DefaultConfig(shared.RepoDir)
	cfg.BaseRef = shared.Base
	cfg.BaseExplicit = true
	cfg.HeadRef = shared.Head
	cfg.ApprovedSHA = opts.ApprovedSHA
	cfg.ApprovedBase = opts.ApprovedBase
	cfg.RequireApprovedSHA = opts.RequireApprovedSHA
	cfg.TicketPath = ticketPath
	cfg.Reviewers = 1
	cfg.Standards = []string{"scope"}
	cfg.Shared = reviewShared(shared)
	rep, err := review.Run(&cfg)
	if err != nil {
		return StepResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return StepResult{}, err
	}
	ok := driftOK(rep.Drift)
	exit := 0
	if !ok {
		exit = 1
	}
	return StepResult{Op: string(OpDrift), OK: ok, ExitCode: exit, Result: rep.Drift}, nil
}

func runGuard(ctx context.Context, opts *Options, shared *Shared) (StepResult, error) {
	if err := ctx.Err(); err != nil {
		return StepResult{}, err
	}
	res, err := guard.NewChecker(shared.RepoDir).CheckDiff(shared.Base, shared.Head, shared.NameStatus)
	if err != nil {
		return StepResult{}, err
	}
	if err := ctx.Err(); err != nil {
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

// runSecCheck runs the registered AppSec checks (.toolbelt/secchecks.yaml) over
// the working tree. It is read-only and regex-only: it never executes matched
// content. Error-severity findings fail the step unless --warn-only.
func runSecCheck(ctx context.Context, opts *Options, shared *Shared) (StepResult, error) {
	if err := ctx.Err(); err != nil {
		return StepResult{}, err
	}
	// Scan the git work tree root (not the caller's cwd) and scope to the
	// stack's changed files, so the op gates the diff — like guard/drift — and
	// never fails on untouched legacy code.
	root := secCheckRoot(shared.RepoDir)
	tb := mem.LocateRepoToolbeltDir(root)
	vs, err := secplay.RunPaths(ctx, root, tb, shared.Changed)
	if err != nil {
		return StepResult{}, err
	}
	ok := true
	exit := 0
	if secplay.HasError(vs) && !opts.WarnOnly {
		ok = false
		exit = 1
	}
	return StepResult{Op: string(OpSecCheck), OK: ok, ExitCode: exit, Result: vs}, nil
}

// secCheckRoot resolves the git work-tree root for repoDir, falling back to
// repoDir when it is not inside a git repo.
func secCheckRoot(repoDir string) string {
	out, err := git.Output(repoDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return repoDir
	}
	if top := strings.TrimSpace(string(out)); top != "" {
		return top
	}
	return repoDir
}

func reviewShared(shared *Shared) *review.Shared {
	return &review.Shared{
		Diff:          shared.Diff,
		Changed:       shared.Changed,
		Invariants:    shared.Invariants,
		VerifyVerdict: shared.VerifyVerdict,
		HasDiff:       true,
		HasChanged:    true,
		HasInvariants: true,
	}
}

func reviewOK(rep *review.Report) bool {
	if rep == nil || !rep.FreshOK {
		return false
	}
	if pinnedApprovalMismatch(rep.PinnedApproval) {
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
	if pinnedApprovalMismatch(drift.PinnedApproval) {
		return false
	}
	return len(drift.OutOfScope) == 0 && len(drift.ForbiddenHit) == 0 && !drift.BaseMismatch
}

func pinnedApprovalMismatch(p *review.PinnedApproval) bool {
	return p.Diverged()
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
