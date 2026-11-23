package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		if err := c.Run(); err != nil {
			t.Fatal(err)
		}
	}
	run("git", "init")
	run("git", "config", "user.email", "t@t")
	run("git", "config", "user.name", "t")
	return dir
}

func commitAll(t *testing.T, dir, msg string) string {
	t.Helper()
	run := exec.Command("git", "add", "-A")
	run.Dir = dir
	if err := run.Run(); err != nil {
		t.Fatal(err)
	}
	run = exec.Command("git", "commit", "--allow-empty", "-m", msg)
	run.Dir = dir
	if err := run.Run(); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out[:len(out)-1])
}

func hasViolation(res *CheckResult, typ ViolationType, file string) bool {
	for _, v := range res.Violations {
		if v.Type == typ && v.File == file {
			return true
		}
	}
	return false
}

func TestDetectTestDeletion(t *testing.T) {
	dir := setupRepo(t)
	_ = os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("package x\n"), 0o644)
	run := exec.Command("git", "add", "a_test.go")
	run.Dir = dir
	_ = run.Run()
	run = exec.Command("git", "commit", "-m", "a")
	run.Dir = dir
	_ = run.Run()
	baseOut, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	base := string(baseOut[:len(baseOut)-1])
	_ = os.Remove(filepath.Join(dir, "a_test.go"))
	run = exec.Command("git", "add", "-A")
	run.Dir = dir
	_ = run.Run()
	run = exec.Command("git", "commit", "-m", "del")
	run.Dir = dir
	_ = run.Run()
	headOut, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	head := string(headOut[:len(headOut)-1])

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Violations) == 0 {
		t.Fatal("esperava violação")
	}
}

func TestDefaultsWhenGuardYAMLAbsent(t *testing.T) {
	dir := setupRepo(t)
	_ = os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package x\n"), 0o644)
	base := commitAll(t, dir, "base")

	_ = os.MkdirAll(filepath.Join(dir, "tests", "hidden"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "tests", "hidden", "sentinel.js"), []byte("1\n"), 0o644)
	head := commitAll(t, dir, "touch hidden")

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !hasViolation(res, ViolationProtectedPath, "tests/hidden/sentinel.js") {
		t.Fatalf("esperava protected_path default; got %#v", res.Violations)
	}
}

func TestLoadProtectedPathsFromGuardYAML(t *testing.T) {
	dir := setupRepo(t)
	_ = os.MkdirAll(filepath.Join(dir, "lib"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "lib", "help.js"), []byte("exports.x=1\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths:\n  - lib/help.js\n"), 0o644)
	base := commitAll(t, dir, "base")

	_ = os.WriteFile(filepath.Join(dir, "lib", "help.js"), []byte("exports.x=2\n"), 0o644)
	head := commitAll(t, dir, "edit help")

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !hasViolation(res, ViolationProtectedPath, "lib/help.js") {
		t.Fatalf("esperava protected_path de guard.yaml; got %#v", res.Violations)
	}
}

