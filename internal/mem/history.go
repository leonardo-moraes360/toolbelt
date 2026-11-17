package mem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const DefaultHistoryFile = "invariants_history.jsonl"

type HistoryEntry struct {
	At       time.Time  `json:"at"`
	Action   string     `json:"action"`
	ID       string     `json:"id"`
	Who      string     `json:"who"`
	Reason   string     `json:"reason"`
	OldValue *Invariant `json:"old_value"`
}

func (s *Store) historyPath() string {
	return filepath.Join(filepath.Dir(s.path), DefaultHistoryFile)
}

func (s *Store) appendHistory(entry *HistoryEntry) error {
	if entry.At.IsZero() {
		entry.At = time.Now().UTC()
	}
	if entry.Who == "" {
		entry.Who = currentWho()
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	path := s.historyPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func currentWho() string {
	for _, key := range []string{"USER", "USERNAME", "LOGNAME"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return "unknown"
}
