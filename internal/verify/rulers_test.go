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
	head := commitConfig(t, dir, "verify:\n  test_cmd: true\n")
	user := config.DefaultConfig()
	user.Verify.LintCmd = "user-lint"
	user.Verify.Checks = []config.VerifyCheck{{Name: "user", Cmd: "u"}}
	user.Env.Allow = []string{"USER_OK"}

	rc, err := LoadVerifyConfig(dir, base, head, user, false)
	if err != nil {
		t.Fatal(err)
	}
	if rc.Source != "" {
		t.Fatalf("source=%q, want omitted when the base file is used", rc.Source)
	}
	if rc.Verify.TestCmd != "base-test" || rc.Verify.LintCmd != "user-lint" {
		t.Fatalf("vc=%+v", rc.Verify)
	}
	if len(rc.Verify.Checks) != 1 || rc.Verify.Checks[0].Name != "vuln" {
		t.Fatalf("checks=%+v (base list replaces the user list)", rc.Verify.Checks)
	}
	if len(rc.Env.Pass) != 1 || rc.Env.Pass[0] != "SECRET" || len(rc.Env.Allow) != 1 {
		t.Fatalf("env=%+v", rc.Env)
	}
	// The head commit changed the file; the working tree matches head, so
	// every changed key is reported as ignored.
	want := "ignored .toolbelt/config.yaml keys that differ from the base ref: verify.checks, verify.test_cmd, env.pass"
	if len(rc.Warnings) != 1 || !strings.HasPrefix(rc.Warnings[0], want) {
		t.Fatalf("warnings=%v", rc.Warnings)
	}
	if len(user.Verify.Checks) != 1 || user.Verify.Checks[0].Name != "user" {
		t.Fatal("LoadVerifyConfig mutated the user config")
	}
}

func TestLoadVerifyConfigIgnoresWorktree(t *testing.T) {
	dir := goldenRepo(t)
	base := gitRev(t, dir)
	user := config.DefaultConfig()
	user.Verify.TestCmd = "exit 1"
	writeFile(t, filepath.Join(dir, ".toolbelt", "config.yaml"), "verify:\n  test_cmd: wt-test\nenv:\n  pass: [GITHUB_TOKEN]\n")
	rc, err := LoadVerifyConfig(dir, base, base, user, false)
	if err != nil {
		t.Fatal(err)
	}
	if rc.Source != ConfigSourceNone || rc.Verify.TestCmd != "exit 1" || len(rc.Env.Pass) != 0 {
		t.Fatalf("no base file: rc=%+v", rc)
	}
	joined := strings.Join(rc.Warnings, "\n")
	if !strings.Contains(joined, "base ref: verify.test_cmd, env.pass") || !strings.Contains(joined, "base resolves to head") {
		t.Fatalf("warnings=%v", rc.Warnings)
	}

	head := commitConfig(t, dir, "verify:\n  test_cmd: committed\n")
	writeFile(t, filepath.Join(dir, ".toolbelt", "config.yaml"), "verify:\n  test_cmd: dirty\n")
	rc, err = LoadVerifyConfig(dir, head, head, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if rc.Source != "" || rc.Verify.TestCmd != "committed" || len(rc.Warnings) != 2 {
		t.Fatalf("base==HEAD dirty: rc=%+v", rc)
	}

	// Worktree equal to base: no warnings beyond base == head.
	runGit(t, dir, "checkout", "--", ".toolbelt/config.yaml")
	rc, err = LoadVerifyConfig(dir, head, head, nil, false)
	if err != nil || len(rc.Warnings) != 1 {
		t.Fatalf("clean: rc=%+v err=%v", rc, err)
	}
}

func TestLoadVerifyConfigExplicitSkipsRepo(t *testing.T) {
	dir := goldenRepo(t)
	base := commitConfig(t, dir, "verify:\n  test_cmd: base-test\nenv:\n  pass: [SECRET]\n")
	writeFile(t, filepath.Join(dir, ".toolbelt", "config.yaml"), "verify:\n  test_cmd: wt\n")
	user := config.DefaultConfig()
	user.Verify.TestCmd = "from-c"
	rc, err := LoadVerifyConfig(dir, base, base, user, true)
	if err != nil {
		t.Fatal(err)
	}
	if rc.Verify.TestCmd != "from-c" || rc.Source != "" || len(rc.Warnings) != 0 || len(rc.Env.Pass) != 0 {
		t.Fatalf("-c: rc=%+v", rc)
	}
}

func TestLoadVerifyConfigErrors(t *testing.T) {
	dir := goldenRepo(t)
	load := func(ref string) error {
		_, err := LoadVerifyConfig(dir, ref, ref, nil, false)
		return err
	}
	bad := commitConfig(t, dir, "verify: [unclosed\n")
	if err := load(bad); err == nil || !strings.Contains(err.Error(), "malformed at base") {
		t.Fatalf("malformed base: err=%v", err)
	}
	dup := commitConfig(t, dir, "verify:\n  checks:\n    - {name: a, cmd: x}\n    - {name: a, cmd: y}\n")
	if err := load(dup); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate check: err=%v", err)
	}
	empty := commitConfig(t, dir, "verify:\n  checks:\n    - {name: a, cmd: ''}\n")
	if err := load(empty); err == nil {
		t.Fatal("empty check cmd accepted")
	}
	if err := load("0123456789abcdef0123456789abcdef01234567"); err == nil {
		t.Fatal("unknown base ref must be an error, not a missing file")
	}
}