func TestGuardYAMLPatternObjectMergedWithDefaults(t *testing.T) {
	dir := setupRepo(t)
	_ = os.MkdirAll(filepath.Join(dir, "src", "click"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "src", "click", "core.py"), []byte("x=1\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package x\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	yaml := "protected_paths:\n  - pattern: src/click/core.py\n    reason: core\n"
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte(yaml), 0o644)
	base := commitAll(t, dir, "base")

	_ = os.WriteFile(filepath.Join(dir, "src", "click", "core.py"), []byte("x=2\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "tests", "hidden"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "tests", "hidden", "s.js"), []byte("1\n"), 0o644)
	head := commitAll(t, dir, "touch both")

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !hasViolation(res, ViolationProtectedPath, "src/click/core.py") {
		t.Fatalf("esperava path custom; got %#v", res.Violations)
	}
	if !hasViolation(res, ViolationProtectedPath, "tests/hidden/s.js") {
		t.Fatalf("esperava default preservado; got %#v", res.Violations)
	}
}

func TestMalformedGuardYAMLFailsClosed(t *testing.T) {
	dir := setupRepo(t)
	_ = os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package x\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths: [\n"), 0o644)
	base := commitAll(t, dir, "base")

	_ = os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package x\nfunc X(){}\n"), 0o644)
	head := commitAll(t, dir, "edit")

	chk := NewChecker(dir)
	_, err := chk.Check(base, head)
	if err == nil {
		t.Fatal("esperava erro fail-closed para guard.yaml malformado")
	}
	if !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("mensagem deveria citar malformado: %v", err)
	}
}

func TestLoadProtectedPathsFromBase(t *testing.T) {
	dir := setupRepo(t)
	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package x\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths:\n  - lib/help.js\n"), 0o644)
	base := commitAll(t, dir, "base")

	chk := NewChecker(dir)
	got, err := chk.loadProtectedPaths(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(defaultProtectedPaths)+1 {
		t.Fatalf("merge: got %d want %d+1; %#v", len(got), len(defaultProtectedPaths), got)
	}
	last := got[len(got)-1]
	if last.pattern != "lib/help.js" || !last.custom {
		t.Fatalf("custom no fim: got %#v", last)
	}
}

func TestProtectedPathSourceDefaultVsCustom(t *testing.T) {
	dir := setupRepo(t)
	_ = os.MkdirAll(filepath.Join(dir, "lib"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "lib", "help.js"), []byte("1\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths:\n  - lib/help.js\n"), 0o644)
	base := commitAll(t, dir, "base")

	_ = os.WriteFile(filepath.Join(dir, "lib", "help.js"), []byte("2\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "thresholds.yaml"), []byte("x: 1\n"), 0o644)
	head := commitAll(t, dir, "touch both")

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	var custom, def bool
	for _, v := range res.Violations {
		if v.Type != ViolationProtectedPath {
			continue
		}
		switch v.File {
		case "lib/help.js":
			if v.Source != SourceCustom {
				t.Fatalf("custom source=%q", v.Source)
			}
			custom = true
		case ".toolbelt/thresholds.yaml":
			if v.Source != SourceDefault {
				t.Fatalf("default source=%q", v.Source)
			}
			def = true
		}
	}
	if !custom || !def {
		t.Fatalf("custom=%v default=%v violations=%#v", custom, def, res.Violations)
	}
}

func TestDirectoryTrailingSlashProtectsChildren(t *testing.T) {
	dir := setupRepo(t)
	_ = os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package x\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths:\n  - secrets/\n"), 0o644)
	base := commitAll(t, dir, "base")

	_ = os.MkdirAll(filepath.Join(dir, "secrets", "nested"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "secrets", "token"), []byte("s3cret\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "secrets", "nested", "key"), []byte("k\n"), 0o644)
	head := commitAll(t, dir, "add secrets")

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !hasViolation(res, ViolationProtectedPath, "secrets/token") {
		t.Fatalf("secrets/ deveria proteger secrets/token; got %#v", res.Violations)
	}
	if !hasViolation(res, ViolationProtectedPath, "secrets/nested/key") {
		t.Fatalf("secrets/ deveria proteger secrets/nested/key; got %#v", res.Violations)
	}
}

func TestCheckRejectsOptionInjection(t *testing.T) {
	dir := setupRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	commitAll(t, dir, "init")
	commitAll(t, dir, "empty")

	outDir := t.TempDir()
	injected := filepath.Join(outDir, "pwned")
	_, err := NewChecker(dir).Check("--output="+injected, "HEAD")
	if err == nil {
		t.Fatal("ref com '-' deveria ser rejeitada")
	}
	if _, statErr := os.Stat(injected); statErr == nil {
		t.Fatal("injeção não deveria criar o arquivo alvo")
	}
	if _, statErr := os.Stat(injected + "..HEAD"); statErr == nil {
		t.Fatal("injeção não deveria criar arquivo via --output=<path>..<head>")
	}
}

func TestCheckRejectsMissingSHA(t *testing.T) {
	dir := setupRepo(t)
	commitAll(t, dir, "init")
	fake := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	_, err := NewChecker(dir).Check(fake, "HEAD")
	if err == nil {
		t.Fatal("SHA inexistente deveria falhar")
	}
	if !strings.Contains(err.Error(), "missing or invalid") {
		t.Fatalf("erro claro esperado: %v", err)
	}
}

func TestIsProtectedDirectoryPrefix(t *testing.T) {
	c := &Checker{protectedPaths: []protectedEntry{
		{pattern: "secrets/"},
		{pattern: "lib/help.js"},
		{pattern: "tests/hidden/*"},
	}}
	cases := []struct {
		path string
		want bool
	}{
		{"secrets/token", true},
		{"secrets/a/b", true},
		{"secrets", true},
		{"secretsfoo", false},
		{"lib/help.js", true},
		{"lib/other.js", false},
		{"tests/hidden/x", true},
		{"tests/hidden", true},
		{"tests/hiddenX", false},
		{"ok.go", false},
	}
	for _, tc := range cases {
		if _, got := c.protectedMatch(tc.path); got != tc.want {
			t.Fatalf("protectedMatch(%q)=%v want %v", tc.path, got, tc.want)
		}
	}
}

func TestRenameProtectedPathDetected(t *testing.T) {
	dir := setupRepo(t)
	_ = os.MkdirAll(filepath.Join(dir, "tests", "hidden"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "tests", "hidden", "secret.js"), []byte("s\n"), 0o644)
	base := commitAll(t, dir, "base")

	_ = os.MkdirAll(filepath.Join(dir, "tests", "visible"), 0o755)
	run := exec.Command("git", "mv", "tests/hidden/secret.js", "tests/visible/secret.js")
	run.Dir = dir
	if err := run.Run(); err != nil {
		t.Fatal(err)
	}
	head := commitAll(t, dir, "rename bypass")

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !hasViolation(res, ViolationProtectedPath, "tests/hidden/secret.js") {
		t.Fatalf("rename da origem protegida deveria violar; got %#v", res.Violations)
	}
}

func TestProtectedPathCaseInsensitive(t *testing.T) {
	dir := setupRepo(t)
	_ = os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package x\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths:\n  - .github/\n"), 0o644)
	base := commitAll(t, dir, "base")

	_ = os.MkdirAll(filepath.Join(dir, ".GITHUB", "workflows"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".GITHUB", "workflows", "evil.yml"), []byte("x\n"), 0o644)
	head := commitAll(t, dir, "case bypass")

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !hasViolation(res, ViolationProtectedPath, ".GITHUB/workflows/evil.yml") {
		t.Fatalf("match case-insensitive deveria pegar .GITHUB/; got %#v", res.Violations)
	}
}

func TestGuardYAMLEditIsViolation(t *testing.T) {
	dir := setupRepo(t)
	_ = os.MkdirAll(filepath.Join(dir, "lib"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "lib", "help.js"), []byte("1\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths:\n  - lib/help.js\n"), 0o644)
	base := commitAll(t, dir, "base")

	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths: []\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "lib", "help.js"), []byte("hacked\n"), 0o644)
	head := commitAll(t, dir, "weaken")

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !hasViolation(res, ViolationProtectedPath, ".toolbelt/guard.yaml") {
		t.Fatalf("editar guard.yaml deveria violar; got %#v", res.Violations)
	}
	if !hasViolation(res, ViolationProtectedPath, "lib/help.js") {
		t.Fatalf("proteção da BASE ainda deveria valer; got %#v", res.Violations)
	}
}

func TestGlobDoubleStarMatchesAndInvalidNotCounted(t *testing.T) {
	dir := setupRepo(t)
	_ = os.MkdirAll(filepath.Join(dir, "src", "a", "b"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "src", "a", "b", "secret.js"), []byte("1\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package x\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	yaml := "protected_paths:\n  - \"src/**/*.js\"\n  - \"[unclosed\"\n"
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte(yaml), 0o644)
	base := commitAll(t, dir, "base")

	_ = os.WriteFile(filepath.Join(dir, "src", "a", "b", "secret.js"), []byte("2\n"), 0o644)
	head := commitAll(t, dir, "edit nested")

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !hasViolation(res, ViolationProtectedPath, "src/a/b/secret.js") {
		t.Fatalf("glob ** deveria casar; got %#v", res.Violations)
	}
	wantRules := len(defaultProtectedPaths) + 1
	if res.RulesChecked != wantRules {
		t.Fatalf("rules_checked=%d want %d (sem regra inválida)", res.RulesChecked, wantRules)
	}
}

func TestScanUsesDiffNotWorkingTree(t *testing.T) {
	dir := setupRepo(t)
	_ = os.WriteFile(filepath.Join(dir, "skip_test.go"), []byte("package x\n"), 0o644)
	base := commitAll(t, dir, "base")

	content := "package x\nimport \"testing\"\nfunc TestA(t *testing.T) { t.Skip(\"cheat\") }\n"
	_ = os.WriteFile(filepath.Join(dir, "skip_test.go"), []byte(content), 0o644)
	head := commitAll(t, dir, "add skip")

	clean := "package x\nimport \"testing\"\nfunc TestA(t *testing.T) {}\n"
	_ = os.WriteFile(filepath.Join(dir, "skip_test.go"), []byte(clean), 0o644)

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !hasViolation(res, ViolationTestSkip, "skip_test.go") {
		t.Fatalf("deveria ler o conteúdo do commit head, não do working tree; got %#v", res.Violations)
	}
}

func TestDetectJSTSTestWeakening(t *testing.T) {
	dir := setupRepo(t)
	_ = os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "src", "app.test.js"), []byte("test('ok', () => expect(1).toBe(2));\n"), 0o644)
	base := commitAll(t, dir, "base")

	weak := strings.Join([]string{
		`describe.skip("suite", () => {});`,
		`it.skip("case", () => {});`,
		`test.skip("case2", () => {});`,
		`xit("old", () => {});`,
		`it.only("focus", () => {});`,
		`test("trivial", () => expect(true).toBe(true));`,
		`test("num", () => expect(1).toBe(1));`,
	}, "\n") + "\n"
	_ = os.WriteFile(filepath.Join(dir, "src", "app.test.js"), []byte(weak), 0o644)
	head := commitAll(t, dir, "weaken js")

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	var skips, trivials int
	for _, v := range res.Violations {
		switch v.Type {
		case ViolationTestSkip:
			skips++
		case ViolationTrivialAssert:
			trivials++
		}
	}
	if skips < 5 {
		t.Fatalf("esperava >=5 test_skip (skip/only/xit); got %d em %#v", skips, res.Violations)
	}
	if trivials < 2 {
		t.Fatalf("esperava >=2 trivial_assert; got %d em %#v", trivials, res.Violations)
	}
}

func TestParseDiffRenameBothPaths(t *testing.T) {
	diff := "R100\ttests/hidden/a.js\ttests/visible/a.js\nM\tok.go\n"
	got := parseDiff(diff)
	if len(got) != 3 {
		t.Fatalf("got %#v", got)
	}
	if got[0].path != "tests/hidden/a.js" || !got[0].deleted {
		t.Fatalf("origem: %#v", got[0])
	}
	if got[1].path != "tests/visible/a.js" || got[1].deleted {
		t.Fatalf("destino: %#v", got[1])
	}
}

func TestMatchGlobDoubleStarAndCase(t *testing.T) {
	if !matchProtectedPattern("src/**/*.js", "src/a/b/c.js") {
		t.Fatal("** deveria casar nested")
	}
	if !matchProtectedPattern(".github/", ".GITHUB/workflows/x.yml") {
		t.Fatal("case-insensitive directory")
	}
	if matchProtectedPattern("[unclosed", "x") {
		t.Fatal("padrão inválido não deve casar")
	}
	if validProtectedPattern("[unclosed") {
		t.Fatal("[unclosed deveria ser inválido")
	}
}

func gitMv(t *testing.T, dir, from, to string) {
	t.Helper()
	c := exec.Command("git", "mv", from, to)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git mv: %v\n%s", err, out)
	}
}

func hasViolationType(res *CheckResult, typ ViolationType) bool {
	for _, v := range res.Violations {
		if v.Type == typ {
			return true
		}
	}
	return false
}

func TestDirectoryRenameIsNotTestDeletion(t *testing.T) {
	dir := setupRepo(t)
	_ = os.MkdirAll(filepath.Join(dir, "server-go", "tests"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "server-go", "a_test.go"), []byte("package x\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "server-go", "tests", "b.go"), []byte("package x\n"), 0o644)
	base := commitAll(t, dir, "base")
	gitMv(t, dir, "server-go", "server")
	head := commitAll(t, dir, "move")

	res, err := NewChecker(dir).Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if hasViolationType(res, ViolationTestDeletion) {
		t.Fatalf("rename of tests is not a deletion: %#v", res.Violations)
	}
}

func TestRenameTestToNonTestIsDeletion(t *testing.T) {
	dir := setupRepo(t)
	_ = os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte("package x\n"), 0o644)
	base := commitAll(t, dir, "base")
	gitMv(t, dir, "foo_test.go", "foo_test.go.bak")
	head := commitAll(t, dir, "disable")

	res, err := NewChecker(dir).Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !hasViolation(res, ViolationTestDeletion, "foo_test.go") {
		t.Fatalf("test renamed to non-test must count as deletion: %#v", res.Violations)
	}
}

func TestParseDiffCopyKeepsSource(t *testing.T) {
	got := parseDiff("C100\ta_test.go\tb_test.go\n")
	if len(got) != 1 || got[0].path != "b_test.go" || got[0].deleted || got[0].testRemoved {
		t.Fatalf("copy must only report the destination: %#v", got)
	}
}

func TestLooseRenamesOfTestsAreDeletions(t *testing.T) {
	for _, c := range []struct{ src, dst string }{
		{"x/foo_test.go", "x/_foo_test.go"},
		{"x/bar_test.go", "x/testdata/bar_test.go"},
		{"x/baz_test.go", "x/test/baz_test.go.off"},
	} {
		dir := setupRepo(t)
		_ = os.MkdirAll(filepath.Join(dir, filepath.Dir(c.src)), 0o755)
		_ = os.MkdirAll(filepath.Join(dir, filepath.Dir(c.dst)), 0o755)
		_ = os.WriteFile(filepath.Join(dir, c.src), []byte("package x\n"), 0o644)
		base := commitAll(t, dir, "base")
		gitMv(t, dir, c.src, c.dst)
		head := commitAll(t, dir, "move")
		res, err := NewChecker(dir).Check(base, head)
		if err != nil {
			t.Fatal(err)
		}
		if !hasViolation(res, ViolationTestDeletion, c.src) {
			t.Errorf("%s -> %s must count as test deletion: %#v", c.src, c.dst, res.Violations)
		}
	}
}

func TestNonASCIITestDeletionDetected(t *testing.T) {
	dir := setupRepo(t)
	_ = os.WriteFile(filepath.Join(dir, "ação_test.go"), []byte("package x\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "tests"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "tests", "a.py"), []byte("x\n"), 0o644)
	base := commitAll(t, dir, "base")
	_ = os.Remove(filepath.Join(dir, "ação_test.go"))
	_ = os.RemoveAll(filepath.Join(dir, "tests"))
	head := commitAll(t, dir, "del")
	res, err := NewChecker(dir).Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !hasViolation(res, ViolationTestDeletion, "ação_test.go") {
		t.Errorf("non-ASCII test deletion missed: %#v", res.Violations)
	}
	if !hasViolation(res, ViolationTestDeletion, "tests/a.py") {
		t.Errorf("top-level tests/ deletion missed: %#v", res.Violations)
	}
}

func TestRenamesChangingBasenameAreDeletions(t *testing.T) {
	for _, c := range []struct{ src, dst string }{
		{"x/foo_test.go", "x/foo_windows_test.go"},
		{"web/a.test.ts", "web/a.test.off.ts"},
		{"tests/test_foo.py", "tests/foo_disabled.py"},
		{"src/test/FooTest.java", "src/test/Foo.java"},
	} {
		dir := setupRepo(t)
		_ = os.MkdirAll(filepath.Join(dir, filepath.Dir(c.src)), 0o755)
		_ = os.WriteFile(filepath.Join(dir, c.src), []byte("package x\n"), 0o644)
		base := commitAll(t, dir, "base")
		gitMv(t, dir, c.src, c.dst)
		head := commitAll(t, dir, "rename")
		res, err := NewChecker(dir).Check(base, head)
		if err != nil {
			t.Fatal(err)
		}
		if !hasViolation(res, ViolationTestDeletion, c.src) {
			t.Errorf("%s -> %s must count as test deletion: %#v", c.src, c.dst, res.Violations)
		}
	}
}
