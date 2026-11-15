package mem

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultInvariantsFile = "invariants.yaml"

type File struct {
	Version    int         `yaml:"version"`
	Invariants []Invariant `yaml:"invariants"`
}

type Invariant struct {
	ID        string    `yaml:"id"`
	Category  string    `yaml:"category"`
	Rule      string    `yaml:"rule"`
	AddedAt   time.Time `yaml:"added_at"`
	Immutable bool      `yaml:"immutable"`
}

type CheckReport struct {
	Path        string   `json:"path"`
	SHA256      string   `json:"sha256"`
	Count       int      `json:"count"`
	Retention   float64  `json:"retention"`
	CompactSafe bool     `json:"compact_safe"`
	Missing     []string `json:"missing,omitempty"`
}

type Store struct {
	path string
}

func NewStore(toolbeltDir string) *Store {
	return &Store{path: filepath.Join(toolbeltDir, DefaultInvariantsFile)}
}

func LocateRepoToolbeltDir(start string) string {
	if start == "" {
		start, _ = os.Getwd()
	}
	dir := start
	for {
		candidate := filepath.Join(dir, ".toolbelt")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Join(start, ".toolbelt")
		}
		dir = parent
	}
}

func (s *Store) Load() (*File, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return &File{Version: 1, Invariants: []Invariant{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if f.Version == 0 {
		f.Version = 1
	}
	return &f, nil
}

func (s *Store) Save(f *File) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

func (s *Store) Add(category, rule string) (*Invariant, error) {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return nil, errors.New("regra vazia")
	}
	f, err := s.Load()
	if err != nil {
		return nil, err
	}
	inv := Invariant{
		ID:        fmt.Sprintf("inv-%d", time.Now().UnixNano()),
		Category:  category,
		Rule:      rule,
		AddedAt:   time.Now().UTC(),
		Immutable: true,
	}
	f.Invariants = append(f.Invariants, inv)
	if err := s.Save(f); err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *Store) List(category string) ([]Invariant, error) {
	f, err := s.Load()
	if err != nil {
		return nil, err
	}
	if category == "" {
		return f.Invariants, nil
	}
	var out []Invariant
	for _, inv := range f.Invariants {
		if inv.Category == category {
			out = append(out, inv)
		}
	}
	return out, nil
}

func (s *Store) Check(expectedSHA string, requiredIDs []string) (*CheckReport, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	f, err := s.Load()
	if err != nil {
		return nil, err
	}
	report := &CheckReport{
		Path:        s.path,
		SHA256:      sha,
		Count:       len(f.Invariants),
		CompactSafe: true,
		Retention:   1.0,
	}
	if expectedSHA != "" && !strings.EqualFold(expectedSHA, sha) {
		report.Retention = 0
		report.CompactSafe = false
	}
	ids := map[string]bool{}
	for _, inv := range f.Invariants {
		ids[inv.ID] = true
	}
	for _, id := range requiredIDs {
		if !ids[id] {
			report.Missing = append(report.Missing, id)
		}
	}
	sort.Strings(report.Missing)
	if len(report.Missing) > 0 && len(requiredIDs) > 0 {
		report.Retention = float64(len(requiredIDs)-len(report.Missing)) / float64(len(requiredIDs))
	}
	return report, nil
}

func (s *Store) Path() string {
	return s.path
}

func EnsureDefaultAntiCheat(toolbeltDir string) error {
	s := NewStore(toolbeltDir)
	f, err := s.Load()
	if err != nil {
		return err
	}
	if len(f.Invariants) > 0 {
		return nil
	}
	defaults := []struct {
		cat  string
		rule string
	}{
		{"anti_cheat", "NUNCA modifique arquivos em tests/hidden/"},
		{"anti_cheat", "NUNCA altere thresholds em .toolbelt/thresholds.yaml"},
		{"security", "Secrets nunca aparecem em logs ou prompts"},
	}
	for _, d := range defaults {
		_, err := s.Add(d.cat, d.rule)
		if err != nil {
			return err
		}
	}
	return nil
}