func TestLoadVerifyConfigSymlinkAtBaseIgnored(t *testing.T) {
	dir := goldenRepo(t)
	writeFile(t, filepath.Join(dir, "real.yaml"), "verify:\n  test_cmd: from-link\n")
	if err := os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../real.yaml", filepath.Join(dir, ".toolbelt", "config.yaml")); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "link")
	base := gitRev(t, dir)
	user := config.DefaultConfig()
	user.Verify.TestCmd = "global-test"
	rc, err := LoadVerifyConfig(dir, base, base, user, false)
	if err != nil {
		t.Fatal(err)
	}
	if rc.Source != ConfigSourceNone || rc.Verify.TestCmd != "global-test" {
		t.Fatalf("symlink must not be followed: rc=%+v", rc)
	}
	if !slices.Contains(rc.Warnings, SymlinkConfigWarning) {
		t.Fatalf("warnings=%v", rc.Warnings)
	}
	// -c does not read the repository config at all.
	rc, err = LoadVerifyConfig(dir, base, base, user, true)
	if err != nil || len(rc.Warnings) != 0 {
		t.Fatalf("-c: rc=%+v err=%v", rc, err)
	}
}

func TestLoadVerifyConfigMalformedBaseWithExplicit(t *testing.T) {
	dir := goldenRepo(t)
	bad := commitConfig(t, dir, "verify: [unclosed\n")
	if _, err := LoadVerifyConfig(dir, bad, bad, nil, true); err != nil {
		t.Fatalf("-c must not read the repository config: %v", err)
	}
}

