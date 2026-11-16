package guard

import (
	"os"
	"os/exec"
	"path/filepath"
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
	base := commitAll(t, dir, "base")

	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths:\n  - lib/help.js\n"), 0o644)
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
	base := commitAll(t, dir, "base")

	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	yaml := "protected_paths:\n  - pattern: src/click/core.py\n    reason: core\n"
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte(yaml), 0o644)
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

func TestInvalidGuardYAMLKeepsDefaults(t *testing.T) {
	dir := setupRepo(t)
	_ = os.WriteFile(filepath.Join(dir, "ok.go"), []byte("package x\n"), 0o644)
	base := commitAll(t, dir, "base")

	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths: [\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "benchmarks"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "benchmarks", "a.txt"), []byte("1\n"), 0o644)
	head := commitAll(t, dir, "bench")

	chk := NewChecker(dir)
	res, err := chk.Check(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !hasViolation(res, ViolationProtectedPath, "benchmarks/a.txt") {
		t.Fatalf("esperava default com yaml inválido; got %#v", res.Violations)
	}
}

func TestLoadProtectedPathsHelpers(t *testing.T) {
	dir := t.TempDir()
	got := loadProtectedPaths(dir)
	if len(got) != len(defaultProtectedPaths) {
		t.Fatalf("sem arquivo: got %v", got)
	}
	for _, e := range got {
		if e.custom {
			t.Fatalf("default marcado como custom: %#v", e)
		}
	}

	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths:\n  - lib/help.js\n  - tests/hidden/*\n"), 0o644)
	got = loadProtectedPaths(dir)
	if len(got) != len(defaultProtectedPaths)+1 {
		t.Fatalf("merge dedupe: got %v", got)
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
	base := commitAll(t, dir, "base")

	_ = os.MkdirAll(filepath.Join(dir, ".toolbelt"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".toolbelt", "guard.yaml"), []byte("protected_paths:\n  - lib/help.js\n"), 0o644)
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
