package busrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
	"github.com/leonardo-moraes360/toolbelt/internal/mem"
)

// RunDir is <private>/run/<channel>, outside the repo (see bus.PrivateDir).
func RunDir(root, channel string) (string, error) {
	priv, err := bus.PrivateDir(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(priv, "run", channel), nil
}

type State struct {
	NotifiedSeq int64     `json:"notified_seq"`
	NotifiedAt  time.Time `json:"notified_at,omitempty"`
	Rewaked     bool      `json:"rewaked,omitempty"`
	Unread      bool      `json:"unread,omitempty"`
	Read        bool      `json:"read,omitempty"`
	FailedSeq   int64     `json:"failed_seq,omitempty"`
	Session     string    `json:"session,omitempty"`
	// harness.session from the config on the last run: if the human changes
	// the config, it wins over the saved session.
	ConfigSession string      `json:"config_session,omitempty"`
	Wakes         []time.Time `json:"wakes,omitempty"`
}

func loadState(path string) (State, error) {
	var st State
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, fmt.Errorf("harness state %s: %w", path, err)
	}
	return st, nil
}

func saveState(path string, st *State) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// allowWake applies the sliding-window budget and records the wake.
func (st *State) allowWake(now time.Time, b Budget) bool {
	kept := st.Wakes[:0]
	for _, t := range st.Wakes {
		if now.Sub(t) < b.Per {
			kept = append(kept, t)
		}
	}
	st.Wakes = kept
	if len(st.Wakes) >= b.MaxWakes {
		return false
	}
	st.Wakes = append(st.Wakes, now)
	return true
}

type Event struct {
	Time       time.Time `json:"time"`
	Event      string    `json:"event"`
	Backend    string    `json:"backend,omitempty"` // notify: desktop | terminal
	FirstSeq   int64     `json:"first_seq,omitempty"`
	LastSeq    int64     `json:"last_seq,omitempty"`
	Attempt    int       `json:"attempt,omitempty"`
	ExitCode   *int      `json:"exit_code,omitempty"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	Detail     string    `json:"detail,omitempty"`
}

const maxDetail = 4 << 10

type auditLog struct{ path string }

func (a auditLog) write(ev *Event) error {
	ev.Detail = mem.RedactText(ev.Detail) // before truncating: the cut must not split a secret
	if len(ev.Detail) > maxDetail {
		ev.Detail = ev.Detail[len(ev.Detail)-maxDetail:]
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(a.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}
