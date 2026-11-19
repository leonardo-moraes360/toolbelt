package registry

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEgmToolsRequireAbsoluteCwd(t *testing.T) {
	for _, name := range []string{"tb_egm_status", "tb_egm_resolve"} {
		res, err := Default().Call(context.Background(), name, map[string]any{"cwd": "rel", "doc": "x.md"})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError || !strings.Contains(res.Content[0].Text, "absolute") {
			t.Fatalf("%s accepted a relative cwd: %+v", name, res)
		}
	}
}

func TestEgmStatusTool(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs", "adr"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "adr", "001-x.md"), []byte("# ADR\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "commit", "-qm", "x"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	for _, at := range []string{"", "HEAD"} {
		res, err := Default().Call(context.Background(), "tb_egm_status", map[string]any{"cwd": dir, "doc": "docs/adr/001-x.md", "at": at})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError || !strings.Contains(res.Content[0].Text, `"NO_STATE"`) {
			t.Fatalf("at=%q: %s", at, res.Content[0].Text)
		}
	}
}
