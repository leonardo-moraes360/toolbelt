package bench

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFakeAgentCommandPortable(t *testing.T) {
	cmd := FakeAgentCommand("")
	if strings.Contains(cmd, "go run") {
		t.Fatalf("fake agent must not use go run: %s", cmd)
	}
	if !strings.Contains(cmd, FakeAgentSubcommand) {
		t.Fatalf("missing hidden subcommand: %s", cmd)
	}
	if !strings.Contains(cmd, "--workdir {workdir}") {
		t.Fatalf("missing workdir placeholder: %s", cmd)
	}
	if !strings.Contains(cmd, "--prompt {prompt}") {
		t.Fatalf("missing prompt placeholder: %s", cmd)
	}
}

func TestFakeAgentCommandWithConfig(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "fake.json")
	cmd := FakeAgentCommand(cfg)
	if !strings.Contains(cmd, "--config") {
		t.Fatalf("expected --config: %s", cmd)
	}
	if !strings.Contains(cmd, shellSingleQuote(cfg)) {
		t.Fatalf("config path not quoted: %s", cmd)
	}
}

func TestShellSingleQuote(t *testing.T) {
	got := shellSingleQuote(`it's`)
	want := `'it'\''s'`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFakeAgentSubcommandViaTBBinary(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	root := findModuleRoot(t)
	binDir := t.TempDir()
	tbBin := filepath.Join(binDir, "tb")
	build := exec.Command("go", "build", "-o", tbBin, "./cmd/tb")
	build.Dir = root
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build tb: %v\n%s", err, out)
	}

	foreign := t.TempDir()
	_ = os.WriteFile(filepath.Join(foreign, "README.md"), []byte("not go\n"), 0o644)
	work := filepath.Join(foreign, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(tbBin, FakeAgentSubcommand, "--workdir", work, "--prompt", "hello")
	cmd.Dir = foreign
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fake agent via tb: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), `"status": "completed"`) && !strings.Contains(string(out), `"status":"completed"`) {
		t.Fatalf("saída inesperada: %s", out)
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod não encontrado")
		}
		dir = parent
	}
}
