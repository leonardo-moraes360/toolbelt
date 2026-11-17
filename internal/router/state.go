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

// Estados do router em state.json.
const (
	StateRunning = "running"
	StateStopped = "stopped"
)

// ErrNoState indica que nenhum router rodou neste repositório.
var ErrNoState = errors.New("nenhum estado do router encontrado (rode tb router run)")

// Dir devolve .toolbelt/router.
func Dir(root string) string { return filepath.Join(root, ".toolbelt", "router") }

// StatePath devolve o caminho de state.json.
func StatePath(root string) string { return filepath.Join(Dir(root), "state.json") }

// AlertsPath devolve o caminho de alerts.jsonl.
func AlertsPath(root string) string { return filepath.Join(Dir(root), "alerts.jsonl") }

// LockPath devolve o arquivo de lock exclusivo do router.
func LockPath(root string) string { return filepath.Join(Dir(root), "lock") }

// ControlDir devolve o diretório de pedidos de controle.
func ControlDir(root string) string { return filepath.Join(Dir(root), "control") }

// LogsDir devolve o diretório de logs por ticket.
func LogsDir(root string) string { return filepath.Join(Dir(root), "logs") }

// WorkerState descreve um slot de worker.
type WorkerState struct {
	ID            string     `json:"id"`
	Status        string     `json:"status"` // idle | working
	Ticket        string     `json:"ticket,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	Subscriptions []string   `json:"subscriptions"`
}

// QueueState resume a fila.
type QueueState struct {
	Ready   int `json:"ready"`
	Blocked int `json:"blocked"`
	Running int `json:"running"`
}

// State é o conteúdo de state.json.
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

// StatusReport é State acrescido de leitura de liveness.
type StatusReport struct {
	State
	Alive bool `json:"alive"`
	Stale bool `json:"stale"` // state diz running mas o PID não existe mais
}

// ReadState lê state.json.
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
		return nil, fmt.Errorf("state.json ilegível: %w", err)
	}
	return &st, nil
}

// LoadStatus lê o estado e verifica se o PID ainda existe.
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
