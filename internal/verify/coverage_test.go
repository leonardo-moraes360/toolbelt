package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/guard"
)

func commitThresholds(t *testing.T, dir, content string) string {
	t.Helper()
	writeFile(t, filepath.Join(dir, ".toolbelt", "thresholds.yaml"), content)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "thresholds")
	return gitRev(t, dir)
}

func thresholdsYAML(minPercent string) string {
	return "version: 1\ncoverage:\n  min_percent: " + minPercent + "\n"
}

// genericCoverageCfg reports pct through a generic coverage_cmd.
func genericCoverageCfg(dir, pct string) Config {
	cfg := rulerCfg(dir)
	cfg.CoverageCmd = "echo 'coverage: " + pct + "% of lines'"
	cfg.CoveragePattern = `coverage: ([\d.]+)%`
	return cfg
}

func runCoverage(t *testing.T, cfg *Config) (*MultiResult, *CoverageReport) {
	t.Helper()
	multi, err := RunRulers(context.Background(), cfg, []string{RulerCoverage})
	if err != nil {
		t.Fatal(err)
	}
	rep, ok := multi.Payload().(*CoverageReport)
	if !ok {
		t.Fatalf("payload %T", multi.Payload())
	}
	return multi, rep
}

func floatPtr(v float64) *float64 { return &v }

// goFixtureModule writes a module where a and b have tests and c has none,
// so -coverpkg=./... repeats every block once per test binary.
func goFixtureModule(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/covfix\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "a", "a.go"), `package a

func Sign(n int) int {
	if n > 0 {
		return 1
	}
	if n < 0 {
		return -1
	}
	return 0
}

func Unused(s string) string {
	if s == "" {
		return "empty"
	}
	return s + "!"
}
`)
	writeFile(t, filepath.Join(dir, "a", "a_test.go"), `package a

import "testing"

func TestSign(t *testing.T) {
	if Sign(3) != 1 {
		t.Fatal("sign")
	}
}
`)
	writeFile(t, filepath.Join(dir, "b", "b.go"), `package b

import "example.com/covfix/c"

func Twice(n int) int {
	if n == 0 {
		return c.Zero()
	}
	return n * 2
}
`)
	writeFile(t, filepath.Join(dir, "b", "b_test.go"), `package b

import "testing"

func TestTwice(t *testing.T) {
	if Twice(0) != 0 || Twice(2) != 4 {
		t.Fatal("twice")
	}
}
`)
	writeFile(t, filepath.Join(dir, "c", "c.go"), `package c

func Zero() int { return 0 }

func Many(n int) []int {
	out := make([]int, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, i)
	}
	return out
}
`)
}

func goEnv() []string {
	return append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "GOFLAGS=")
}

