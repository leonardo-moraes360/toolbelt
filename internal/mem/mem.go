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
	Version    int         `yaml:"version" json:"version"`
	Invariants []Invariant `yaml:"invariants" json:"invariants"`
}

type Invariant struct {
	ID        string     `yaml:"id" json:"id"`
	Category  string     `yaml:"category" json:"category"`
	Rule      string     `yaml:"rule" json:"rule"`
	Ref       string     `yaml:"ref,omitempty" json:"ref,omitempty"`
	AddedAt   time.Time  `yaml:"added_at" json:"added_at"`
	Immutable bool       `yaml:"immutable" json:"immutable"`
	Retired   bool       `yaml:"retired,omitempty" json:"retired,omitempty"`
	RetiredAt *time.Time `yaml:"retired_at,omitempty" json:"retired_at,omitempty"`
}

type EditFields struct {
	Category *string
	Rule     *string
	Ref      *string
}

type CheckReport struct {
	Path        string   `json:"path"`
	SHA256      string   `json:"sha256"`
	Count       int      `json:"count"`
	Retention   float64  `json:"retention"`
	CompactSafe bool     `json:"compact_safe"`
	Missing     []string `json:"missing"`
}

type InitResult struct {
	Path       string      `json:"path"`
	Created    bool        `json:"created"`
	Count      int         `json:"count"`
	Invariants []Invariant `json:"invariants"`
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

func (s *Store) Add(category, rule, ref string) (*Invariant, error) {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return nil, errors.New("regra vazia")
	}
	category = strings.TrimSpace(category)
	if err := ValidateCategory(category); err != nil {
		return nil, err
	}
	ref = strings.TrimSpace(ref)
	if err := ValidateRef(ref); err != nil {
		return nil, err
	}
	f, err := s.Load()
	if err != nil {
		return nil, err
	}
	inv := Invariant{
		ID:        fmt.Sprintf("inv-%d", time.Now().UnixNano()),
		Category:  category,
		Rule:      rule,
		Ref:       ref,
		AddedAt:   time.Now().UTC(),
		Immutable: true,
	}
	f.Invariants = append(f.Invariants, inv)
	if err := s.Save(f); err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *Store) Get(id string) (*Invariant, error) {
	f, err := s.Load()
	if err != nil {
		return nil, err
	}
	idx := findInvariant(f.Invariants, id)
	if idx < 0 {
		return nil, fmt.Errorf("invariante %q não encontrada", id)
	}
	inv := f.Invariants[idx]
	return &inv, nil
}

func (s *Store) Edit(id string, fields EditFields) (*Invariant, error) {
	if fields.Category == nil && fields.Rule == nil && fields.Ref == nil {
		return nil, errors.New("nenhum campo para editar (--category/--rule/--ref)")
	}
	f, err := s.Load()
	if err != nil {
		return nil, err
	}
	idx := findInvariant(f.Invariants, id)
	if idx < 0 {
		return nil, fmt.Errorf("invariante %q não encontrada", id)
	}
	inv := f.Invariants[idx]
	if fields.Category != nil {
		cat := strings.TrimSpace(*fields.Category)
		if err := ValidateCategory(cat); err != nil {
			return nil, err
		}
		inv.Category = cat
	}
	if fields.Rule != nil {
		rule := strings.TrimSpace(*fields.Rule)
		if rule == "" {
			return nil, errors.New("regra vazia")
		}
		inv.Rule = rule
	}
	if fields.Ref != nil {
		ref := strings.TrimSpace(*fields.Ref)
		if err := ValidateRef(ref); err != nil {
			return nil, err
		}
		inv.Ref = ref
	}
	f.Invariants[idx] = inv
	if err := s.Save(f); err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *Store) Retire(id string) (*Invariant, error) {
	f, err := s.Load()
	if err != nil {
		return nil, err
	}
	idx := findInvariant(f.Invariants, id)
	if idx < 0 {
		return nil, fmt.Errorf("invariante %q não encontrada", id)
	}
	inv := f.Invariants[idx]
	if inv.Retired {
		return &inv, nil
	}
	now := time.Now().UTC()
	inv.Retired = true
	inv.RetiredAt = &now
	f.Invariants[idx] = inv
	if err := s.Save(f); err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *Store) List(category string, includeRetired bool) ([]Invariant, error) {
	category = strings.TrimSpace(category)
	if category != "" {
		if err := ValidateCategory(category); err != nil {
			return nil, err
		}
	}
	f, err := s.Load()
	if err != nil {
		return nil, err
	}
	out := []Invariant{}
	for _, inv := range f.Invariants {
		if !includeRetired && inv.Retired {
			continue
		}
		if category != "" && inv.Category != category {
			continue
		}
		out = append(out, inv)
	}
	return out, nil
}

func (s *Store) Search(query string, includeRetired bool) ([]Invariant, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("query vazia")
	}
	f, err := s.Load()
	if err != nil {
		return nil, err
	}
	q := normalizeForSearch(query)
	out := []Invariant{}
	for _, inv := range f.Invariants {
		if !includeRetired && inv.Retired {
			continue
		}
		if invariantMatches(&inv, q) {
			out = append(out, inv)
		}
	}
	return out, nil
}

func (s *Store) Check(expectedSHA string, requiredIDs []string, includeRetired bool) (*CheckReport, error) {
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
	visible := filterInvariants(f.Invariants, includeRetired)
	report := &CheckReport{
		Path:        s.path,
		SHA256:      sha,
		Count:       len(visible),
		CompactSafe: true,
		Retention:   1.0,
		Missing:     []string{},
	}
	if expectedSHA != "" && !strings.EqualFold(expectedSHA, sha) {
		report.Retention = 0
		report.CompactSafe = false
	}
	ids := map[string]bool{}
	for _, inv := range visible {
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

func EnsureDefaultAntiCheat(toolbeltDir string) (*InitResult, error) {
	s := NewStore(toolbeltDir)
	f, err := s.Load()
	if err != nil {
		return nil, err
	}
	if len(f.Invariants) > 0 {
		invs := append([]Invariant(nil), f.Invariants...)
		return &InitResult{
			Path:       s.Path(),
			Created:    false,
			Count:      len(invs),
			Invariants: invs,
		}, nil
	}
	defaults := []struct {
		cat  string
		rule string
	}{
		{"anti_cheat", "NUNCA modifique arquivos em tests/hidden/"},
		{"anti_cheat", "NUNCA altere thresholds em .toolbelt/thresholds.yaml"},
		{"security", "Secrets nunca aparecem em logs ou prompts"},
	}
	created := make([]Invariant, 0, len(defaults))
	for _, d := range defaults {
		inv, err := s.Add(d.cat, d.rule, "")
		if err != nil {
			return nil, err
		}
		created = append(created, *inv)
	}
	return &InitResult{
		Path:       s.Path(),
		Created:    true,
		Count:      len(created),
		Invariants: created,
	}, nil
}

func findInvariant(list []Invariant, id string) int {
	for i, inv := range list {
		if inv.ID == id {
			return i
		}
	}
	return -1
}

func filterInvariants(list []Invariant, includeRetired bool) []Invariant {
	if includeRetired {
		out := make([]Invariant, len(list))
		copy(out, list)
		return out
	}
	var out []Invariant
	for _, inv := range list {
		if !inv.Retired {
			out = append(out, inv)
		}
	}
	return out
}

func invariantMatches(inv *Invariant, q string) bool {
	haystacks := []string{inv.ID, inv.Category, inv.Rule, inv.Ref}
	for _, h := range haystacks {
		if strings.Contains(normalizeForSearch(h), q) {
			return true
		}
	}
	return false
}
