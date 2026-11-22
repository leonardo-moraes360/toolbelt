package busrun

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
)

// The notify adapter pings the human, never the agent, with counts, types
// and sender names only (docs/bus-notify-discover.md).

// Desktop binaries; no PATH search, so a notify-send an agent drops in
// ~/.local/bin is never run (tests override these).
var (
	notifySendCandidates = []string{"/usr/bin/notify-send", "/usr/local/bin/notify-send"}
	osascriptPath        = "/usr/bin/osascript"
)

const (
	maxSenderNames = 3
	maxSenderLen   = 32
)

type notifyAdapter struct {
	h   *Harness
	bin string    // desktop: resolved once at open
	out io.Writer // terminal
}

func newNotifyAdapter(h *Harness) (*notifyAdapter, error) {
	a := &notifyAdapter{h: h, out: os.Stderr}
	if h.cfg.Notify.Backend != BackendDesktop {
		return a, nil
	}
	bin, err := resolveNotifyBinary(h.cfg.Notify.Command)
	if err != nil {
		return nil, err
	}
	if err := trustedBinary(bin, h.root); err != nil {
		return nil, err
	}
	a.bin = bin
	return a, nil
}

func resolveNotifyBinary(command string) (string, error) {
	candidates := notifySendCandidates
	switch {
	case goos == "darwin":
		candidates = []string{osascriptPath}
	case command != "":
		candidates = []string{command}
	}
	for _, c := range candidates {
		resolved, err := filepath.EvalSymlinks(c)
		if err != nil {
			continue
		}
		if st, err := os.Stat(resolved); err == nil && st.Mode().IsRegular() {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("notify: no desktop notifier found (tried %s)", strings.Join(candidates, ", "))
}

// trustedBinary refuses a notifier an agent could have replaced: it and its
// directory must be root-owned and not group/world-writable, and it must not
// live under the repository or the bus private directory.
func trustedBinary(bin, root string) error {
	// Every ancestor too: a user-writable one could swap the directory
	// between the check at open and the exec at wake.
	for p := bin; ; p = filepath.Dir(p) {
		if err := rootOwned(p); err != nil {
			return fmt.Errorf("notify: %s: %w", p, err)
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	priv, err := bus.PrivateDir(root)
	if err != nil {
		return err
	}
	for _, dir := range []string{root, priv} {
		if within(bin, dir) {
			return fmt.Errorf("notify: %s is under %s", bin, dir)
		}
	}
	return nil
}

func within(path, dir string) bool {
	if d, err := filepath.EvalSymlinks(dir); err == nil {
		dir = d
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (a *notifyAdapter) Wake(ctx context.Context, p *Packet, _ *State) (Outcome, error) {
	summary, body, urgency := notifyText(p, &a.h.cfg.Notify)
	if a.h.cfg.Notify.Backend == BackendTerminal {
		if _, err := fmt.Fprintf(a.out, "%s: %s\a\n", summary, body); err != nil {
			return Outcome{}, err
		}
		return Outcome{}, nil
	}
	dir := a.h.base + ".notify" // empty private workdir: nothing else is writable
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Outcome{}, err
	}
	out, err := runChild(ctx, a.h, &childSpec{argv: desktopArgv(a.bin, summary, body, urgency), env: notifyEnv(), workdir: dir})
	out.Output = a.h.redactor.redact(out.Output)
	return out, err
}

// desktopArgv puts the text in argv items, never in a shell or the
// AppleScript source. The body starts with a count, so it is never an option.
func desktopArgv(bin, summary, body, urgency string) []string {
	if goos == "darwin" {
		return []string{bin, "-e", "on run argv", "-e", "display notification (item 1 of argv) with title (item 2 of argv)",
			"-e", "end run", body, summary}
	}
	return []string{bin, "--app-name", "toolbelt", "--urgency", urgency, "--", summary, body}
}

// notifyEnv: what a desktop notifier needs to reach the session bus, and
// nothing else (no DISPLAY, no secrets, no TB_BUS_*).
func notifyEnv() []string {
	env := []string{} // never nil: a nil Env makes the sandbox inherit everything
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch {
		case k == "PATH", k == "HOME", k == "LANG", k == "DBUS_SESSION_BUS_ADDRESS", k == "XDG_RUNTIME_DIR",
			strings.HasPrefix(k, "LC_"):
			env = append(env, kv)
		}
	}
	return env
}

// notifyText builds fixed English text from bounded values only: validated
// channel/agent names, the closed Type enum, counts and quoted sender names.
func notifyText(p *Packet, cfg *NotifyConfig) (summary, body, urgency string) {
	agent := strings.TrimPrefix(p.Agent, "agent:")
	state := "new"
	if p.Reason == "rewake" {
		state = "still unread"
	}
	noun := "messages"
	if p.Count == 1 {
		noun = "message"
	}
	summary = fmt.Sprintf("toolbelt: %s has %d %s %s in %s", agent, p.Count, state, noun, p.Channel)
	urgency = cfg.Urgency
	if p.Types[bus.TypeUrgent] > 0 {
		summary = "urgent: " + summary
		urgency = cfg.UrgentUrgency
	}
	body = fmt.Sprintf("%s from %s. Open %s and ask it to call tb_bus_recv. Peek from a shell: tb bus recv --channel %s --as %s --peek",
		typeCounts(p.Types), senderList(p), agent, p.Channel, agent)
	return summary, body, urgency
}

func typeCounts(types map[bus.Type]int) string {
	keys := make([]bus.Type, 0, len(types))
	for t := range types {
		keys = append(keys, t)
	}
	sort.Slice(keys, func(i, j int) bool {
		if types[keys[i]] != types[keys[j]] {
			return types[keys[i]] > types[keys[j]]
		}
		return keys[i] < keys[j]
	})
	parts := make([]string, len(keys))
	for i, t := range keys {
		parts[i] = fmt.Sprintf("%d %s", types[t], t)
	}
	return strings.Join(parts, ", ")
}

// senderList quotes agent-chosen names, cuts them, and shows system:*
// senders as the broker; senders arrive sorted by count, then name.
func senderList(p *Packet) string {
	var names []string
	seen := map[string]bool{}
	for _, s := range p.Senders {
		label := senderLabel(s.From)
		if seen[label] {
			continue // all system:* senders are one "the broker"
		}
		seen[label] = true
		names = append(names, label)
	}
	shown := names
	if len(shown) > maxSenderNames {
		shown = shown[:maxSenderNames]
	}
	out := strings.Join(shown, ", ")
	switch others := len(names) + p.SendersMore - len(shown); {
	case others == 1:
		out += " and 1 other"
	case others > 1:
		out += fmt.Sprintf(" and %d others", others)
	}
	return out
}

func senderLabel(from string) string {
	if strings.HasPrefix(from, "system:") {
		return "the broker"
	}
	name := strings.TrimPrefix(from, "agent:")
	if r := []rune(name); len(r) > maxSenderLen {
		name = string(r[:maxSenderLen-1]) + "…"
	}
	return `"` + name + `"`
}

// wakeMismatch warns when the card says another wake mode than notify; the
// card is informational for the channel, the wake file rules.
func (h *Harness) wakeMismatch() (string, error) {
	if h.cfg.Adapter != AdapterNotify {
		return "", nil
	}
	agents, err := h.store.Agents(h.member.Channel)
	if err != nil {
		return "", err
	}
	for i := range agents {
		if agents[i].Name != h.member.Name {
			continue
		}
		if w := agents[i].Card.Wake; w != bus.WakeNotify {
			if w == "" {
				w = bus.WakeManual
			}
			return fmt.Sprintf("wake_mismatch (card says %s, wake file says notify); to make the card say notify, stop this tb bus run, then: "+
				"tb bus invite %s --role %s --name %s, and tb bus join %s --invite <code> --name %s --wake notify (then restart the GUI MCP client: the join rotates the session token); "+
				"to follow the card instead, change adapter in %s",
				w, h.member.Channel, h.member.Role, h.member.Name, h.member.Channel, h.member.Name, h.wakeFilePath()), nil
		}
	}
	return "", nil
}

func (h *Harness) wakeFilePath() string {
	if h.opts.ConfigPath != "" {
		return h.opts.ConfigPath
	}
	p, err := ConfigPath(h.root, h.member.Channel, h.member.Name)
	if err != nil {
		return "the wake file"
	}
	return p
}
