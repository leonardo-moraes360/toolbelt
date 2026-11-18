package sandbox

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsSubpath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("caminhos POSIX")
	}
	cases := []struct {
		path, base string
		want       bool
	}{
		{"/home/u/work/a", "/home/u/work", true},
		{"/home/u/work/a/b/c", "/home/u/work", true},
		{"/home/u/work/..x", "/home/u/work", true},
		{"/home/u/work/a/../b", "/home/u/work", true},
		{"/home/u/work", "/home/u/work", false},
		{"/home/u/work/", "/home/u/work", false},
		{"/home/u", "/home/u/work", false},
		{"/home/u/other", "/home/u/work", false},
		{"/home/u/workx", "/home/u/work", false},
		{"/etc/passwd", "/home/u/work", false},
		{"/", "/home/u/work", false},
		{"/home/u/work/../../../etc", "/home/u/work", false},
		{"relative/x", "/home/u/work", false},
		{"/tmp", "/tmp/job", false},
		{"/tmp/job", "/tmp", true},
	}
	for _, c := range cases {
		if got := isSubpath(c.path, c.base); got != c.want {
			t.Errorf("isSubpath(%q, %q) = %v, want %v", c.path, c.base, got, c.want)
		}
	}
}

func TestValidatePathScope(t *testing.T) {
	work := t.TempDir()
	forbiddenAbs := filepath.Join(t.TempDir(), "abs-secret")
	sb, err := New(Config{
		WorkDir:          work,
		ForbiddenPaths:   []string{"secrets", forbiddenAbs, filepath.Join(work, "abs-in")},
		AllowUnsandboxed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		path string
		ok   bool
	}{
		{"workdir", work, true},
		{"filho", filepath.Join(work, "a.txt"), true},
		{"neto", filepath.Join(work, "a", "b.txt"), true},
		{"nome com ..", filepath.Join(work, "..x"), true},
		{"pai", filepath.Dir(work), false},
		{"irmão com prefixo", work + "x", false},
		{"fora", "/etc/passwd", false},
		{"travessia", filepath.Join(work, "..", "..", "etc", "passwd"), false},
		{"proibido relativo", filepath.Join(work, "secrets"), false},
		{"dentro do proibido", filepath.Join(work, "secrets", "k"), false},
		{"vizinho do proibido", filepath.Join(work, "secrets2"), true},
		{"proibido absoluto fora", filepath.Join(forbiddenAbs, "k"), false},
		{"proibido absoluto dentro", filepath.Join(work, "abs-in"), false},
		{"dentro do proibido absoluto", filepath.Join(work, "abs-in", "k"), false},
		{"vizinho do proibido absoluto", filepath.Join(work, "abs-in2"), true},
	}
	for _, c := range cases {
		err := sb.ValidatePath(c.path)
		if (err == nil) != c.ok {
			t.Errorf("%s: ValidatePath(%q) err=%v, want ok=%v", c.name, c.path, err, c.ok)
		}
	}
}
