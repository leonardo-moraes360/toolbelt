package busrun

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
)

// fakeNotifier installs a recording notify-send outside the repository and
// makes the test user count as root for the ownership check.
func fakeNotifier(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "notify-send")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done > \"$PWD/argv\"\nenv > \"$PWD/env\"\n" +
		"if [ -n \"$XDG_RUNTIME_DIR\" ] && (: > \"$XDG_RUNTIME_DIR/probe\") 2>/dev/null; then echo wrote > \"$PWD/xdg\"; fi\nexit 0\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	oldCand, oldUID := notifySendCandidates, rootUID
	notifySendCandidates, rootUID = []string{bin}, uint32(os.Getuid()) //nolint:gosec // test uid
	t.Cleanup(func() { notifySendCandidates, rootUID = oldCand, oldUID })
	return bin
}

func setGOOS(t *testing.T, v string) {
	t.Helper()
	old := goos
	goos = v
	t.Cleanup(func() { goos = old })
}

const notifyCfg = "adapter: notify\nnotify:\n  backend: desktop\ncoalesce:\n  debounce: 80ms\n  max_delay: 2s\n"

// AC1, AC3, AC6: one desktop call with exact argv, bounded text only, and a
// minimal environment.
func TestNotifyDesktopFlow(t *testing.T) {
	setGOOS(t, "linux")
	fakeNotifier(t)
	e := newEnv(t)
	e.writeConfig(t, notifyCfg)
	xdg := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", xdg)
	t.Setenv("OTHER_SECRET_TOKEN", "must-not-pass")
	t.Setenv("DISPLAY", ":0")
	for _, typ := range []bus.Type{bus.TypeTaskAssign, bus.TypeQuery, bus.TypeTaskAssign} {
		if _, err := e.store.Send(e.owner, &bus.SendInput{To: "agent:w", Type: typ, Thread: "click.here",
			Payload: []byte(`{"x":"click http://x"}`)}); err != nil {
			t.Fatal(err)
		}
	}
	h, err := Open(&Options{Root: e.root, Channel: "ch", Name: "w", Poll: 10 * time.Millisecond, AllowUnsandboxed: true})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	runFor(t, h, 800*time.Millisecond)
	argv, err := os.ReadFile(h.base + ".notify/argv")
	if err != nil {
		t.Fatalf("notifier did not run: %v\n%s", err, readAudit(t, e))
	}
	want := "--app-name\ntoolbelt\n--urgency\nnormal\n--\ntoolbelt: w has 3 new messages in ch\n" +
		"2 task.assign, 1 query from \"boss\". Open w and ask it to call tb_bus_recv. Peek from a shell: tb bus recv --channel ch --as w --peek\n"
	if string(argv) != want {
		t.Fatalf("argv:\n%s\nwant:\n%s", argv, want)
	}
	envb, _ := os.ReadFile(h.base + ".notify/env")
	for _, bad := range []string{"OTHER_SECRET_TOKEN", "DISPLAY=", "TB_BUS_CHANNEL"} {
		if strings.Contains(string(envb), bad) {
			t.Fatalf("child env has %s:\n%s", bad, envb)
		}
	}
	audit := readAudit(t, e)
	if strings.Count(audit, `"wake_ok"`) != 1 || !strings.Contains(audit, `"backend":"desktop"`) || strings.Contains(audit, "click") {
		t.Fatalf("audit:\n%s", audit)
	}
	// AC11 (file side): the sandbox grants nothing in XDG_RUNTIME_DIR.
	if landlockOn() {
		if _, err := os.Stat(h.base + ".notify/xdg"); err == nil {
			t.Fatal("the notifier could write in XDG_RUNTIME_DIR")
		}
	}
}

func packetOf(reason string, msgs ...bus.Message) *Packet {
	return newPacket(bus.Member{Channel: "ch", Name: "w"}, reason, msgs)
}

// AC3, AC4, AC5, AC9: text from counts and quoted, cut sender names.
func TestNotifyText(t *testing.T) {
	cfg := &NotifyConfig{Urgency: "normal", UrgentUrgency: "critical"}
	var msgs []bus.Message
	for i := range 25 {
		msgs = append(msgs, bus.Message{Seq: int64(i + 1), From: "agent:s" + strconv.Itoa(i%5), Type: bus.TypeQuery})
	}
	sum, body, urg := notifyText(packetOf("messages", msgs...), cfg)
	if sum != "toolbelt: w has 25 new messages in ch" || urg != "normal" ||
		!strings.HasPrefix(body, `25 query from "s0", "s1", "s2" and 2 others.`) {
		t.Fatalf("%q / %q / %s", sum, body, urg)
	}
	long := "agent:" + strings.Repeat("run-this-now", 5)
	p := packetOf("rewake",
		bus.Message{Seq: 1, From: bus.WatchdogSender, Type: bus.TypeTaskStatus},
		bus.Message{Seq: 2, From: long, Type: bus.TypeUrgent})
	sum, body, urg = notifyText(p, cfg)
	if sum != "urgent: toolbelt: w has 2 still unread messages in ch" || urg != "critical" {
		t.Fatalf("%q %s", sum, urg)
	}
	if !strings.Contains(body, `"`+strings.Repeat("run-this-now", 5)[:31]+`…"`) || !strings.Contains(body, "the broker") {
		t.Fatalf("senders: %q", body)
	}
	cfg.UrgentUrgency = "normal"
	if _, _, urg = notifyText(p, cfg); urg != "normal" {
		t.Fatalf("urgent_urgency ignored: %s", urg)
	}
	if sum, _, _ = notifyText(packetOf("messages", msgs[0]), cfg); sum != "toolbelt: w has 1 new message in ch" {
		t.Fatalf("singular: %q", sum)
	}
}

