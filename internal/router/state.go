package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

const (
	StateRunning = "running"
	StateStopped = "stopped"
)

// ErrNoState means no router has ever run in this repository.
var ErrNoState = errors.New("no router state found (run tb router run)")

func Dir(root string) string { return filepath.Join(root, ".toolbelt", "router") }

func StatePath(root string) string { return filepath.Join(Dir(root), "state.json") }

func AlertsPath(root string) string { return filepath.Join(Dir(root), "alerts.jsonl") }

func LockPath(root string) string { return filepath.Join(Dir(root), "lock") }

func ControlDir(root string) string { return filepath.Join(Dir(root), "control") }

func LogsDir(root string) string { return filepath.Join(Dir(root), "logs") }

type WorkerState struct {
	ID            string     `json:"id"`
	Status        string     `json:"status"` // idle | working
	Ticket        string     `json:"ticket,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	Subscriptions []string   `json:"subscriptions"`
	// Bus slots: the attempt thread, the claiming agent and the phase.
	Thread    string     `json:"thread,omitempty"`
	Agent     string     `json:"agent,omitempty"`
	Phase     string     `json:"phase,omitempty"`
	ClaimedAt *time.Time `json:"claimed_at,omitempty"`
}

type QueueState struct {
	Ready   int `json:"ready"`
	Blocked int `json:"blocked"`
	Running int `json:"running"`
}

type State struct {
	PID       int           `json:"pid"`
	StartedAt time.Time     `json:"started_at"`
	StoppedAt *time.Time    `json:"stopped_at,omitempty"`
	Status    string        `json:"status"` // running | stopped
	Strategy  string        `json:"strategy"`
	Paused    bool          `json:"paused"`
	Draining  bool          `json:"draining"`
	Workers   []WorkerState `json:"workers"`
	Queue     QueueState    `json:"queue"`
}

type StatusReport struct {
	State
	Alive bool `json:"alive"`
	Stale bool `json:"stale"` // state says running but the PID no longer exists
}

func ReadState(root string) (*State, error) {
	data, err := os.ReadFile(StatePath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoState
	}
	if err != nil {
		return nil, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("state.json unreadable: %w", err)
	}
	return &st, nil
}

func LoadStatus(root string) (*StatusReport, error) {
	st, err := ReadState(root)
	if err != nil {
		return nil, err
	}
	rep := &StatusReport{State: *st}
	rep.Alive = st.Status == StateRunning && PidAlive(st.PID)
	rep.Stale = st.Status == StateRunning && !rep.Alive
	return rep, nil
}

func writeState(root string, st *State) ([]byte, error) {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		return nil, err
	}
	return data, ticket.WriteFileAtomic(StatePath(root), data)
}
