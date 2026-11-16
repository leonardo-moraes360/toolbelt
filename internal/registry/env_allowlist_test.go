package registry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/stack"
)

func TestTBRunFiltersSecrets(t *testing.T) {
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

func TestTBRunPassEnvOptIn(t *testing.T) {
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
