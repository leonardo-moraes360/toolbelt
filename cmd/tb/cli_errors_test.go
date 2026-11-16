package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestRunEErrorOmitsUsage(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)

	stderr, code := runTBCapture(t, tbBin, "", "graph", "callers")
	if code == 0 {
		t.Fatalf("erro esperado deveria sair ≠ 0\n%s", stderr)
	}
	if !strings.Contains(stderr, "--function") {
		t.Fatalf("stderr deveria mencionar o erro: %s", stderr)
	}
	if strings.Contains(stderr, "Usage:") {
		t.Fatalf("erro de RunE não deve dumpar Usage:\n%s", stderr)
	}
}

func TestFlagParseErrorKeepsUsage(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)

	stderr, code := runTBCapture(t, tbBin, "", "drift", "--not-a-real-flag")
	if code == 0 {
		t.Fatalf("flag desconhecida deveria sair ≠ 0\n%s", stderr)
	}
	if !strings.Contains(stderr, "unknown flag") {
		t.Fatalf("stderr deveria mencionar unknown flag: %s", stderr)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Fatalf("erro de parse de flag deve manter Usage:\n%s", stderr)
	}
}

func TestArgParseErrorKeepsUsage(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)

	stderr, code := runTBCapture(t, tbBin, "", "workspace", "remove")
	if code == 0 {
		t.Fatalf("args inválidos deveriam sair ≠ 0\n%s", stderr)
	}
	if !strings.Contains(stderr, "accepts") {
		t.Fatalf("stderr deveria mencionar erro de args: %s", stderr)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Fatalf("erro de parse de args deve manter Usage:\n%s", stderr)
	}
}

func TestUnknownSubcommandExitsNonZero(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	tbBin := buildTB(t)

	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "root", args: []string{"nonexistent"}, want: `unknown command "nonexistent"`},
		{name: "mem", args: []string{"mem", "show"}, want: `unknown command "show"`},
		{name: "bench", args: []string{"bench", "nope"}, want: `unknown command "nope"`},
		{name: "graph", args: []string{"graph", "query"}, want: `unknown command "query"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stderr, code := runTBCapture(t, tbBin, "", tc.args...)
			if code == 0 {
				t.Fatalf("subcommand desconhecido deveria sair ≠ 0\n%s", stderr)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("stderr deveria conter %q:\n%s", tc.want, stderr)
			}
		})
	}
}

func runTBCapture(t *testing.T, tbBin, dir string, args ...string) (stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(tbBin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var errBuf bytes.Buffer
	cmd.Stdout = &errBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err == nil {
		return errBuf.String(), 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("erro inesperado: %v\n%s", err, errBuf.String())
	}
	return errBuf.String(), exitErr.ExitCode()
}
