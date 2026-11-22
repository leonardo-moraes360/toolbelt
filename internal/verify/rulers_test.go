package verify

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
)

func rulerCfg(dir string) Config {
	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.VisibleTest = "true"
	return cfg
}

func TestTypecheckFailEscalatesOnce(t *testing.T) {
	dir := goldenRepo(t)
	logPath := filepath.Join(t.TempDir(), "tc.count")
	cfg := rulerCfg(dir)
	cfg.MaxIterations = 3
	cfg.TypecheckCmd = "echo x >> " + logPath + "; echo 'TS2322: bad type'; exit 2"
	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != VerdictEscalate || res.Iterations != 1 {
		t.Fatalf("verdict=%s iterations=%d", res.Verdict, res.Iterations)
	}
	if !res.TypecheckRan || res.TypecheckPassed == nil || *res.TypecheckPassed {
		t.Fatalf("typecheck_ran=%v typecheck_passed=%v", res.TypecheckRan, res.TypecheckPassed)
	}
	if !strings.HasPrefix(res.Reason, "typecheck failed: TS2322") {
		t.Fatalf("reason=%q", res.Reason)
	}
	if n := countLines(t, logPath); n != 1 {
		t.Fatalf("typecheck ran %d times", n)
	}
	raw, _ := res.JSON()
	if !strings.Contains(string(raw), `"typecheck_passed": false`) {
		t.Fatalf("json lacks typecheck_passed=false:\n%s", raw)
	}
}

func TestTypecheckCachedAcrossIterationsAndOrdered(t *testing.T) {
	dir := goldenRepo(t)
	logPath := filepath.Join(t.TempDir(), "order.log")
	cfg := rulerCfg(dir)
	cfg.MaxIterations = 3
	cfg.BuildCmd = "echo build >> " + logPath
	cfg.TypecheckCmd = "echo typecheck >> " + logPath
	cfg.LintCmd = "echo lint >> " + logPath
	cfg.Checks = []config.VerifyCheck{{Name: "c1", Cmd: "echo c1 >> " + logPath}}
	cfg.VisibleTest = "echo visible >> " + logPath + "; echo --- FAIL: run $(wc -l < " + logPath + "); exit 1"
	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Iterations != 3 || res.Verdict != VerdictEscalate {
		t.Fatalf("verdict=%s iterations=%d", res.Verdict, res.Iterations)
	}
	got := strings.Fields(readFile(t, logPath))
	want := []string{"build", "typecheck", "lint", "c1", "visible", "visible", "visible"}
	if !slices.Equal(got, want) {
		t.Fatalf("order=%v want %v", got, want)
	}
	if res.TypecheckPassed == nil || !*res.TypecheckPassed || len(res.Checks) != 1 || !res.Checks[0].Passed {
		t.Fatalf("typecheck=%v checks=%+v", res.TypecheckPassed, res.Checks)
	}
}

func TestTwoChecksOneFailing(t *testing.T) {
	dir := goldenRepo(t)
	cfg := rulerCfg(dir)
	cfg.Checks = []config.VerifyCheck{
		{Name: "vuln", Cmd: "echo 'GO-2024-0001 found'; exit 3"},
		{Name: "license", Cmd: "true"},
	}
	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != VerdictEscalate {
		t.Fatalf("verdict=%s", res.Verdict)
	}
	if len(res.Checks) != 2 {
		t.Fatalf("checks=%+v", res.Checks)
	}
	if res.Checks[0].Passed || !strings.Contains(res.Checks[0].Reason, "GO-2024-0001") || !res.Checks[1].Passed {
		t.Fatalf("checks=%+v", res.Checks)
	}
	if !strings.HasPrefix(res.Reason, "check vuln failed: GO-2024-0001") {
		t.Fatalf("reason=%q", res.Reason)
	}
}