// AC2: the macOS argv keeps the text out of the AppleScript source.
func TestNotifyMacArgv(t *testing.T) {
	setGOOS(t, "darwin")
	got := strings.Join(desktopArgv("/usr/bin/osascript", "S", "B", "normal"), "|")
	want := "/usr/bin/osascript|-e|on run argv|-e|display notification (item 1 of argv) with title (item 2 of argv)|-e|end run|B|S"
	if got != want {
		t.Fatalf("%s", got)
	}
}

// AC7, AC9: Validate refusals and notify defaults.
func TestNotifyValidate(t *testing.T) {
	setGOOS(t, "linux")
	e := newEnv(t)
	bad := map[string]string{
		notifyCfg + "pass_env: [X]\n":                                               "not allowed",
		notifyCfg + "secret_files: [/x]\n":                                          "not allowed",
		notifyCfg + "sandbox:\n  read: [/x]\n":                                      "not allowed",
		notifyCfg + "exec:\n  argv: [/bin/true]\n":                                  "block not allowed",
		"adapter: exec\nexec:\n  argv: [/bin/true]\nnotify:\n  backend: terminal\n": "block not allowed",
		"adapter: notify\nnotify:\n  backend: desktop\n  command: rel/notify\n":     "absolute",
		"adapter: notify\nnotify:\n  backend: pager\n":                              "invalid",
		"adapter: notify\nnotify:\n  backend: terminal\n  urgency: loud\n":          "urgency",
	}
	for body, sub := range bad {
		if _, err := LoadConfig(e.writeConfig(t, body)); err == nil || !strings.Contains(err.Error(), sub) {
			t.Errorf("%q: %v (want %q)", body, err, sub)
		}
	}
	c, err := LoadConfig(e.writeConfig(t, "adapter: notify\nnotify:\n  backend: terminal\nsandbox: {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.RewakeAfter != time.Hour || c.Timeout != 10*time.Second || c.Notify.UrgentUrgency != "critical" {
		t.Fatalf("defaults: %+v", c)
	}
	setGOOS(t, "windows")
	if _, err := LoadConfig(e.writeConfig(t, notifyCfg)); err == nil || !strings.Contains(err.Error(), "notify_unsupported") {
		t.Fatalf("windows desktop: %v", err)
	}
}

// AC7: a notifier an agent could have replaced is refused at open.
func TestNotifyUntrustedBinary(t *testing.T) {
	setGOOS(t, "linux")
	e := newEnv(t)
	e.writeConfig(t, notifyCfg)
	open := func() error {
		h, err := Open(&Options{Root: e.root, Channel: "ch", Name: "w", AllowUnsandboxed: true})
		if err == nil {
			h.Close()
		}
		return err
	}
	bin := fakeNotifier(t)
	if err := open(); err != nil {
		t.Fatalf("trusted notifier refused: %v", err)
	}
	if os.Getuid() != 0 {
		rootUID = 0
		if err := open(); err == nil || !strings.Contains(err.Error(), "not owned by root") {
			t.Fatalf("user-owned notifier: %v", err)
		}
		rootUID = uint32(os.Getuid()) //nolint:gosec // test uid
	}
	if err := os.Chmod(filepath.Dir(bin), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := open(); err == nil || !strings.Contains(err.Error(), "writable") {
		t.Fatalf("writable dir: %v", err)
	}
	inRepo := filepath.Join(e.root, "notify-send")
	if err := os.WriteFile(inRepo, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(e.root, 0o755); err != nil {
		t.Fatal(err)
	}
	notifySendCandidates = []string{inRepo}
	if err := open(); err == nil || !strings.Contains(err.Error(), "under") {
		t.Fatalf("notifier in the repository: %v", err)
	}
	notifySendCandidates = []string{"/nonexistent/notify-send"}
	if err := open(); err == nil || !strings.Contains(err.Error(), "no desktop notifier") {
		t.Fatalf("missing notifier: %v", err)
	}
}

// AC10: terminal writes one line with the bell, without a sandbox.
func TestNotifyTerminal(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, "adapter: notify\nnotify:\n  backend: terminal\ncoalesce:\n  debounce: 50ms\n  max_delay: 1s\n")
	e.send(t, bus.TypeQuery, `{}`)
	h, err := Open(&Options{Root: e.root, Channel: "ch", Name: "w", Poll: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.Check(); err != nil {
		t.Fatalf("terminal needs no sandbox: %v", err)
	}
	var buf bytes.Buffer
	h.adapter.(*notifyAdapter).out = &buf
	runFor(t, h, 500*time.Millisecond)
	got := buf.String()
	if strings.Count(got, "\n") != 1 || !strings.HasSuffix(got, "\a\n") || !strings.HasPrefix(got, "toolbelt: w has 1 new message in ch: 1 query") {
		t.Fatalf("terminal line: %q", got)
	}
}

// AC8: the card's wake disagreeing with notify is a warning, not an error.
func TestNotifyWakeMismatch(t *testing.T) {
	e := newEnv(t)
	e.writeConfig(t, "adapter: notify\nnotify:\n  backend: terminal\n")
	h := e.open(t, nil)
	w, err := h.Warnings()
	if err != nil || len(w) != 1 || !strings.Contains(w[0], "card says manual") {
		t.Fatalf("warnings: %v %v", w, err)
	}
}
