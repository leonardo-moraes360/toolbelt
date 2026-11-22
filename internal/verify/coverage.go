package verify

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/git"
	"github.com/leonardo-moraes360/toolbelt/internal/guard"
)

const (
	RulerCoverage = "coverage"

	// Coverage metrics: Go counts statements; a generic command reports
	// whatever its tool measures (usually lines).
	MetricStatements = "statements"
	MetricReported   = "reported"

	// goCoverageCmdPlan is the Go default as shown in plans and reports; the
	// profile path is a private temp file chosen per run.
	goCoverageCmdPlan = "go test -coverprofile=<tmp> ./..."

	reasonNoThreshold     = "coverage: no threshold"
	reasonNoCoverageCmd   = "coverage: threshold set but no coverage_cmd for this repository"
	reasonPatternNoMatch  = "coverage: pattern did not match"
	reasonNoCoveragePatrn = "coverage: coverage_cmd set but no coverage_pattern to read its output"
)

// CoverageReport is the coverage ruler's payload.
type CoverageReport struct {
	Verdict  Verdict       `json:"verdict"`
	SkipKind SkipKind      `json:"skip_kind,omitempty"`
	Reason   string        `json:"reason,omitempty"`
	Coverage *CoverageInfo `json:"coverage,omitempty"`
	// Guard is set only when guard aborted the run, so a coverage-only run
	// still shows the blocking finding.
	Guard   *guard.CheckResult `json:"guard,omitempty"`
	Metrics map[string]string  `json:"metrics,omitempty"`
	HeadSHA string             `json:"head_sha,omitempty"`
	WorkDir string             `json:"workdir,omitempty"`
	DryRun  bool               `json:"dry_run,omitempty"`
	Plan    *Plan              `json:"plan,omitempty"`
}

// CoverageInfo is present whenever a threshold applies; Percent only once
// a measurement was read.
type CoverageInfo struct {
	Percent         *float64 `json:"percent,omitempty"`
	Threshold       float64  `json:"threshold"`
	ThresholdSource string   `json:"threshold_source"`
	Cmd             string   `json:"cmd,omitempty"`
	Metric          string   `json:"metric,omitempty"`
}

// ValidateCoveragePattern checks verify.coverage_pattern: it must compile
// and have exactly one capture group (the percentage).
func ValidateCoveragePattern(pattern string) error {
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("verify.coverage_pattern: %w", err)
	}
	if n := re.NumSubexp(); n != 1 {
		return fmt.Errorf("verify.coverage_pattern: want exactly one capture group, got %d", n)
	}
	return nil
}

// effectiveThreshold is max(base, flag): the caller may only raise the
// base floor. ok is false when neither sets one.
func effectiveThreshold(base, flag *float64) (value float64, source string, ok bool) {
	switch {
	case base != nil && (flag == nil || *flag <= *base):
		return *base, ThresholdSourceBase, true
	case flag != nil:
		return *flag, ThresholdSourceFlag, true
	}
	return 0, "", false
}

type coverageRuler struct{}

func (coverageRuler) Name() string { return RulerCoverage }

func (coverageRuler) Run(ctx context.Context, st *RunState) (*RulerResult, error) {
	cfg := st.Config
	rep := &CoverageReport{Verdict: VerdictFail, HeadSHA: st.HeadSHA, WorkDir: st.WorkDir}
	if st.GuardAborted {
		rep.Reason, rep.Guard = st.GuardReason, st.Guard
		return coverageResult(rep, 1), nil
	}
	base := st.BaseThresholds
	if base == nil {
		loaded, err := LoadBaseThresholds(cfg.RepoDir, st.BaseSHA)
		if err != nil {
			return nil, err
		}
		base = &loaded
	}
	threshold, source, hasThreshold := effectiveThreshold(base.MinCoverage, cfg.MinCoverage)
	// The Go default is decided by the base ref: a change cannot opt in or
	// out by adding or removing go.mod.
	_, baseGoErr := git.Output(cfg.RepoDir, "cat-file", "-e", st.BaseSHA+":go.mod")
	goDefault := cfg.CoverageCmd == "" && baseGoErr == nil
	cmdShown := cfg.CoverageCmd
	if goDefault {
		cmdShown = goCoverageCmdPlan
	}
	if hasThreshold {
		rep.Coverage = &CoverageInfo{Threshold: threshold, ThresholdSource: source, Cmd: cmdShown}
	}
	if cfg.DryRun {
		return coverageDryRun(rep, st.WorkDir, cmdShown), nil
	}
	switch {
	case !hasThreshold:
		rep.Verdict, rep.SkipKind, rep.Reason = VerdictSkipped, SkipNotApplicable, reasonNoThreshold
		return coverageResult(rep, 0), nil
	case cmdShown == "":
		rep.Reason = reasonNoCoverageCmd
		return coverageResult(rep, 1), nil
	case !goDefault && cfg.CoveragePattern == "":
		rep.Reason = reasonNoCoveragePatrn
		return coverageResult(rep, 1), nil
	}
	return measureCoverage(ctx, st, rep, goDefault)
}

