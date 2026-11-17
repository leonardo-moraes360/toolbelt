package execenv

import (
	"strings"
	"testing"
)

func TestFilterBlocksSecrets(t *testing.T) {
	env := []string{
		"PATH=/usr/bin",
		"HOME=/home/u",
		"LANG=C",
		"GITHUB_TOKEN=secret-gh",
		"AWS_SECRET_ACCESS_KEY=aws",
		"API_KEY=api",
		"MY_PASSWORD=pass",
		"SAFE_CUSTOM=nope",
		"GOPATH=/go",
		"GOCACHE=/tmp/gocache",
		"NODE_ENV=test",
		"npm_config_registry=https://registry.npmjs.org/",
		"npm_config_//registry.npmjs.org/:_authToken=npm-secret",
	}
	got := Filter(env, Options{})
	joined := strings.Join(got, "\n")
	for _, secret := range []string{"secret-gh", "aws", "api", "pass", "npm-secret", "SAFE_CUSTOM"} {
		if strings.Contains(joined, secret) {
			t.Fatalf("secret/unlisted %q leaked in:\n%s", secret, joined)
		}
	}
	for _, want := range []string{"PATH=", "HOME=", "LANG=", "GOPATH=", "GOCACHE=", "NODE_ENV=", "npm_config_registry="} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing allowlisted %q in:\n%s", want, joined)
		}
	}
}

func TestFilterPassEnvOptIn(t *testing.T) {
	env := []string{
		"PATH=/bin",
		"GITHUB_TOKEN=secret-gh",
		"CUSTOM_OK=yes",
	}
	got := Filter(env, Options{
		Allow: []string{"CUSTOM_OK"},
		Pass:  []string{"GITHUB_TOKEN"},
	})
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "GITHUB_TOKEN=secret-gh") {
		t.Fatalf("pass-env should allow GITHUB_TOKEN: %s", joined)
	}
	if !strings.Contains(joined, "CUSTOM_OK=yes") {
		t.Fatalf("--env should allow CUSTOM_OK: %s", joined)
	}
}

func TestFilterAllowDoesNotBypassSecret(t *testing.T) {
	env := []string{"GITHUB_TOKEN=secret-gh", "PATH=/bin"}
	got := Filter(env, Options{Allow: []string{"GITHUB_TOKEN"}})
	for _, e := range got {
		if strings.HasPrefix(e, "GITHUB_TOKEN=") {
			t.Fatalf("--env alone must not pass secrets: %v", got)
		}
	}
}

func TestIsSecret(t *testing.T) {
	cases := map[string]bool{
		"GITHUB_TOKEN":        true,
		"MY_SECRET":           true,
		"API_KEY":             true,
		"DB_PASSWORD":         true,
		"PATH":                false,
		"HOME":                false,
		"NODE_ENV":            false,
		"npm_config_registry": false,
		"AWS_ACCESS_KEY_ID":   true,
		"SSL_CERT_FILE":       false,
	}
	for name, want := range cases {
		if got := IsSecret(name); got != want {
			t.Errorf("IsSecret(%q)=%v want %v", name, got, want)
		}
	}
}