func TestCheckMissingExecutableExits127(t *testing.T) {
	dir := goldenRepo(t)
	cfg := rulerCfg(dir)
	cfg.Checks = []config.VerifyCheck{{Name: "scan", Cmd: "tb-no-such-scanner-xyz"}}
	multi, err := RunRulers(context.Background(), &cfg, []string{RulerTests})
	if err != nil {
		t.Fatal(err)
	}
	if code := multi.ExitCode(); code != 127 {
		t.Fatalf("exit=%d verdict=%s", code, multi.Verdict)
	}
	res := multi.Payload().(*Result)
	if !strings.Contains(res.Error, "verify.checks.scan") {
		t.Fatalf("error=%q", res.Error)
	}
}

func TestMergeFlagsChecksOverrides(t *testing.T) {
	vc := &config.VerifyConfig{
		TestCmd: "go test ./...",
		Checks:  []config.VerifyCheck{{Name: "a", Cmd: "a1"}, {Name: "b", Cmd: "b1"}},
	}
	cfg := DefaultConfig("/r")
	o := &FlagOverrides{
		Checks:     []config.VerifyCheck{{Name: "b", Cmd: "b2"}, {Name: "c", Cmd: "c1"}},
		SkipChecks: []string{"a"},
	}
	if err := MergeFlags(&cfg, vc, o); err != nil {
		t.Fatal(err)
	}
	want := []config.VerifyCheck{{Name: "b", Cmd: "b2"}, {Name: "c", Cmd: "c1"}}
	if !slices.Equal(cfg.Checks, want) {
		t.Fatalf("checks=%v", cfg.Checks)
	}
	if vc.Checks[1].Cmd != "b1" {
		t.Fatal("MergeFlags mutated the file config")
	}

	cfg = DefaultConfig("/r")
	if err := MergeFlags(&cfg, vc, &FlagOverrides{NoChecks: true}); err != nil || len(cfg.Checks) != 0 {
		t.Fatalf("no-checks: checks=%v err=%v", cfg.Checks, err)
	}
	cfg = DefaultConfig("/r")
	err := MergeFlags(&cfg, vc, &FlagOverrides{SkipChecks: []string{"zzz"}})
	if !IsUsageError(err) {
		t.Fatalf("unknown --no-check: err=%v", err)
	}
	cfg = DefaultConfig("/r")
	if err := MergeFlags(&cfg, nil, &FlagOverrides{Checks: []config.VerifyCheck{{Name: "Bad_Name", Cmd: "x"}}}); err == nil {
		t.Fatal("invalid check name accepted")
	}
}

