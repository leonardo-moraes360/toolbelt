package verify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunConfiguredCmdFiltersSecrets(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	outFile := filepath.Join(dir, "child-env.txt")
	t.Setenv("GITHUB_TOKEN", "secret-gh-token-xyz")
	t.Setenv("MY_PASSWORD", "pass123")
	t.Setenv("API_KEY", "api-key-leak")
	t.Setenv("SAFE_CUSTOM", "should-not-pass")
	t.Setenv("PATH", os.Getenv("PATH"))

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.VisibleTest = "printenv > " + outFile

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s", res.Verdict)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, secret := range []string{"secret-gh-token-xyz", "pass123", "api-key-leak", "should-not-pass"} {
		if strings.Contains(text, secret) {
			t.Fatalf("child env leaked %q:\n%s", secret, text)
		}
	}
	if !strings.Contains(text, "PATH=") {
		t.Fatalf("PATH should remain allowlisted:\n%s", text)
	}
}

func TestRunConfiguredCmdPassEnvOptIn(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	outFile := filepath.Join(dir, "child-env.txt")
	t.Setenv("GITHUB_TOKEN", "secret-gh-token-xyz")
	t.Setenv("CUSTOM_OK", "yes-custom")

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.VisibleTest = "printenv > " + outFile
	cfg.EnvAllow = []string{"CUSTOM_OK"}
	cfg.EnvPass = []string{"GITHUB_TOKEN"}

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != VerdictPass {
		t.Fatalf("veredicto %s", res.Verdict)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "GITHUB_TOKEN=secret-gh-token-xyz") {
		t.Fatalf("pass-env should allow GITHUB_TOKEN:\n%s", text)
	}
	if !strings.Contains(text, "CUSTOM_OK=yes-custom") {
		t.Fatalf("--env should allow CUSTOM_OK:\n%s", text)
	}
}
