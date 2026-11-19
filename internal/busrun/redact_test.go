package busrun

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRedactorGenericPatterns(t *testing.T) {
	r := &redactor{}
	const tok = "a1B2c3D4e5F6g7H8i9J0k1L2m3N4o5P6q7"
	cases := []struct{ in, leak, keep string }{
		{"Authorization: Basic dXNlcjpwYXNz", "dXNlcjpwYXNz", "Authorization: "},
		{"> authorization: Bearer abc123", "abc123", "authorization: "},
		{`curl -H "X-Api-Key: abc12345xyz" https://h.test`, "abc12345xyz", "https://h.test"},
		{"Cookie: a=1; sid=zz9", "sid=zz9", "Cookie: "},
		{"session_token=s3cr3tvalue failed", "s3cr3tvalue", " failed"},
		{"api_secret: s3cr3tvalue ok", "s3cr3tvalue", "api_secret: "},
		{`{"api_key":"abc","x":1}`, `"abc"`, `"x":1`},
		{"access_token: Bearer abc123 end", "abc123", " end"},
		{`password: "hunter22"`, "hunter22", "password: "},
		{"POST https://bob:pa55word@example.test/hook", "pa55word", "example.test/hook"},
		{"GET https://h.test/w?id=1&token=zzz999&x=2", "zzz999", "id=1"},
		{"sh: 5: " + tok + ": not found", tok, "not found"},
		{"key AbCdEfGh12/IjKlMnOp34/QrStUvWx56+YzAbCdEf78=", "QrStUvWx56", "key "},
		{"line1\nerr: x-api-key: shortk1 ok", "shortk1", " ok"},
		{"SECRET_KEY=abc1234 AWS_ACCESS_KEY_ID=AKIAXX12 next", "AKIAXX12", " next"},
		{`password="correct horse" next`, "horse", " next"},
		{"https://u:p@ss@host.test/x", "ss@host", "host.test/x"},
		{"jwt eyJhbGciOi.eyJzdWIiOjEy.sIgNaTuRe9 end", "sIgNaTuRe9", " end"},
	}
	for _, c := range cases {
		got := r.redact(c.in)
		if strings.Contains(got, c.leak) || !strings.Contains(got, c.keep) {
			t.Errorf("redact(%q) = %q", c.in, got)
		}
	}
	for _, keep := range []string{
		`{"success":true,"runUuid":"7ae92da5-af6f-4c87-b2cc-68106022c534"}`,
		"result subtype=success is_error=false turns=6",
		"usage max_tokens=4096 input_tokens: 123",
		"401 unauthorized: authorization failed for request",
		"monkey: banana",
		"/tmp/claude-1000/-home-u--spare-BbeOct/7eb7b369-579a-452e-8aab-917fe3a66224/scratchpad/wake.sh: not found",
	} {
		if got := r.redact(keep); got != keep {
			t.Errorf("redigiu demais: %q => %q", keep, got)
		}
	}
}

func writeSecret(t *testing.T, body string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(f, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestRedactorKnownValues(t *testing.T) {
	f := writeSecret(t, "# comentário\nWAKE_URL=https://hooks.test/r/abc\n"+
		"export WAKE_AUTHORIZATION=Authorization: Bearer\tshrtk9\n"+
		"Q='quoted-value-1'\nC=commented1 # nota\nW=Bearer \"wordq7\"\n")
	t.Setenv("MY_API_KEY", "envsecret42")
	t.Setenv("WAKE_PASS", "abcd")
	r, err := newRedactor(&Config{SecretFiles: []string{f}, PassEnv: []string{" WAKE_PASS"}})
	if err != nil {
		t.Fatal(err)
	}
	in := "e: shrtk9: not found; https://hooks.test/r/abc; quoted-value-1; commented1; wordq7; envsecret42; p abcd"
	got := r.redact(in)
	for _, leak := range []string{"shrtk9", "hooks.test/r/abc", "quoted-value-1", "commented1", "wordq7", "envsecret42", "abcd"} {
		if strings.Contains(got, leak) {
			t.Errorf("vazou %q em %q", leak, got)
		}
	}
	if !strings.Contains(r.redact("Authorization: x"), "Authorization") || !strings.Contains(r.redact("Bearer"), "Bearer") {
		t.Error("palavras de esquema não são segredo")
	}
}

func TestSecretFilesFailClosed(t *testing.T) {
	dir := t.TempDir()
	open := filepath.Join(dir, "aberto")
	if err := os.WriteFile(open, []byte("A=segredo123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(dir, "ausente"), open, fifo, "relativo"} {
		if _, err := newRedactor(&Config{SecretFiles: []string{p}}); err == nil {
			t.Errorf("secret_files %s deveria falhar", p)
		}
	}
}

// Reproduces the incident: wake.sh sources the env unquoted, so sh executes
// the token word and prints it to stderr.
func TestExecStderrRedactedInAudit(t *testing.T) {
	e := newEnv(t)
	sec := writeSecret(t, "WAKE_AUTHORIZATION=Bearer tk7secretw\n")
	script := filepath.Join(filepath.Dir(sec), "wake.sh") // argv[0]'s dir is granted to the child
	if err := os.WriteFile(script, []byte("#!/bin/sh\n. "+sec+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	e.writeConfig(t, "adapter: exec\nexec:\n  argv: ["+script+"]\nsecret_files: ["+sec+"]\ncoalesce:\n  debounce: 10ms\n  max_delay: 1s\n")
	e.send(t, "query", "")
	h, err := Open(&Options{Root: e.root, Channel: "ch", Name: "w", Poll: 10 * time.Millisecond,
		Backoff: time.Millisecond, Attempts: 1, AllowUnsandboxed: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	runFor(t, h, 400*time.Millisecond)
	a := readAudit(t, e)
	if !strings.Contains(a, "wake_attempt_failed") || !strings.Contains(a, "not found") {
		t.Fatalf("audit sem a falha do sh: %s", a)
	}
	if strings.Contains(a, "tk7secretw") {
		t.Fatalf("palavra do segredo vazou no audit: %s", a)
	}
}

func TestResultSummaryRedactsBeforeCut(t *testing.T) {
	r := &redactor{}
	secret := "Authorization: Bearer " + strings.Repeat("x", 10) + "9"
	out := resultSummary(strings.Repeat("ruído ", 200)+secret, 1, r.redact)
	if strings.Contains(out, "xxxxxxxxxx9") {
		t.Fatalf("segredo partido no corte: %s", out)
	}
}

func TestPlainTailRedactsEachLine(t *testing.T) {
	r := &redactor{}
	got := plainTail("first line\nAuthorization: Basic c2hvcnQ\nCookie: sid=zz9\n", 1024, r.redact)
	if strings.Contains(got, "c2hvcnQ") || strings.Contains(got, "zz9") {
		t.Fatalf("linha 2+ não redigida: %s", got)
	}
}

func TestWholeValueAlwaysRedacted(t *testing.T) {
	t.Setenv("WAKE_X", "token")
	r, err := newRedactor(&Config{PassEnv: []string{"WAKE_X"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.redact("valor token aqui"); strings.Contains(got, "token") {
		t.Fatalf("pass_env igual a palavra de esquema não redigido: %s", got)
	}
}
