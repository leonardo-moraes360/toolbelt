//go:build linux

package egm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAgent is a custom-vendor agent driven by the first stdin line, so one
// script covers every scenario without the real claude binary.
const fakeAgent = `#!/bin/sh
read -r mode
case "$mode" in
echo) cat ;;
env) env ;;
probe)
	read -r secret
	read -r repo
	cat prompt.txt >/dev/null 2>&1 && echo packet-readable
	(: > "$PWD/w") 2>/dev/null && echo packet-writable
	cat "$secret" >/dev/null 2>&1 && echo home-readable
	ls "$repo" >/dev/null 2>&1 && echo repo-readable
	(: > "$HOME/w") 2>/dev/null && echo home-writable
	(: > "$TMPDIR/w") 2>/dev/null && echo tmp-writable
	;;
flood) head -c 5000 /dev/zero | tr '\0' a ;;
sleep) sleep 30 ;;
fail) echo boom >&2; exit 3 ;;
escape) setsid sh -c 'sleep 30' </dev/null >/dev/null 2>&1 & sleep 0.3; echo payload ;;
anon) env -u TB_EGM_OWNER setsid sh -c 'sleep 30' </dev/null >/dev/null 2>&1 & sleep 0.3; echo payload ;;
esac
`

// fakeClaude checks the profile flags and the copied credential, then
// prints the envelope stored next to it.
const fakeClaude = `#!/bin/sh
[ "$1" = "-p" ] || exit 7
cred="$CLAUDE_CONFIG_DIR/.credentials.json"
grep -q claudeAiOauth "$cred" || exit 8
grep -q otherAccount "$cred" && exit 9
cat >/dev/null
cat "$(dirname "$0")/envelope.json"
`

type harness struct {
	t      *testing.T
	tmp    string // TMPDIR of the test: workspaces land here
	home   string // fake real HOME
	binDir string
	packet string
	set    *Settings
}

func newHarness(t *testing.T, script string) *harness {
	t.Helper()
	tmp := t.TempDir()
	h := &harness{
		t: t, tmp: tmp, home: filepath.Join(tmp, "realhome"),
		binDir: filepath.Join(tmp, "bin"), packet: filepath.Join(tmp, "packet"),
	}
	t.Setenv("TMPDIR", tmp)
	t.Setenv("HOME", h.home)
	writeFile(t, filepath.Join(h.home, "secret"), "real home secret\n")
	writeFile(t, filepath.Join(h.packet, "prompt.txt"), "prompt\n")
	agent := filepath.Join(h.binDir, "agent")
	writeFile(t, agent, script)
	if err := os.Chmod(agent, 0o700); err != nil {
		t.Fatal(err)
	}
	h.set = testSettings(agent)
	h.set.Vendor, h.set.ToolsDisabled, h.set.Output = VendorCustom, true, OutputRaw
	h.set.TimeoutPerReviewer = "20s"
	return h
}

// runner builds the Runner, skipping where the kernel lacks Landlock or the
// subreaper, and checks at cleanup that no workspace survived.
func (h *harness) runner() *Runner {
	h.t.Helper()
	p, err := ResolveProfile(h.set)
	if err != nil {
		h.t.Fatal(err)
	}
	r, err := NewRunner(h.set, p)
	var e *Error
	if errors.As(err, &e) && (e.Reason == "no_landlock" || e.Reason == "no_subreaper") {
		h.t.Skipf("goldfish sandbox unavailable: %v", err)
	}
	if err != nil {
		h.t.Fatal(err)
	}
	h.t.Cleanup(r.Close)
	return r
}

func (h *harness) req(owner, prompt string) *GoldfishRequest {
	return &GoldfishRequest{Owner: owner, Attempt: 1, Packet: h.packet, Prompt: prompt}
}

func (h *harness) assertNoWorkspace() {
	h.t.Helper()
	left, _ := filepath.Glob(filepath.Join(h.tmp, workspacePrefix+"*"))
	if len(left) > 0 {
		h.t.Fatalf("workspaces left behind: %v", left)
	}
}

func wantResult(t *testing.T, res *GoldfishResult, reason string) {
	t.Helper()
	if res.Reason != reason || res.Completed != (reason == "") {
		t.Fatalf("want reason %q, got completed=%v reason=%q stderr=%q", reason, res.Completed, res.Reason, res.Stderr)
	}
}

func TestRunnerRawPayload(t *testing.T) {
	h := newHarness(t, fakeAgent)
	r := h.runner()
	res := r.Run(context.Background(), h.req("egm:s:r1:design", "echo\nhello reviewer\n"))
	wantResult(t, res, "")
	if string(res.Payload) != "hello reviewer\n" {
		t.Fatalf("payload = %q", res.Payload)
	}
	h.assertNoWorkspace()
}