// From a subdirectory, the ignored-keys warning still compares the file at
// the repository top level.
func TestLoadVerifyConfigFromSubdir(t *testing.T) {
	dir := goldenRepo(t)
	base := commitConfig(t, dir, "verify:\n  test_cmd: base-test\n")
	writeFile(t, filepath.Join(dir, ".toolbelt", "config.yaml"), "verify:\n  test_cmd: wt\n")
	sub := filepath.Join(dir, "sub", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	rc, err := LoadVerifyConfig(sub, base, base, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	want := "ignored .toolbelt/config.yaml keys that differ from the base ref: verify.test_cmd (verify reads the base ref)"
	if !slices.Contains(rc.Warnings, want) {
		t.Fatalf("warnings=%v", rc.Warnings)
	}
}

func TestMergeFlagsAppliedOverrides(t *testing.T) {
	vc := &config.VerifyConfig{
		TestCmd: "t-file", TypecheckCmd: "tc-file", LintCmd: "l-file",
		Checks: []config.VerifyCheck{{Name: "govulncheck", Cmd: "g"}, {Name: "b", Cmd: "b1"}},
	}
	cfg := DefaultConfig("/r")
	o := &FlagOverrides{
		TestCmd:     CmdOverride{Cmd: "t-flag", Set: true},
		LintCmd:     CmdOverride{Cmd: "l-file", Set: true}, // same value: not an override
		BuildCmd:    CmdOverride{Cmd: "b-flag", Set: true},
		NoTypecheck: true,
		Checks:      []config.VerifyCheck{{Name: "b", Cmd: "b1"}, {Name: "c", Cmd: "c1"}},
		SkipChecks:  []string{"govulncheck"},
	}
	if err := MergeFlags(&cfg, vc, o); err != nil {
		t.Fatal(err)
	}
	want := []string{"build_cmd", "checks:c", "no_typecheck", "skip_checks:govulncheck", "test_cmd"}
	if !slices.Equal(cfg.AppliedOverrides, want) {
		t.Fatalf("overrides=%v, want %v", cfg.AppliedOverrides, want)
	}

	cfg = DefaultConfig("/r")
	if err := MergeFlags(&cfg, vc, &FlagOverrides{NoChecks: true, NoBuild: true}); err != nil {
		t.Fatal(err)
	}
	// no_build disabled nothing configured.
	if !slices.Equal(cfg.AppliedOverrides, []string{"no_checks"}) {
		t.Fatalf("overrides=%v", cfg.AppliedOverrides)
	}
	cfg = DefaultConfig("/r")
	if err := MergeFlags(&cfg, vc, &FlagOverrides{}); err != nil || cfg.AppliedOverrides != nil {
		t.Fatalf("no overrides: %v err=%v", cfg.AppliedOverrides, err)
	}
}

func TestRunRulersReportsOverrides(t *testing.T) {
	dir := goldenRepo(t)
	commitConfig(t, dir, "verify:\n  test_cmd: 'false'\n")
	cfg := rulerCfg(dir)
	cfg.TrustedConfig = config.DefaultConfig()
	cfg.Overrides = &FlagOverrides{TestCmd: CmdOverride{Cmd: "true", Set: true}}
	multi, err := RunRulers(context.Background(), &cfg, []string{RulerTests})
	if err != nil {
		t.Fatal(err)
	}
	res := multi.Payload().(*Result)
	if !slices.Equal(res.Overrides, []string{"test_cmd"}) || !slices.Equal(multi.Overrides, res.Overrides) {
		t.Fatalf("overrides=%v multi=%v", res.Overrides, multi.Overrides)
	}
	raw, _ := json.Marshal(res)
	if !strings.Contains(string(raw), `"overrides":["test_cmd"]`) {
		t.Fatalf("json=%s", raw)
	}
}

func TestRunRulersConfigFromBase(t *testing.T) {
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
	if res.ConfigSource != "" || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "base resolves to head") {
		t.Fatalf("source=%s warnings=%v", res.ConfigSource, res.Warnings)
	}
	if res.Verdict != VerdictEscalate || !strings.Contains(res.Reason, "tc-from-base") {
		t.Fatalf("verdict=%s reason=%q", res.Verdict, res.Reason)
	}
	raw, _ := json.Marshal(res)
	if strings.Contains(string(raw), `"config_source"`) {
		t.Fatalf("json=%s", raw)
	}
}

func TestMergeFlagsBadCheckIsUsageError(t *testing.T) {
	for _, checks := range [][]config.VerifyCheck{
		{{Name: "Bad Name", Cmd: "x"}},
		{{Name: "a", Cmd: "x"}, {Name: "a", Cmd: "y"}},
	} {
		cfg := DefaultConfig(t.TempDir())
		err := MergeFlags(&cfg, nil, &FlagOverrides{Checks: checks, NoChecks: true})
		if !IsUsageError(err) {
			t.Fatalf("checks=%v: err=%v, want a usage error", checks, err)
		}
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
