package registry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
)

// runConfigYAML prepends sandbox.allow_unsandboxed when the host has no
// Landlock, so tb_run tests that are not about the sandbox still run on such
// kernels (tb_run fails closed by default).
func runConfigYAML(extra string) string {
	if ok, _, _ := sandbox.CheckLandlockSupport(); ok {
		return extra
	}
	return "sandbox:\n  allow_unsandboxed: true\n" + extra
}

func requireLandlock(t *testing.T) {
	t.Helper()
	if ok, _, err := sandbox.CheckLandlockSupport(); !ok {
		t.Skipf("Landlock unavailable: %v", err)
	}
}

func runTool(t *testing.T, args map[string]any) (payload map[string]any, errText string) {
	t.Helper()
	res, err := Default().Call(context.Background(), "tb_run", args)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		return nil, res.Content[0].Text
	}
	if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	return payload, ""
}

// tb_run is confined to the repository (M8ven review, finding 8): files
// outside it can be neither read nor written, /tmp included, while the
// child's own TMPDIR works.
func TestTBRunSandboxConfinesFilesystem(t *testing.T) {
	requireLandlock(t)
	useConfig(t, "")
	outside := t.TempDir() // under /tmp: outside the repository
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("s3cr3t-value"), 0o600); err != nil {
		t.Fatal(err)
	}

	payload, errText := runTool(t, map[string]any{
		"command": "cat " + secret + "; touch " + filepath.Join(outside, "new") + " && echo WROTE-OUTSIDE; touch \"$TMPDIR/ok\" && echo TMPDIR-OK",
		"timeout": "10s",
	})
	if errText != "" {
		t.Fatal(errText)
	}
	out, _ := payload["output"].(string)
	if payload["sandboxed"] != true {
		t.Fatalf("want sandboxed:true, got %v", payload)
	}
	if strings.Contains(out, "s3cr3t-value") {
		t.Fatalf("read a file outside the repository:\n%s", out)
	}
	if strings.Contains(out, "WROTE-OUTSIDE") {
		t.Fatalf("wrote outside the repository:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(outside, "new")); err == nil {
		t.Fatal("file created outside the repository")
	}
	if !strings.Contains(out, "TMPDIR-OK") {
		t.Fatalf("own TMPDIR should be writable:\n%s", out)
	}
}

func TestTBRunSandboxConfigReadPath(t *testing.T) {
	requireLandlock(t)
	extra := t.TempDir()
	if err := os.WriteFile(filepath.Join(extra, "data.txt"), []byte("granted"), 0o600); err != nil {
		t.Fatal(err)
	}
	useConfig(t, "sandbox:\n  read: ["+extra+"]\n")
	payload, errText := runTool(t, map[string]any{
		"command": "cat " + filepath.Join(extra, "data.txt") + "; touch " + filepath.Join(extra, "w") + " && echo WROTE",
		"timeout": "10s",
	})
	if errText != "" {
		t.Fatal(errText)
	}
	out, _ := payload["output"].(string)
	if !strings.Contains(out, "granted") || strings.Contains(out, "WROTE") {
		t.Fatalf("sandbox.read must be readable and not writable:\n%s", out)
	}
}

func TestTBRunRejectsCwdOutsideRepository(t *testing.T) {
	useConfig(t, runConfigYAML(""))
	for _, cwd := range []string{t.TempDir(), "../../.."} { // test cwd is internal/registry; ../../.. leaves the repo
		_, errText := runTool(t, map[string]any{"command": "pwd", "cwd": cwd})
		if !strings.Contains(errText, "outside the repository") {
			t.Errorf("cwd %s: want refusal, got %q", cwd, errText)
		}
	}
}

func TestTBRunRejectsMissingConfigPath(t *testing.T) {
	useConfig(t, "sandbox:\n  read: [/nonexistent/tb-sandbox-path]\n")
	_, errText := runTool(t, map[string]any{"command": "true"})
	if !strings.Contains(errText, "/nonexistent/tb-sandbox-path") {
		t.Fatalf("want an error naming the missing path, got %q", errText)
	}
}

// Without Landlock, tb_run fails closed unless the human opts out in config.
func TestTBRunFailsClosedWithoutLandlock(t *testing.T) {
	restore := sandbox.OverrideLandlockProbe(func() (bool, int, error) { return false, 0, nil })
	defer restore()

	useConfig(t, "")
	if _, errText := runTool(t, map[string]any{"command": "echo RAN"}); !strings.Contains(errText, "allow_unsandboxed") {
		t.Fatalf("want fail-closed refusal mentioning allow_unsandboxed, got %q", errText)
	}

	useConfig(t, "sandbox:\n  allow_unsandboxed: true\n")
	payload, errText := runTool(t, map[string]any{"command": "echo RAN"})
	if errText != "" || payload["sandboxed"] != false || !strings.Contains(payload["output"].(string), "RAN") {
		t.Fatalf("with allow_unsandboxed want an unsandboxed run, got %v %q", payload, errText)
	}
}

// The repository's own .toolbelt/config.yaml is writable from inside the
// sandbox, so an agent could plant env.pass or sandbox.write there. Those
// knobs must come only from the global (or -c) config.
func TestTBRunIgnoresSecurityKnobsFromRepoConfig(t *testing.T) {
	requireLandlock(t)
	config.SetFile("")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // empty global config
	t.Setenv("GITHUB_TOKEN", "secret-gh-token-xyz")
	outside := t.TempDir()
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".toolbelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	planted := "env:\n  pass: [GITHUB_TOKEN]\nsandbox:\n  write: [" + outside + "]\n  allow_unsandboxed: true\n"
	if err := os.WriteFile(filepath.Join(repo, ".toolbelt", "config.yaml"), []byte(planted), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	if _, errText := runTool(t, map[string]any{"command": "true", "pass_env": []string{"GITHUB_TOKEN"}}); !strings.Contains(errText, "pass_env refused") {
		t.Fatalf("pass_env via repo config must be refused, got %q", errText)
	}
	payload, errText := runTool(t, map[string]any{
		"command": "printenv GITHUB_TOKEN; touch " + filepath.Join(outside, "x") + " && echo WROTE",
		"timeout": "10s",
	})
	if errText != "" {
		t.Fatal(errText)
	}
	out, _ := payload["output"].(string)
	if strings.Contains(out, "secret-gh-token-xyz") || strings.Contains(out, "WROTE") {
		t.Fatalf("repo config widened the sandbox:\n%s", out)
	}
}