// measureCoverage runs the coverage command and judges the percentage
// against rep.Coverage.Threshold.
func measureCoverage(ctx context.Context, st *RunState, rep *CoverageReport, goDefault bool) (*RulerResult, error) {
	execCfg := *st.Config
	execCfg.RepoDir = st.WorkDir
	measure, metric := measureGenericCoverage, MetricReported
	if goDefault {
		measure, metric = measureGoCoverage, MetricStatements
	}
	rep.Coverage.Metric = metric
	percent, reason, missing, err := measure(ctx, &execCfg)
	if err != nil {
		return nil, err
	}
	if reason != "" {
		rep.Reason = reason
		code := 1
		if missing {
			code = 127
		}
		return coverageResult(rep, code), nil
	}
	shown := math.Round(percent*100) / 100
	rep.Coverage.Percent = &shown
	rep.Metrics = map[string]string{"coverage_percent": strconv.FormatFloat(shown, 'f', 2, 64)}
	threshold := rep.Coverage.Threshold
	if percent >= threshold {
		rep.Verdict = VerdictPass
		return coverageResult(rep, 0), nil
	}
	rep.Reason = fmt.Sprintf("coverage: %s%% is below the threshold %s%% (%s)",
		strconv.FormatFloat(shown, 'f', -1, 64), strconv.FormatFloat(threshold, 'f', -1, 64), rep.Coverage.ThresholdSource)
	return coverageResult(rep, 1), nil
}

func coverageDryRun(rep *CoverageReport, dir, cmd string) *RulerResult {
	rep.DryRun, rep.Verdict, rep.Reason = true, VerdictSkipped, "dry-run: plan generated without executing"
	rep.Plan = &Plan{Dir: dir, Commands: map[string]string{}}
	if cmd != "" {
		rep.Plan.Commands["coverage"] = cmd
	}
	return coverageResult(rep, 0)
}

func coverageResult(rep *CoverageReport, code int) *RulerResult {
	return &RulerResult{Name: RulerCoverage, Verdict: rep.Verdict, SkipKind: rep.SkipKind, ExitCode: code, Payload: rep}
}

