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

type MutateOpts struct {
	Force  bool
	Reason string
	Who    string
}

type CheckReport struct {
	Path        string   `json:"path"`
	SHA256      string   `json:"sha256"`
	Count       int      `json:"count"`
	Retention   float64  `json:"retention"`
	CompactSafe bool     `json:"compact_safe"`
	Missing     []string `json:"missing"`
	Errors      []string `json:"errors"`
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
		return nil, fmt.Errorf("memory corrupted: %w", err)
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
	return os.WriteFile(s.path, data, 0o644) //nolint:gosec // invariants.yaml é versionável (dogfood #7)
}

func (s *Store) Add(category, rule, ref string, immutable bool) (*Invariant, error) {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return nil, errors.New("empty rule")
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
		Immutable: immutable,
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
		return nil, fmt.Errorf("invariant %q not found", id)
	}
	inv := f.Invariants[idx]
	return &inv, nil
}

func (s *Store) Edit(id string, fields EditFields, opts MutateOpts) (*Invariant, error) {
	if !editFieldsProvided(fields) {
		return nil, errors.New("no field to edit (--category/--rule/--ref)")
	}
	f, err := s.Load()
	if err != nil {
		return nil, err
	}
	idx := findInvariant(f.Invariants, id)
	if idx < 0 {
		return nil, fmt.Errorf("invariant %q not found", id)
	}
	old := f.Invariants[idx]
	if err := requireForce(&old, opts, "edit"); err != nil {
		return nil, err
	}
	inv := old
	if err := applyEditFields(&inv, fields); err != nil {
		return nil, err
	}
	f.Invariants[idx] = inv
	if err := s.Save(f); err != nil {
		return nil, err
	}
	if err := s.recordProtectedMutation(&old, "edit", id, opts); err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *Store) Retire(id string, opts MutateOpts) (*Invariant, error) {
	f, err := s.Load()
	if err != nil {
		return nil, err
	}
	idx := findInvariant(f.Invariants, id)
	if idx < 0 {
		return nil, fmt.Errorf("invariant %q not found", id)
	}
	old := f.Invariants[idx]
	if old.Retired {
		return &old, nil
	}
	if err := requireForce(&old, opts, "retire"); err != nil {
		return nil, err
	}
	inv := old
	now := time.Now().UTC()
	inv.Retired = true
	inv.RetiredAt = &now
	f.Invariants[idx] = inv
	if err := s.Save(f); err != nil {
		return nil, err
	}
	if err := s.recordProtectedMutation(&old, "retire", id, opts); err != nil {
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
		return nil, errors.New("empty query")
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
		return nil, fmt.Errorf("memory unreadable: %w", err)
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
		Errors:      []string{},
	}
	if err := collectUnevaluable(visible, report); err != nil {
		return report, err
	}
	applyIntegrity(report, expectedSHA, sha, visible, requiredIDs)
	if !report.CompactSafe {
		return report, integrityError(report, expectedSHA, sha)
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
	type seed struct {
		cat  string
		rule string
	}
	defaults := []seed{{"anti_cheat", "NEVER modify files in tests/hidden/"}}
	// Only point at thresholds.yaml when it exists: nothing creates it, and an
	// invariant about a missing file reads as broken setup (field study: paprika).
	// guard keeps protecting the path either way.
	if _, statErr := os.Stat(filepath.Join(toolbeltDir, "thresholds.yaml")); statErr == nil {
		defaults = append(defaults, seed{"anti_cheat", "NEVER change thresholds in .toolbelt/thresholds.yaml"})
	}
	defaults = append(defaults, seed{"security", "Secrets never appear in logs or prompts"})
	created := make([]Invariant, 0, len(defaults))
	for _, d := range defaults {
		inv, err := s.Add(d.cat, d.rule, "", true)
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

func isProtected(inv *Invariant) bool {
	return inv.Immutable || inv.Category == "anti_cheat"
}

func editFieldsProvided(fields EditFields) bool {
	return fields.Category != nil || fields.Rule != nil || fields.Ref != nil
}

func applyEditFields(inv *Invariant, fields EditFields) error {
	if fields.Category != nil {
		cat := strings.TrimSpace(*fields.Category)
		if err := ValidateCategory(cat); err != nil {
			return err
		}
		inv.Category = cat
	}
	if fields.Rule != nil {
		rule := strings.TrimSpace(*fields.Rule)
		if rule == "" {
			return errors.New("empty rule")
		}
		inv.Rule = rule
	}
	if fields.Ref != nil {
		ref := strings.TrimSpace(*fields.Ref)
		if err := ValidateRef(ref); err != nil {
			return err
		}
		inv.Ref = ref
	}
	return nil
}

func (s *Store) recordProtectedMutation(old *Invariant, action, id string, opts MutateOpts) error {
	if !isProtected(old) {
		return nil
	}
	oldCopy := *old
	return s.appendHistory(&HistoryEntry{
		Action:   action,
		ID:       id,
		Who:      opts.Who,
		Reason:   strings.TrimSpace(opts.Reason),
		OldValue: &oldCopy,
	})
}

func requireForce(inv *Invariant, opts MutateOpts, action string) error {
	if !isProtected(inv) {
		return nil
	}
	if !opts.Force {
		return fmt.Errorf("invariant %s protected (immutable/anti_cheat): use --force --reason to %s", inv.ID, action)
	}
	if strings.TrimSpace(opts.Reason) == "" {
		return fmt.Errorf("invariant %s protected: --reason is required with --force", inv.ID)
	}
	return nil
}

func evaluateInvariant(inv *Invariant) string {
	if strings.TrimSpace(inv.ID) == "" {
		return "invariant without id"
	}
	if strings.TrimSpace(inv.Rule) == "" {
		return fmt.Sprintf("%s: empty rule", inv.ID)
	}
	if strings.TrimSpace(inv.Category) == "" {
		return fmt.Sprintf("%s: empty category", inv.ID)
	}
	return ""
}

func collectUnevaluable(visible []Invariant, report *CheckReport) error {
	for i := range visible {
		if errMsg := evaluateInvariant(&visible[i]); errMsg != "" {
			report.Errors = append(report.Errors, errMsg)
		}
	}
	sort.Strings(report.Errors)
	if len(report.Errors) == 0 {
		return nil
	}
	report.CompactSafe = false
	report.Retention = 0
	return fmt.Errorf("rules not evaluable: %s", strings.Join(report.Errors, "; "))
}

func applyIntegrity(report *CheckReport, expectedSHA, sha string, visible []Invariant, requiredIDs []string) {
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
		report.CompactSafe = false
	}
}

func integrityError(report *CheckReport, expectedSHA, sha string) error {
	if len(report.Missing) > 0 {
		return fmt.Errorf("invariants missing: %v", report.Missing)
	}
	if expectedSHA != "" && !strings.EqualFold(expectedSHA, sha) {
		return fmt.Errorf("sha256 mismatch (expected %s, got %s)", expectedSHA, sha)
	}
	return errors.New("integrity compromised")
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