func TestMergeFlagsCommands(t *testing.T) {
	vc := &config.VerifyConfig{TestCmd: "t-file", BuildCmd: "b-file", LintCmd: "l-file", TypecheckCmd: "tc-file", HiddenTestCmd: "h-file"}
	cfg := DefaultConfig("/r")
	o := &FlagOverrides{
		TestCmd:       CmdOverride{Set: true},
		HiddenTestCmd: CmdOverride{Set: true},
		BuildCmd:      CmdOverride{Cmd: "b-flag", Set: true},
		LintCmd:       CmdOverride{Set: true},
		NoTypecheck:   true,
	}
	if err := MergeFlags(&cfg, vc, o); err != nil {
		t.Fatal(err)
	}
	if cfg.VisibleTest != "t-file" || cfg.HiddenTest != "" || cfg.BuildCmd != "b-flag" || cfg.LintCmd != "" || cfg.TypecheckCmd != "" {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestDryRunPlanListsTypecheckAndChecks(t *testing.T) {
	dir := goldenRepo(t)
	cfg := rulerCfg(dir)
	cfg.DryRun = true
	cfg.TypecheckCmd = "tsc --noEmit"
	cfg.Checks = []config.VerifyCheck{{Name: "vuln", Cmd: "govulncheck ./..."}}
	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Plan.Commands["typecheck"] != "tsc --noEmit" || res.Plan.Commands["check:vuln"] != "govulncheck ./..." {
		t.Fatalf("plan=%v", res.Plan.Commands)
	}
	if res.TypecheckRan || res.Checks != nil {
		t.Fatal("dry-run executed a gate")
	}
}

func TestParseRulers(t *testing.T) {
	cases := []struct {
		in      string
		want    []string
		wantErr bool
	}{
		{"", []string{"tests"}, false},
		{"tests", []string{"tests"}, false},
		{" tests , tests ", []string{"tests"}, false},
		{"visual", []string{"visual"}, false},
		{"foo", nil, true},
		{"coverage", nil, true},
		{"tests,", nil, true},
		{"visual,tests", nil, true},
		{"tests,visual", nil, true},
	}
	for _, c := range cases {
		got, err := ParseRulers(c.in)
		if c.wantErr {
			if !IsUsageError(err) {
				t.Errorf("%q: err=%v, want UsageError", c.in, err)
			}
			continue
		}
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("%q: got=%v err=%v", c.in, got, err)
		}
	}
	if _, err := ParseRulers("foo"); err == nil || err.Error() != `invalid ruler "foo": tests or visual` {
		t.Fatalf("message=%v", err)
	}
}

func TestMergeVerdictsAndExitCode(t *testing.T) {
	rr := func(v Verdict, k SkipKind, code int) *RulerResult {
		return &RulerResult{Verdict: v, SkipKind: k, ExitCode: code}
	}
	na := rr(VerdictSkipped, SkipNotApplicable, 0)
	cases := []struct {
		in       []*RulerResult
		verdict  Verdict
		kind     SkipKind
		exitCode int
	}{
		{[]*RulerResult{rr(VerdictPass, "", 0), na}, VerdictPass, "", 0},
		{[]*RulerResult{na, na}, VerdictSkipped, SkipNotApplicable, 0},
		{[]*RulerResult{rr(VerdictPass, "", 0), rr(VerdictSkipped, SkipUnavailable, 0)}, VerdictSkipped, SkipUnavailable, 0},
		{[]*RulerResult{rr(VerdictFail, "", 127), rr(VerdictEscalate, "", 1)}, VerdictEscalate, "", 127},
		{[]*RulerResult{rr(VerdictPass, "", 0), rr(VerdictFail, "", 1)}, VerdictFail, "", 1},
	}
	for i, c := range cases {
		v, k := MergeVerdicts(c.in)
		if v != c.verdict || k != c.kind {
			t.Errorf("case %d: %s/%s want %s/%s", i, v, k, c.verdict, c.kind)
		}
		if code := ExitCode(v, c.in); code != c.exitCode {
			t.Errorf("case %d: exit=%d want %d", i, code, c.exitCode)
		}
	}
}

func TestRunRulersVisualAloneSkipsRefs(t *testing.T) {
	dir := t.TempDir() // not a git repository: visual must not resolve refs
	cfg := DefaultConfig(dir)
	multi, err := RunRulers(context.Background(), &cfg, []string{RulerVisual})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := multi.Payload().(*VisualReport); !ok {
		t.Fatalf("payload %T", multi.Payload())
	}
	if multi.Verdict != VerdictSkipped || multi.SkipKind != SkipUnavailable || multi.ExitCode() != 0 {
		t.Fatalf("verdict=%s kind=%s", multi.Verdict, multi.SkipKind)
	}
}

func commitConfig(t *testing.T, dir, yaml string) string {
	t.Helper()
	writeFile(t, filepath.Join(dir, ".toolbelt", "config.yaml"), yaml)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "config")
	return gitRev(t, dir)
}

func TestLoadVerifyConfigFromBase(t *testing.T) {
	dir := goldenRepo(t)
	base := commitConfig(t, dir, "verify:\n  test_cmd: base-test\n  checks:\n    - {name: vuln, cmd: govulncheck ./...}\nenv:\n  pass: [SECRET]\n")
	commitConfig(t, dir, "verify:\n  test_cmd: true\n")
	user := config.DefaultConfig()
	user.Verify.LintCmd = "user-lint"
	user.Verify.Checks = []config.VerifyCheck{{Name: "user", Cmd: "u"}}

	vc, source, warnings, err := LoadVerifyConfig(dir, base, user)
	if err != nil {
		t.Fatal(err)
	}
	if source != ConfigSourceBase || len(warnings) != 0 {
		t.Fatalf("source=%s warnings=%v", source, warnings)
	}
	if vc.TestCmd != "base-test" || vc.LintCmd != "user-lint" {
		t.Fatalf("vc=%+v", vc)
	}
	if len(vc.Checks) != 1 || vc.Checks[0].Name != "vuln" {
		t.Fatalf("checks=%+v (base list replaces the user list)", vc.Checks)
	}
	if len(user.Verify.Checks) != 1 || user.Verify.Checks[0].Name != "user" {
		t.Fatal("LoadVerifyConfig mutated the user config")
	}
}

