package egm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const stateVersion = 1

// Lineage is the stable identity of a finding across rounds (R19–R22).
type Lineage struct {
	ID        string     `json:"id"`
	Severity  string     `json:"severity"`
	Owner     string     `json:"owner"`
	Persona   string     `json:"persona,omitempty"`
	Reviewers []string   `json:"reviewers"`
	Key       string     `json:"key,omitempty"`
	Status    string     `json:"status"`
	Confirmed bool       `json:"confirmed"`
	Contests  int        `json:"contests"`
	Scope     Scope      `json:"scope"`
	Message   string     `json:"message"`
	Rationale string     `json:"rationale,omitempty"`
	Evidence  []Evidence `json:"evidence,omitempty"`
	Cited     []string   `json:"cited,omitempty"`
	Context   string     `json:"context,omitempty"`
	MatchSHA  string     `json:"match_sha,omitempty"`
	// Downgrade is the lineage downgrade (R24): forced, forced_blocker or
	// declined_major.
	Downgrade   string `json:"downgrade,omitempty"`
	OriginRound int    `json:"origin_round"`
	OpenedRound int    `json:"opened_round"`
	// RefRound is the round of the last status transition, the reference of
	// the --fix change test (R28).
	RefRound int `json:"ref_round"`
	// ScopeRound is the round whose snapshot Scope's lines refer to.
	ScopeRound  int      `json:"scope_round"`
	Occurrences []string `json:"occurrences"`
	Unverified  bool     `json:"unverified,omitempty"`
}

// ReviewerRun is a reviewer's status in a round (glossary).
type ReviewerRun struct {
	Name   string `json:"name"`
	Status string `json:"status"` // completed | not_run | unavailable | skipped
	Reason string `json:"reason,omitempty"`
}

// Round is one review round (R21).
type Round struct {
	N            int               `json:"n"`
	Cycle        int               `json:"cycle"`
	Content      string            `json:"content"`
	PolicySHA    string            `json:"policy_sha"`
	DocSHA256    string            `json:"doc_sha256"`
	HeadCommit   string            `json:"head_commit,omitempty"`
	Sources      map[string]string `json:"sources"`
	Engine       string            `json:"engine"`
	EngineReason string            `json:"engine_reason,omitempty"`
	Required     []string          `json:"required"`
	Reviewers    []ReviewerRun     `json:"reviewers"`
	ArgvSHA256   string            `json:"argv_sha256,omitempty"`
	Clean        bool              `json:"clean"`
	// Downgrades are the cycle downgrades this round contributes (R24).
	Downgrades []string  `json:"downgrades,omitempty"`
	Warnings   []Warning `json:"warnings,omitempty"`
}

// Cycle groups rounds since the last convergence, review or --restart (R23).
type Cycle struct {
	N          int  `json:"n"`
	FirstRound int  `json:"first_round"`
	Restart    bool `json:"restart,omitempty"`
	Capped     bool `json:"capped,omitempty"`
	Converged  bool `json:"converged,omitempty"`
	AgentRuns  int  `json:"agent_runs"`
}

// Transition records a lineage status change (R21).
type Transition struct {
	Command string `json:"command"`
	Lineage string `json:"lineage"`
	From    string `json:"from"`
	To      string `json:"to"`
	Round   int    `json:"round"`
	Reason  string `json:"reason,omitempty"`
	By      string `json:"by"` // elephant | reviewer | rerun | arbiter
}

// Convergence ties a content to a convergence (R23).
type Convergence struct {
	Content    string   `json:"content"`
	PolicySHA  string   `json:"policy_sha"`
	Rounds     []int    `json:"rounds"`
	Cycle      int      `json:"cycle"`
	Downgrades []string `json:"downgrades,omitempty"`
}

// State is state.json (R21).
type State struct {
	Version      int               `json:"version"`
	Doc          string            `json:"doc"`
	Slug         string            `json:"slug"`
	Rounds       []Round           `json:"rounds"`
	Cycles       []Cycle           `json:"cycles"`
	Lineages     []*Lineage        `json:"lineages"`
	Occurrences  map[string]string `json:"occurrences"`
	Transitions  []Transition      `json:"transitions"`
	Convergences []Convergence     `json:"convergences"`
	NextLineage  int               `json:"next_lineage"`
	NextResolve  int               `json:"next_resolve"`
}

func newState(doc, slug string) *State {
	return &State{Version: stateVersion, Doc: doc, Slug: slug, Occurrences: map[string]string{}}
}

func (s *State) lastRound() *Round {
	if len(s.Rounds) == 0 {
		return nil
	}
	return &s.Rounds[len(s.Rounds)-1]
}

func (s *State) cycle() *Cycle {
	if len(s.Cycles) == 0 {
		return nil
	}
	return &s.Cycles[len(s.Cycles)-1]
}

func (s *State) lineage(id string) *Lineage {
	for _, l := range s.Lineages {
		if l.ID == id {
			return l
		}
	}
	return nil
}

func (s *State) newLineageID() string {
	s.NextLineage++
	return fmt.Sprintf("F-%d", s.NextLineage)
}

// cycleRounds returns the rounds of cycle c, in order.
func (s *State) cycleRounds(c int) []*Round {
	var out []*Round
	for i := range s.Rounds {
		if s.Rounds[i].Cycle == c {
			out = append(out, &s.Rounds[i])
		}
	}
	return out
}

// StateRoot is <state> (glossary): $XDG_STATE_HOME/tb, ~/.local/state/tb, or
// os.UserConfigDir()/toolbelt/state on macOS.
func StateRoot() (string, error) {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" && filepath.IsAbs(x) {
		return filepath.Join(x, "tb"), nil
	}
	if runtime.GOOS == "darwin" {
		d, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(d, "toolbelt", "state"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "tb"), nil
}

// StateDir is <state>/egm/<repo-id>/<slug> (R21).
func StateDir(repoID, slug string) (string, error) {
	root, err := StateRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "egm", repoID, slug), nil
}

// loadState returns nil, nil when there is no state (NO_STATE).
func loadState(dir string) (*State, error) {
	b, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("state.json: %w", err)
	}
	if s.Version != stateVersion {
		return nil, fmt.Errorf("state.json: unsupported version %d", s.Version)
	}
	if s.Occurrences == nil {
		s.Occurrences = map[string]string{}
	}
	return &s, nil
}

func saveState(dir string, s *State) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(dir, "state.json", append(b, '\n'))
}

// writeAtomic writes dir/name through a temporary file, fsync, rename and an
// fsync of the directory, so a crash leaves the previous file valid (R21).
func writeAtomic(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+name+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) //nolint:errcheck // no-op after the rename
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
		return err
	}
	return syncDir(dir)
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := d.Sync(); err != nil && runtime.GOOS != "windows" {
		return err
	}
	return nil
}