func runGo(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = goEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// goToolCoverTotal is the percentage on go tool cover -func's total line.
func goToolCoverTotal(t *testing.T, dir, profile string) float64 {
	t.Helper()
	out := strings.TrimSpace(runGo(t, dir, "tool", "cover", "-func="+profile))
	lines := strings.Split(out, "\n")
	f := strings.Fields(lines[len(lines)-1])
	if len(f) == 0 || f[0] != "total:" {
		t.Fatalf("no total line:\n%s", out)
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(f[len(f)-1], "%"), 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func parseProfileFile(t *testing.T, path string) float64 {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseGoCoverProfile(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func requireGo(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("runs go test on a fixture module")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
}

func TestParseGoCoverProfileMatchesGoToolCover(t *testing.T) {
	requireGo(t)
	dir := t.TempDir()
	goFixtureModule(t, dir)
	plain := filepath.Join(dir, "plain.out")
	runGo(t, dir, "test", "-coverprofile="+plain, "./...")
	pkg := filepath.Join(dir, "pkg.out")
	runGo(t, dir, "test", "-coverpkg=./...", "-coverprofile="+pkg, "./...")

	data, err := os.ReadFile(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(data), "covfix/c/c.go:3."); lines < 2 {
		t.Fatalf("fixture lacks -coverpkg duplicates (%d copies of c.go:3):\n%s", lines, data)
	}
	for _, profile := range []string{plain, pkg} {
		got, want := parseProfileFile(t, profile), goToolCoverTotal(t, dir, profile)
		if math.Abs(got-want) > 0.1 {
			t.Fatalf("%s: got %.2f want %.1f (go tool cover)", filepath.Base(profile), got, want)
		}
	}
}

func TestParseGoCoverProfileDedupAndErrors(t *testing.T) {
	profile := "mode: set\n" +
		"m/x.go:1.1,2.2 3 0\n" +
		"m/x.go:1.1,2.2 3 1\n" + // duplicate copy that ran: covered
		"m/x.go:4.1,5.2 1 0\n"
	got, err := ParseGoCoverProfile(strings.NewReader(profile), nil)
	if err != nil || got != 75 {
		t.Fatalf("got %v err=%v want 75", got, err)
	}
	for _, bad := range []string{"m/x.go:1.1,2.2 3 1\n", "mode: set\nm/x.go:1.1,2.2 x 1\n", "mode: set\nm/x.go:1.1,2.2 3 -1\n",
		"mode: set\nm/x.go:1.1,2.2 3 1\nm/x.go:1.1,2.2 2 1\n", "mode: set\njunk\n"} {
		if _, err := ParseGoCoverProfile(strings.NewReader(bad), nil); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}

func goCoverageRepo(t *testing.T) string {
	t.Helper()
	dir := goldenRepo(t)
	goFixtureModule(t, dir)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "module")
	return dir
}

func TestCoverageRulerGoDefault(t *testing.T) {
	requireGo(t)
	dir := goCoverageRepo(t)
	want := filepath.Join(t.TempDir(), "want.out")
	runGo(t, dir, "test", "-coverprofile="+want, "./...")
	total := goToolCoverTotal(t, dir, want)

	commitThresholds(t, dir, thresholdsYAML(strconv.FormatFloat(total-1, 'f', 1, 64)))
	cfg := rulerCfg(dir)
	multi, rep := runCoverage(t, &cfg)
	if multi.Verdict != VerdictPass || rep.Coverage == nil || rep.Coverage.Percent == nil {
		t.Fatalf("verdict=%s rep=%+v", multi.Verdict, rep)
	}
	c := rep.Coverage
	if math.Abs(*c.Percent-total) > 0.1 || c.Metric != MetricStatements || c.ThresholdSource != ThresholdSourceBase ||
		c.Cmd != goCoverageCmdPlan {
		t.Fatalf("coverage=%+v want percent %.1f", c, total)
	}
	if rep.Metrics["coverage_percent"] == "" {
		t.Fatalf("metrics=%v", rep.Metrics)
	}

	commitThresholds(t, dir, thresholdsYAML(strconv.FormatFloat(total+1, 'f', 1, 64)))
	cfg = rulerCfg(dir)
	multi, rep = runCoverage(t, &cfg)
	if multi.Verdict != VerdictFail || multi.ExitCode() != 1 || !strings.Contains(rep.Reason, "below the threshold") {
		t.Fatalf("verdict=%s reason=%q", multi.Verdict, rep.Reason)
	}
}

func TestCoverageRulerGoTestsFail(t *testing.T) {
	requireGo(t)
	dir := goCoverageRepo(t)
	writeFile(t, filepath.Join(dir, "a", "fail_test.go"), "package a\n\nimport \"testing\"\n\nfunc TestBoom(t *testing.T) { t.Fatal(\"boom-excerpt\") }\n")
	commitThresholds(t, dir, thresholdsYAML("10"))
	cfg := rulerCfg(dir)
	multi, rep := runCoverage(t, &cfg)
	if multi.Verdict != VerdictFail || !strings.Contains(rep.Reason, "boom-excerpt") || rep.Coverage.Percent != nil {
		t.Fatalf("verdict=%s reason=%q", multi.Verdict, rep.Reason)
	}
}

func TestCoverageRulerBaseThreshold(t *testing.T) {
	dir := goldenRepo(t)
	commitThresholds(t, dir, thresholdsYAML("80"))
	for pct, want := range map[string]Verdict{"79.9": VerdictFail, "80": VerdictPass, "93.5": VerdictPass} {
		cfg := genericCoverageCfg(dir, pct)
		multi, rep := runCoverage(t, &cfg)
		if multi.Verdict != want {
			t.Fatalf("%s%%: verdict=%s want %s (%q)", pct, multi.Verdict, want, rep.Reason)
		}
		if rep.Coverage.Metric != MetricReported || rep.Coverage.Threshold != 80 || rep.Coverage.ThresholdSource != ThresholdSourceBase {
			t.Fatalf("coverage=%+v", rep.Coverage)
		}
	}
}

func TestCoverageRulerMinCoverageOnlyRaises(t *testing.T) {
	dir := goldenRepo(t)
	commitThresholds(t, dir, thresholdsYAML("70"))
	cases := []struct {
		flag    float64
		verdict Verdict
		th      float64
		source  string
	}{
		{60, VerdictPass, 70, ThresholdSourceBase},
		{80, VerdictFail, 80, ThresholdSourceFlag},
	}
	for _, c := range cases {
		cfg := genericCoverageCfg(dir, "75")
		cfg.MinCoverage = floatPtr(c.flag)
		multi, rep := runCoverage(t, &cfg)
		if multi.Verdict != c.verdict || rep.Coverage.Threshold != c.th || rep.Coverage.ThresholdSource != c.source {
			t.Fatalf("flag %v: verdict=%s coverage=%+v", c.flag, multi.Verdict, rep.Coverage)
		}
	}
	// Without a base file the flag is the threshold.
	bare := goldenRepo(t)
	cfg := genericCoverageCfg(bare, "75")
	cfg.MinCoverage = floatPtr(76)
	if multi, rep := runCoverage(t, &cfg); multi.Verdict != VerdictFail || rep.Coverage.ThresholdSource != ThresholdSourceFlag {
		t.Fatalf("verdict=%s coverage=%+v", multi.Verdict, rep.Coverage)
	}
}

// A change that lowers min_percent does not lower the threshold used (the
// base file is read) and guard flags the protected path.
func TestCoverageRulerLoweredThresholdIgnored(t *testing.T) {
	dir := goldenRepo(t)
	base := commitThresholds(t, dir, thresholdsYAML("80"))
	head := commitThresholds(t, dir, thresholdsYAML("10"))

	cfg := genericCoverageCfg(dir, "50")
	st := &RunState{Config: &cfg, BaseSHA: base, HeadSHA: head, WorkDir: dir}
	rr, err := coverageRuler{}.Run(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	rep := rr.Payload.(*CoverageReport)
	if rr.Verdict != VerdictFail || rep.Coverage.Threshold != 80 {
		t.Fatalf("verdict=%s coverage=%+v", rr.Verdict, rep.Coverage)
	}

	cfg = genericCoverageCfg(dir, "50")
	cfg.BaseRef, cfg.HeadRef = base, head
	multi, err := RunRulers(context.Background(), &cfg, []string{RulerTests, RulerCoverage})
	if err != nil {
		t.Fatal(err)
	}
	tests := multi.Rulers[RulerTests].(*Result)
	flagged := slices.ContainsFunc(tests.Guard.Violations, func(v guard.Violation) bool {
		return v.File == ".toolbelt/thresholds.yaml" && v.Severity == guard.SeverityError
	})
	if multi.Verdict != VerdictFail || !flagged || !strings.Contains(multi.FirstReason(), "guard aborted verify") {
		t.Fatalf("verdict=%s guard=%+v reason=%q", multi.Verdict, tests.Guard, multi.FirstReason())
	}
}

func TestCoverageRulerCombinedWithTests(t *testing.T) {
	dir := goldenRepo(t)
	cfg := rulerCfg(dir)
	multi, err := RunRulers(context.Background(), &cfg, []string{RulerTests, RulerCoverage})
	if err != nil {
		t.Fatal(err)
	}
	rep := multi.Rulers[RulerCoverage].(*CoverageReport)
	if multi.Verdict != VerdictPass || multi.ExitCode() != 0 || rep.Verdict != VerdictSkipped ||
		rep.SkipKind != SkipNotApplicable || rep.Reason != reasonNoThreshold {
		t.Fatalf("no threshold: verdict=%s rep=%+v", multi.Verdict, rep)
	}

	commitThresholds(t, dir, thresholdsYAML("50"))
	cfg = rulerCfg(dir)
	multi, err = RunRulers(context.Background(), &cfg, []string{RulerTests, RulerCoverage})
	if err != nil {
		t.Fatal(err)
	}
	rep = multi.Rulers[RulerCoverage].(*CoverageReport)
	if multi.Verdict != VerdictFail || multi.ExitCode() != 1 || rep.Reason != reasonNoCoverageCmd {
		t.Fatalf("no command: verdict=%s reason=%q", multi.Verdict, rep.Reason)
	}

	cfg = genericCoverageCfg(dir, "40")
	cfg.VisibleTest = "false"
	cfg.MaxIterations = 1
	multi, err = RunRulers(context.Background(), &cfg, []string{RulerTests, RulerCoverage})
	if err != nil {
		t.Fatal(err)
	}
	if multi.Verdict != VerdictEscalate {
		t.Fatalf("worst verdict=%s", multi.Verdict)
	}
}

func TestCoverageRulerGenericPattern(t *testing.T) {
	dir := goldenRepo(t)
	commitThresholds(t, dir, thresholdsYAML("50"))

	cfg := rulerCfg(dir)
	cfg.CoverageCmd = "printf 'pkg/a 10%%\\npkg/b 30%%\\nTOTAL 62.5%%\\n'"
	cfg.CoveragePattern = `\s([\d.]+)%`
	_, rep := runCoverage(t, &cfg)
	if rep.Verdict != VerdictPass || *rep.Coverage.Percent != 62.5 || rep.Metrics["coverage_percent"] != "62.50" {
		t.Fatalf("last match: rep=%+v", rep)
	}

	cfg.CoverageCmd = "echo no numbers here"
	_, rep = runCoverage(t, &cfg)
	if rep.Verdict != VerdictFail || rep.Reason != reasonPatternNoMatch {
		t.Fatalf("no match: rep=%+v", rep)
	}

	cfg.CoverageCmd = "echo 'TOTAL 120%'"
	if _, rep = runCoverage(t, &cfg); rep.Verdict != VerdictFail || !strings.Contains(rep.Reason, "not a percentage") {
		t.Fatalf("out of range: rep=%+v", rep)
	}

	cfg.CoverageCmd = "echo cov-excerpt; exit 1"
	if _, rep = runCoverage(t, &cfg); rep.Verdict != VerdictFail || !strings.Contains(rep.Reason, "cov-excerpt") {
		t.Fatalf("failing cmd: rep=%+v", rep)
	}

	cfg.CoverageCmd = "tb-no-such-coverage-tool-xyz"
	if multi, rep := runCoverage(t, &cfg); multi.ExitCode() != 127 || !strings.Contains(rep.Reason, "verify.coverage_cmd") {
		t.Fatalf("missing cmd: exit=%d rep=%+v", multi.ExitCode(), rep)
	}

	cfg.CoverageCmd, cfg.CoveragePattern = "echo 'TOTAL 99%'", ""
	if _, rep = runCoverage(t, &cfg); rep.Verdict != VerdictFail || rep.Reason != reasonNoCoveragePatrn {
		t.Fatalf("no pattern: rep=%+v", rep)
	}
}

func TestValidateCoveragePattern(t *testing.T) {
	for pattern, ok := range map[string]bool{
		"": true, `(\d+)%`: true, `(?:x)(\d+)`: true,
		`\d+%`: false, `(\d+)\.(\d+)`: false, `(`: false,
	} {
		if err := ValidateCoveragePattern(pattern); (err == nil) != ok {
			t.Errorf("%q: err=%v", pattern, err)
		}
	}
	cfg := DefaultConfig(t.TempDir())
	err := MergeFlags(&cfg, nil, &FlagOverrides{CoveragePattern: CmdOverride{Cmd: `(a)(b)`, Set: true}})
	if !IsUsageError(err) {
		t.Fatalf("flag pattern: err=%v, want usage error", err)
	}
	if err := MergeFlags(&cfg, &config.VerifyConfig{CoveragePattern: `nogroup`}, nil); err == nil || IsUsageError(err) {
		t.Fatalf("file pattern: err=%v, want config error", err)
	}
	for _, v := range []float64{-1, 100.5, math.NaN()} {
		if err := MergeFlags(&cfg, nil, &FlagOverrides{MinCoverage: floatPtr(v)}); !IsUsageError(err) {
			t.Fatalf("min_coverage %v: err=%v", v, err)
		}
	}

	dir := goldenRepo(t)
	bad := commitConfig(t, dir, "verify:\n  coverage_pattern: '(a)(b)'\n")
	if _, err := LoadVerifyConfig(dir, bad, bad, nil, false); err == nil || !strings.Contains(err.Error(), "capture group") {
		t.Fatalf("base config pattern: err=%v", err)
	}
}

func TestCoverageThresholdsMalformedAbort(t *testing.T) {
	for name, content := range map[string]string{
		"syntax":      "version: 1\ncoverage: [unclosed\n",
		"unknown key": "version: 1\ncoverage:\n  min_percent: 50\n  max_percent: 90\n",
		"top key":     "version: 1\nbench: {}\n",
		"range":       thresholdsYAML("101"),
		"negative":    thresholdsYAML("-1"),
		"string":      thresholdsYAML("'eighty'"),
		"version":     "version: 2\ncoverage:\n  min_percent: 50\n",
		"empty":       "",
	} {
		t.Run(name, func(t *testing.T) {
			dir := goldenRepo(t)
			commitThresholds(t, dir, content)
			cfg := genericCoverageCfg(dir, "99")
			if _, err := RunRulers(context.Background(), &cfg, []string{RulerTests, RulerCoverage}); err == nil ||
				!strings.Contains(err.Error(), "thresholds.yaml malformed") {
				t.Fatalf("err=%v, want abort", err)
			}
		})
	}
	// version 1 without a coverage section is no threshold, not an error.
	dir := goldenRepo(t)
	commitThresholds(t, dir, "version: 1\n")
	cfg := genericCoverageCfg(dir, "1")
	if _, rep := runCoverage(t, &cfg); rep.SkipKind != SkipNotApplicable {
		t.Fatalf("rep=%+v", rep)
	}
}

func TestCoverageThresholdsSymlinkAborts(t *testing.T) {
	dir := goldenRepo(t)
	writeFile(t, filepath.Join(dir, "real.yaml"), thresholdsYAML("1"))
	if err := os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../real.yaml", filepath.Join(dir, ".toolbelt", "thresholds.yaml")); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "link")
	cfg := genericCoverageCfg(dir, "99")
	_, err := RunRulers(context.Background(), &cfg, []string{RulerCoverage})
	if err == nil || !strings.Contains(err.Error(), "symlinked .toolbelt/thresholds.yaml") {
		t.Fatalf("err=%v", err)
	}
}

func TestCoverageDryRunPlan(t *testing.T) {
	dir := goldenRepo(t)
	cfg := genericCoverageCfg(dir, "1")
	cfg.DryRun = true
	multi, err := RunRulers(context.Background(), &cfg, []string{RulerTests, RulerCoverage})
	if err != nil {
		t.Fatal(err)
	}
	rep := multi.Rulers[RulerCoverage].(*CoverageReport)
	if !multi.DryRun || rep.Plan == nil || rep.Plan.Commands["coverage"] != cfg.CoverageCmd {
		t.Fatalf("plan=%+v", rep.Plan)
	}

	goDir := goldenRepo(t)
	writeFile(t, filepath.Join(goDir, "go.mod"), "module x\n")
	runGit(t, goDir, "add", "-A")
	runGit(t, goDir, "commit", "-q", "-m", "go.mod")
	cfg = rulerCfg(goDir)
	cfg.DryRun = true
	if _, rep = runCoverage(t, &cfg); rep.Plan.Commands["coverage"] != goCoverageCmdPlan {
		t.Fatalf("go plan=%+v", rep.Plan)
	}
}

func TestMergeFlagsCoverageOverrides(t *testing.T) {
	cfg := DefaultConfig("/r")
	vc := &config.VerifyConfig{CoverageCmd: "base-cov", CoveragePattern: `(\d+)%`}
	o := &FlagOverrides{
		CoverageCmd:     CmdOverride{Cmd: "flag-cov", Set: true},
		CoveragePattern: CmdOverride{Cmd: `(\d+)%`, Set: true},
		MinCoverage:     floatPtr(90),
	}
	if err := MergeFlags(&cfg, vc, o); err != nil {
		t.Fatal(err)
	}
	if cfg.CoverageCmd != "flag-cov" || *cfg.MinCoverage != 90 {
		t.Fatalf("cfg=%+v", cfg)
	}
	if want := []string{"coverage_cmd", "min_coverage"}; !slices.Equal(cfg.AppliedOverrides, want) {
		t.Fatalf("overrides=%v want %v", cfg.AppliedOverrides, want)
	}
}

func TestGoldenMultiRulerJSON(t *testing.T) {
	dir := goldenRepo(t)
	commitThresholds(t, dir, thresholdsYAML("80"))
	cfg := genericCoverageCfg(dir, "85.25")
	multi, err := RunRulers(context.Background(), &cfg, []string{RulerTests, RulerCoverage})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(multi.Payload(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	rulers := m["rulers"].(map[string]any)
	for name, payload := range rulers {
		one, _ := json.Marshal(payload)
		rulers[name] = json.RawMessage(normalizeGolden(t, one, dir))
	}
	got, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "golden", "multi_tests_coverage.json")
	if *updateGolden {
		writeFile(t, path, string(got)+"\n")
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(got)) {
		t.Fatalf("golden mismatch\n--- want\n%s\n--- got\n%s", want, got)
	}
}

func TestParseGoCoverProfileRejectsUnknownFile(t *testing.T) {
	known := func(f string) bool { return f == "m/x.go" }
	if _, err := ParseGoCoverProfile(strings.NewReader("mode: set\nm/x.go:1.1,2.2 3 1\n"), known); err != nil {
		t.Fatal(err)
	}
	_, err := ParseGoCoverProfile(strings.NewReader("mode: set\nm/x.go:1.1,2.2 3 1\nm/fake.go:1.1,2.2 99 1\n"), known)
	if err == nil || !strings.Contains(err.Error(), "profile names unknown file m/fake.go") {
		t.Fatalf("err=%v", err)
	}
}

func TestGoPackageFiles(t *testing.T) {
	requireGo(t)
	dir := t.TempDir()
	goFixtureModule(t, dir)
	cfg := rulerCfg(dir)
	cfg.EnvAllow = []string{"GOWORK", "GOTOOLCHAIN", "GOFLAGS"}
	known, reason, err := goPackageFiles(context.Background(), &cfg)
	if err != nil || reason != "" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	for file, want := range map[string]bool{
		"example.com/covfix/a/a.go":    true,
		"example.com/covfix/c/c.go":    true,
		"example.com/covfix/a/nope.go": false,
		"example.com/covfix/zzz/a.go":  false,
		"other.org/a/a.go":             false,
		"a.go":                         false,
	} {
		if known(file) != want {
			t.Errorf("known(%q)=%v want %v", file, !want, want)
		}
	}
}

// Without go.mod at the base the Go default does not apply, even when the
// head adds one.
func TestCoverageGoDefaultFromBase(t *testing.T) {
	dir := goldenRepo(t)
	base := commitThresholds(t, dir, thresholdsYAML("1"))
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/m\n\ngo 1.22\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "go.mod")
	head := gitRev(t, dir)
	cfg := rulerCfg(dir)
	cfg.BaseRef, cfg.HeadRef = base, head
	_, rep := runCoverage(t, &cfg)
	if rep.Verdict != VerdictFail || rep.Reason != reasonNoCoverageCmd {
		t.Fatalf("rep=%+v", rep)
	}
}

func TestCoverageThresholdsLoadedBeforeTests(t *testing.T) {
	dir := goldenRepo(t)
	commitThresholds(t, dir, "version: 1\ncoverage: [unclosed\n")
	cfg := rulerCfg(dir)
	marker := filepath.Join(t.TempDir(), "ran")
	cfg.VisibleTest = "touch " + marker
	if _, err := RunRulers(context.Background(), &cfg, []string{RulerTests, RulerCoverage}); err == nil {
		t.Fatal("want abort")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("tests ruler ran before thresholds were validated")
	}
}

func TestMinCoverageOverrideListedOnlyWhenRaising(t *testing.T) {
	for flag, want := range map[float64]bool{60: false, 70: false, 80: true} {
		dir := goldenRepo(t)
		commitThresholds(t, dir, thresholdsYAML("70"))
		cfg := genericCoverageCfg(dir, "75")
		cfg.Overrides = &FlagOverrides{MinCoverage: floatPtr(flag)}
		multi, err := RunRulers(context.Background(), &cfg, []string{RulerCoverage})
		if err != nil {
			t.Fatal(err)
		}
		if got := slices.Contains(multi.Overrides, "min_coverage"); got != want {
			t.Errorf("flag %v: overrides=%v, listed=%v want %v", flag, multi.Overrides, got, want)
		}
	}
}

func TestRunConfiguredCmdTimeoutAndOutputCap(t *testing.T) {
	dir := t.TempDir()
	cfg := rulerCfg(dir)
	cfg.Timeout = time.Second
	start := time.Now()
	// The background sleep keeps the output pipe open after sh is killed.
	ok, out, err := runConfiguredCmd(context.Background(), &cfg, "sleep 30 & sleep 30", "verify.test_cmd")
	if ok || err != nil || time.Since(start) > 15*time.Second {
		t.Fatalf("ok=%v err=%v after %s", ok, err, time.Since(start))
	}
	if got := failReason("verify.test_cmd", out); got != "verify.test_cmd timed out after 1s" {
		t.Fatalf("reason=%q", got)
	}

	cfg.Timeout = time.Minute
	_, out, err = runConfiguredCmd(context.Background(), &cfg, "head -c 5000000 /dev/zero | tr '\\0' x; echo END", "verify.test_cmd")
	if err != nil || len(out) > maxCapturedOutput || !strings.HasSuffix(out, "END\n") {
		t.Fatalf("err=%v len=%d tail=%q", err, len(out), out[max(0, len(out)-8):])
	}
}