func TestLoadVerifyConfigWorktreeFallback(t *testing.T) {
	dir := goldenRepo(t)
	base := gitRev(t, dir)
	writeFile(t, filepath.Join(dir, ".toolbelt", "config.yaml"), "verify:\n  test_cmd: wt-test\n")
	vc, source, warnings, err := LoadVerifyConfig(dir, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	if source != ConfigSourceWorktree || len(warnings) != 1 || vc.TestCmd != "wt-test" {
		t.Fatalf("no base file: source=%s warnings=%v vc=%+v", source, warnings, vc)
	}

	head := commitConfig(t, dir, "verify:\n  test_cmd: committed\n")
	writeFile(t, filepath.Join(dir, ".toolbelt", "config.yaml"), "verify:\n  test_cmd: dirty\n")
	vc, source, warnings, err = LoadVerifyConfig(dir, head, nil)
	if err != nil {
		t.Fatal(err)
	}
	if source != ConfigSourceWorktree || len(warnings) != 1 || vc.TestCmd != "dirty" {
		t.Fatalf("base==HEAD dirty: source=%s warnings=%v vc=%+v", source, warnings, vc)
	}
}

func TestLoadVerifyConfigErrors(t *testing.T) {
	dir := goldenRepo(t)
	bad := commitConfig(t, dir, "verify: [unclosed\n")
	if _, _, _, err := LoadVerifyConfig(dir, bad, nil); err == nil || !strings.Contains(err.Error(), "malformed at base") {
		t.Fatalf("malformed base: err=%v", err)
	}
	dup := commitConfig(t, dir, "verify:\n  checks:\n    - {name: a, cmd: x}\n    - {name: a, cmd: y}\n")
	if _, _, _, err := LoadVerifyConfig(dir, dup, nil); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate check: err=%v", err)
	}
	empty := commitConfig(t, dir, "verify:\n  checks:\n    - {name: a, cmd: ''}\n")
	if _, _, _, err := LoadVerifyConfig(dir, empty, nil); err == nil {
		t.Fatal("empty check cmd accepted")
	}
	if _, _, _, err := LoadVerifyConfig(dir, "0123456789abcdef0123456789abcdef01234567", nil); err == nil {
		t.Fatal("unknown base ref must be an error, not a missing file")
	}
}

func TestRunRulersReportsConfigSource(t *testing.T) {
	dir := goldenRepo(t)
	commitConfig(t, dir, "verify:\n  test_cmd: 'true'\n  typecheck_cmd: 'echo tc-from-base; exit 1'\n")
	cfg := rulerCfg(dir)
	cfg.VisibleTest = "false"
	cfg.TrustedConfig = config.DefaultConfig()
	multi, err := RunRulers(context.Background(), &cfg, []string{RulerTests})
	if err != nil {
		t.Fatal(err)
	}
	res := multi.Payload().(*Result)
	if res.ConfigSource != ConfigSourceBase || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "base resolves to head") {
		t.Fatalf("source=%s warnings=%v", res.ConfigSource, res.Warnings)
	}
	if res.Verdict != VerdictEscalate || !strings.Contains(res.Reason, "tc-from-base") {
		t.Fatalf("verdict=%s reason=%q", res.Verdict, res.Reason)
	}
	raw, _ := json.Marshal(res)
	if !strings.Contains(string(raw), `"config_source":"base"`) {
		t.Fatalf("json=%s", raw)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