func TestRunnerMinimalEnv(t *testing.T) {
	h := newHarness(t, fakeAgent)
	t.Setenv("NODE_OPTIONS", "--require=/tmp/evil.js")
	t.Setenv("PYTHONSTARTUP", "/tmp/evil.py")
	t.Setenv("EGM_TEST_PASS", "forwarded")
	h.set.Env.Pass = []string{"EGM_TEST_PASS"}
	r := h.runner()
	req := h.req("egm:s:r1:style", "env\n")
	req.Attempt = 2
	res := r.Run(context.Background(), req)
	wantResult(t, res, "")
	env := "\n" + string(res.Payload)
	for _, want := range []string{"\nEGM_TEST_PASS=forwarded\n", "\nTB_EGM_OWNER=egm:s:r1:style\n", "\nTB_EGM_ATTEMPT=2\n", "\nLANG=C.UTF-8\n", "\nPATH=/usr/bin:/bin\n"} {
		if !strings.Contains(env, want) {
			t.Errorf("env lacks %q:\n%s", strings.TrimSpace(want), env)
		}
	}
	for _, bad := range []string{"NODE_OPTIONS", "PYTHONSTARTUP", "TB_SANDBOX_", "\nHOME=" + h.home + "\n"} {
		if strings.Contains(env, bad) {
			t.Errorf("env leaks %q", bad)
		}
	}
}

func TestRunnerIsolation(t *testing.T) {
	h := newHarness(t, fakeAgent)
	r := h.runner()
	repo, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	prompt := fmt.Sprintf("probe\n%s\n%s\n", filepath.Join(h.home, "secret"), repo)
	res := r.Run(context.Background(), h.req("egm:s:r1:design", prompt))
	wantResult(t, res, "")
	got := strings.Fields(string(res.Payload))
	want := []string{"packet-readable", "home-writable", "tmp-writable"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("probe = %v, want %v", got, want)
	}
	if _, err := os.Stat(filepath.Join(h.packet, "w")); err == nil {
		t.Fatal("the packet was written")
	}
	h.assertNoWorkspace()
}

func TestRunnerClaudeEnvelope(t *testing.T) {
	h := newHarness(t, fakeClaude)
	h.set.Vendor, h.set.Output, h.set.ToolsDisabled = VendorClaude, "", false
	writeCredential(t, h.home, time.Now().Add(time.Hour))
	r := h.runner()
	cases := []struct {
		name, envelope, reason, payload string
		toolsUsed                       bool
	}{
		{"result", `{"type":"result","result":"{\"nonce\":\"n1\"}","num_turns":1,"permission_denials":[]}`, "", `{"nonce":"n1"}`, false},
		{"two turns", `{"type":"result","result":"x","num_turns":2,"permission_denials":[]}`, ReasonToolsUsed, "", true},
		{"denials", `{"type":"result","result":"x","num_turns":1,"permission_denials":[{"tool_name":"Bash"}]}`, ReasonToolsUsed, "", true},
		{"garbage", `not json`, ReasonMissingOutput, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeFile(t, filepath.Join(h.binDir, "envelope.json"), tc.envelope)
			res := r.Run(context.Background(), h.req("egm:s:r1:design", "review this"))
			wantResult(t, res, tc.reason)
			if string(res.Payload) != tc.payload || res.ToolsUsed != tc.toolsUsed {
				t.Fatalf("payload=%q toolsUsed=%v", res.Payload, res.ToolsUsed)
			}
		})
	}
	h.assertNoWorkspace()
}

func TestRunnerClaudeAuthExpired(t *testing.T) {
	h := newHarness(t, fakeClaude)
	h.set.Vendor, h.set.Output, h.set.ToolsDisabled = VendorClaude, "", false
	writeCredential(t, h.home, time.Now().Add(2*time.Minute))
	p, err := ResolveProfile(h.set)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewRunner(h.set, p)
	var e *Error
	if errors.As(err, &e) && (e.Reason == "no_landlock" || e.Reason == "no_subreaper") {
		t.Skip(err)
	}
	wantReason(t, err, "auth_expired")
}