// measureGoCoverage runs go test with a profile in a private temp dir and
// computes the statement total itself. reason is set when the run cannot
// give a percentage (tests fail, go missing, unreadable profile).
func measureGoCoverage(ctx context.Context, cfg *Config) (percent float64, reason string, missing bool, err error) {
	tmp, err := os.MkdirTemp("", "tb-coverage-*")
	if err != nil {
		return 0, "", false, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	profile := filepath.Join(tmp, "cover.out")
	command := "go test -coverprofile=" + shellQuote(profile) + " ./..."
	ok, out, runErr := runConfiguredCmd(ctx, cfg, command, "verify.coverage_cmd")
	if runErr != nil {
		return 0, "coverage: " + runErr.Error(), IsMissingCommand(runErr), nil
	}
	if !ok {
		return 0, "coverage: " + failReason("verify.coverage_cmd", out), false, nil
	}
	root, err := os.OpenRoot(tmp)
	if err != nil {
		return 0, "", false, err
	}
	defer func() { _ = root.Close() }()
	f, err := root.Open(filepath.Base(profile))
	if err != nil {
		return 0, "coverage: no profile written: " + err.Error(), false, nil
	}
	defer func() { _ = f.Close() }()
	known, reason, err := goPackageFiles(ctx, cfg)
	if err != nil || reason != "" {
		return 0, reason, false, err
	}
	percent, err = ParseGoCoverProfile(f, known)
	if err != nil {
		return 0, "coverage: " + err.Error(), false, nil
	}
	return percent, "", false, nil
}

// measureGenericCoverage runs coverage_cmd and reads the last match of
// coverage_pattern's capture group as the percentage.
func measureGenericCoverage(ctx context.Context, cfg *Config) (percent float64, reason string, missing bool, err error) {
	re, err := regexp.Compile(cfg.CoveragePattern)
	if err != nil {
		return 0, "", false, ValidateCoveragePattern(cfg.CoveragePattern)
	}
	ok, out, runErr := runConfiguredCmd(ctx, cfg, cfg.CoverageCmd, "verify.coverage_cmd")
	if runErr != nil {
		return 0, "coverage: " + runErr.Error(), IsMissingCommand(runErr), nil
	}
	if !ok {
		return 0, "coverage: " + failReason("verify.coverage_cmd", out), false, nil
	}
	matches := re.FindAllStringSubmatch(stripANSI(out), -1)
	if len(matches) == 0 {
		return 0, reasonPatternNoMatch, false, nil
	}
	raw := strings.TrimSpace(matches[len(matches)-1][1])
	v, perr := strconv.ParseFloat(raw, 64)
	if perr != nil || !validPercent(v) {
		return 0, fmt.Sprintf("coverage: matched value %q is not a percentage in [0, 100]", raw), false, nil
	}
	return v, "", false, nil
}

// ParseGoCoverProfile returns the statement-weighted total of a Go cover
// profile, as go tool cover -func reports it. Blocks are deduplicated by
// file:startLine.startCol,endLine.endCol, since -coverpkg repeats a
// package's blocks once per test binary; a block is covered if any copy ran.
//
// known, when non-nil, vets each profile key's file part (import path plus
// file name): a profile naming anything else is rejected, since a test
// process can write whatever it likes into its own output. Negative counts
// and a block repeated with another statement count are rejected too, as
// go tool cover does.
func ParseGoCoverProfile(r io.Reader, known func(file string) bool) (float64, error) {
	type block struct {
		stmts   int64
		covered bool
	}
	blocks := map[string]*block{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	sawMode := false
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "mode:") {
			sawMode = true
			continue
		}
		key, stmts, count, err := parseProfileLine(line)
		if err != nil {
			return 0, fmt.Errorf("cover profile line %d: %w", n, err)
		}
		if known != nil {
			file := key[:strings.LastIndex(key, ":")]
			if !known(file) {
				return 0, fmt.Errorf("profile names unknown file %s", file)
			}
		}
		b, ok := blocks[key]
		if !ok {
			b = &block{stmts: stmts}
			blocks[key] = b
		} else if b.stmts != stmts {
			return 0, fmt.Errorf("cover profile line %d: inconsistent statement count for %s", n, key)
		}
		b.covered = b.covered || count > 0
	}
	if err := sc.Err(); err != nil {
		return 0, fmt.Errorf("reading cover profile: %w", err)
	}
	if !sawMode {
		return 0, fmt.Errorf("cover profile has no mode line")
	}
	var total, covered int64
	for _, b := range blocks {
		total += b.stmts
		if b.covered {
			covered += b.stmts
		}
	}
	if total == 0 {
		return 0, nil
	}
	return float64(covered) * 100 / float64(total), nil
}

// parseProfileLine splits "<file>:<pos> <numStmt> <count>" from the right,
// since the file name may contain spaces.
func parseProfileLine(line string) (key string, stmts, count int64, err error) {
	f := strings.Fields(line)
	if len(f) < 3 {
		return "", 0, 0, fmt.Errorf("malformed: %q", line)
	}
	stmts, err1 := strconv.ParseInt(f[len(f)-2], 10, 64)
	count, err2 := strconv.ParseInt(f[len(f)-1], 10, 64)
	key = strings.Join(f[:len(f)-2], " ")
	if err1 != nil || err2 != nil || stmts < 0 || count < 0 || !strings.Contains(key, ":") {
		return "", 0, 0, fmt.Errorf("malformed: %q", line)
	}
	return key, stmts, count, nil
}

// shellQuote single-quotes s for sh -c.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// goPackageFiles lists the module's packages (go list ./... in the head
// workdir) and returns a predicate accepting "<import path>/<file>.go" only
// for a regular file that exists in that package's directory under the
// workdir. reason is set when go list fails.
func goPackageFiles(ctx context.Context, cfg *Config) (known func(string) bool, reason string, err error) {
	ok, out, runErr := runConfiguredCmd(ctx, cfg, "go list -f '{{.ImportPath}} {{.Dir}}' ./...", "verify.coverage_cmd")
	if runErr != nil {
		return nil, "coverage: " + runErr.Error(), nil
	}
	if !ok {
		return nil, "coverage: " + failReason("go list", out), nil
	}
	work, err := filepath.EvalSymlinks(cfg.RepoDir)
	if err != nil {
		return nil, "", err
	}
	dirs := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		imp, dir, found := strings.Cut(strings.TrimSpace(line), " ")
		if !found || imp == "" || dir == "" {
			continue
		}
		if resolved, e := filepath.EvalSymlinks(dir); e == nil {
			dir = resolved
		}
		if rel, e := filepath.Rel(work, dir); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			dirs[imp] = dir
		}
	}
	return func(file string) bool {
		i := strings.LastIndex(file, "/")
		if i < 0 {
			return false
		}
		dir, ok := dirs[file[:i]]
		if !ok {
			return false
		}
		fi, err := os.Lstat(filepath.Join(dir, file[i+1:]))
		return err == nil && fi.Mode().IsRegular()
	}, "", nil
}
