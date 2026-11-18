package registry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/stack"
)

func TestTBRunFiltersSecrets(t *testing.T) {
	useConfig(t, runConfigYAML(""))
	t.Setenv("GITHUB_TOKEN", "secret-gh-token-xyz")
	t.Setenv("MY_PASSWORD", "pass123")
	t.Setenv("API_KEY", "api-key-leak")
	t.Setenv("SAFE_CUSTOM", "should-not-pass")

	res, err := Default().Call(context.Background(), "tb_run", map[string]any{
		"command": `printenv | grep -E '^(GITHUB_TOKEN|MY_PASSWORD|API_KEY|SAFE_CUSTOM|PATH)=' | sort`,
		"timeout": "5s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("isError: %s", res.Content[0].Text)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	text, _ := payload["output"].(string)
	for _, secret := range []string{"secret-gh-token-xyz", "pass123", "api-key-leak", "should-not-pass"} {
		if strings.Contains(text, secret) {
			t.Fatalf("tb_run leaked %q:\n%s", secret, text)
		}
	}
	if !strings.Contains(text, "PATH=") {
		t.Fatalf("PATH should remain: %s", text)
	}
}

// useConfig points config.Load at a temp file for the test.
func useConfig(t *testing.T, yaml string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	config.SetFile(path)
	t.Cleanup(func() { config.SetFile("") })
}

func TestTBRunPassEnvOptIn(t *testing.T) {
	useConfig(t, runConfigYAML("env:\n  pass: [GITHUB_TOKEN]\n"))
	t.Setenv("GITHUB_TOKEN", "secret-gh-token-xyz")
	t.Setenv("CUSTOM_OK", "yes-custom")

	res, err := Default().Call(context.Background(), "tb_run", map[string]any{
		"command":  `printenv | grep -E '^(GITHUB_TOKEN|CUSTOM_OK)=' | sort`,
		"timeout":  "5s",
		"env":      []string{"CUSTOM_OK"},
		"pass_env": []string{"GITHUB_TOKEN"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("isError: %s", res.Content[0].Text)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	text, _ := payload["output"].(string)
	if !strings.Contains(text, "GITHUB_TOKEN=secret-gh-token-xyz") {
		t.Fatalf("pass_env should allow token: %s", text)
	}
	if !strings.Contains(text, "CUSTOM_OK=yes-custom") {
		t.Fatalf("env should allow CUSTOM_OK: %s", text)
	}
}

func TestStackVerifyFiltersSecrets(t *testing.T) {
	dir := gitRepo(t)
	outFile := filepath.Join(dir, "child-env.txt")
	t.Setenv("GITHUB_TOKEN", "secret-gh-token-xyz")
	t.Setenv("MY_PASSWORD", "pass123")

	opts := &stack.Options{
		RepoDir: dir,
		Base:    "HEAD",
		Head:    "HEAD",
		Ops:     []stack.Op{stack.OpVerify},
		TestCmd: "printenv > " + outFile,
		MaxIter: 1,
		Timeout: 30 * time.Second,
	}
	res, err := stack.Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("stack not ok: %#v", res)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, secret := range []string{"secret-gh-token-xyz", "pass123"} {
		if strings.Contains(text, secret) {
			t.Fatalf("stack verify leaked %q:\n%s", secret, text)
		}
	}
}

// Over MCP, pass_env comes from the model: a key the human did not opt in
// through config must be refused, never passed (M8ven review, finding 2).
func TestToolPassEnvRefusedWithoutConfigOptIn(t *testing.T) {
	useConfig(t, "env:\n  pass: [OTHER_TOKEN]\n")
	t.Setenv("GITHUB_TOKEN", "secret-gh-token-xyz")

	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"tb_run", map[string]any{"command": "printenv GITHUB_TOKEN", "timeout": "5s"}},
		{"tb_verify_run", map[string]any{"cwd": gitRepo(t), "test_cmd": "printenv GITHUB_TOKEN", "max_iter": 1}},
		{"tb_stack", map[string]any{"cwd": gitRepo(t), "ops": []string{"verify"}, "test_cmd": "printenv GITHUB_TOKEN", "max_iter": 1}},
	} {
		tc.args["pass_env"] = []string{"GITHUB_TOKEN"}
		res, err := Default().Call(context.Background(), tc.tool, tc.args)
		if err != nil {
			t.Fatalf("%s: %v", tc.tool, err)
		}
		text := res.Content[0].Text
		if !res.IsError || !strings.Contains(text, "pass_env refused for GITHUB_TOKEN") {
			t.Errorf("%s: want pass_env refusal, got isError=%v: %s", tc.tool, res.IsError, text)
		}
		if strings.Contains(text, "secret-gh-token-xyz") {
			t.Errorf("%s leaked the secret: %s", tc.tool, text)
		}
	}
}