func TestRunnerFailures(t *testing.T) {
	cases := []struct {
		name, mode, reason string
		edit               func(*Settings)
	}{
		{"overflow", "flood", ReasonStdoutOverflow, func(s *Settings) { s.MaxStdoutBytes = 100 }},
		{"timeout", "sleep", ReasonTimeout, func(s *Settings) { s.TimeoutPerReviewer = "300ms" }},
		{"exit code", "fail", "exit_3", func(*Settings) {}},
		{"empty output", "unknown-mode", ReasonMissingOutput, func(*Settings) {}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, fakeAgent)
			tc.edit(h.set)
			r := h.runner()
			start := time.Now()
			res := r.Run(context.Background(), h.req("egm:s:r1:design", tc.mode+"\n"))
			wantResult(t, res, tc.reason)
			if time.Since(start) > 10*time.Second {
				t.Fatalf("took %s", time.Since(start))
			}
			h.assertNoWorkspace()
		})
	}
}

func TestRunnerEscapeAttributed(t *testing.T) {
	h := newHarness(t, fakeAgent)
	r := h.runner()
	owner := "egm:s:r1:design"
	res := r.Run(context.Background(), h.req(owner, "escape\n"))
	wantResult(t, res, ReasonEscaped)
	if !r.Escaped(owner) || r.Escaped("egm:s:r1:style") || r.Unattributed() {
		t.Fatalf("escaped=%v unattributed=%v", r.Escaped(owner), r.Unattributed())
	}
	h.assertNoWorkspace()
}

func TestRunnerUnattributedKillsRunning(t *testing.T) {
	h := newHarness(t, fakeAgent)
	r := h.runner()
	long := make(chan *GoldfishResult, 1)
	go func() { long <- r.Run(context.Background(), h.req("egm:s:r1:long", "sleep\n")) }()
	waitRunning(t, r, 1)
	res := r.Run(context.Background(), h.req("egm:s:r1:anon", "anon\n"))
	if !r.Unattributed() || r.Escaped("egm:s:r1:anon") {
		t.Fatalf("unattributed=%v escaped=%v (result %+v)", r.Unattributed(), r.Escaped("egm:s:r1:anon"), res)
	}
	select {
	case lr := <-long:
		wantResult(t, lr, ReasonUnattributed)
	case <-time.After(10 * time.Second):
		t.Fatal("the running goldfish was not killed")
	}
	h.assertNoWorkspace()
}

func waitRunning(t *testing.T, r *Runner, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.sweepMu.Lock()
		got := len(r.running)
		r.sweepMu.Unlock()
		if got >= n {
			time.Sleep(100 * time.Millisecond) // let the agent reach its sleep
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fewer than %d goldfish running", n)
}

func TestRunnerConcurrent(t *testing.T) {
	h := newHarness(t, fakeAgent)
	r := h.runner()
	const n = 6
	var wg sync.WaitGroup
	results := make([]*GoldfishResult, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = r.Run(context.Background(), h.req("egm:s:r1:g"+strconv.Itoa(i), "echo\npayload "+strconv.Itoa(i)))
		}()
	}
	wg.Wait()
	for i, res := range results {
		wantResult(t, res, "")
		if string(res.Payload) != "payload "+strconv.Itoa(i) {
			t.Fatalf("goldfish %d payload = %q", i, res.Payload)
		}
	}
	if r.Unattributed() {
		t.Fatal("clean goldfish reported an unattributed orphan")
	}
	h.assertNoWorkspace()
}

func TestRunnerClosed(t *testing.T) {
	h := newHarness(t, fakeAgent)
	r := h.runner()
	r.Close()
	wantResult(t, r.Run(context.Background(), h.req("egm:s:r1:design", "echo\n")), ReasonSandboxError)
	h.assertNoWorkspace()
}

func TestRunnerSweepsStaleWorkspaces(t *testing.T) {
	base := t.TempDir()
	old := time.Now().Add(-2 * time.Hour)
	mk := func(name, pid string, mtime time.Time) string {
		dir := filepath.Join(base, name)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if pid != "" {
			writeFile(t, filepath.Join(dir, ownerPIDFile), pid)
		}
		if err := os.Chtimes(dir, mtime, mtime); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	// 99999999 is above any pid_max, so it is never alive.
	dead := mk("tb-egm-dead", "99999999", old)
	alive := mk("tb-egm-alive", strconv.Itoa(os.Getpid()), old)
	fresh := mk("tb-egm-fresh", "99999999", time.Now())
	foreign := mk("tb-egm-nopid", "", old)
	other := mk("other-dead", "99999999", old)
	sweepStaleWorkspaces(base, time.Now())
	for dir, want := range map[string]bool{dead: false, alive: true, fresh: true, foreign: true, other: true} {
		if _, err := os.Stat(dir); (err == nil) != want {
			t.Errorf("%s exists=%v, want %v", filepath.Base(dir), err == nil, want)
		}
	}
}
