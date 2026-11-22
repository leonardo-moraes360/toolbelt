package ticket

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var ErrNotFound = errors.New("ticket not found")

// Store persists tickets in <root>/.toolbelt/tickets/TKT-NNN.yaml.
type Store struct {
	root string
	dir  string
	now  func() time.Time
}

// Open expects root to be the directory containing .toolbelt.
func Open(root string) *Store {
	return &Store{root: root, dir: filepath.Join(root, ".toolbelt", "tickets"), now: time.Now}
}

// SetClock overrides the clock (tests).
func (s *Store) SetClock(now func() time.Time) { s.now = now }

func (s *Store) Root() string { return s.root }

func (s *Store) Dir() string { return s.dir }

func (s *Store) Path(id string) string { return filepath.Join(s.dir, id+".yaml") }

type Filter struct {
	Statuses  []Status
	Worker    string
	DependsOn string // tickets that depend on this ID
}

func (s *Store) lock() (func(), error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	return acquireLock(filepath.Join(s.dir, ".lock"))
}

func (s *Store) Create(in *Ticket) (Ticket, error) {
	t := *in // copy: the caller does not see the ID/Status allocated here
	if err := t.Validate(); err != nil {
		return Ticket{}, err
	}
	unlock, err := s.lock()
	if err != nil {
		return Ticket{}, err
	}
	defer unlock()

	all, err := s.loadAll()
	if err != nil {
		return Ticket{}, err
	}
	maxN := 0
	byID := map[string]Ticket{}
	for i := range all {
		x := &all[i]
		byID[x.ID] = *x
		if n := idNumber(x.ID); n > maxN {
			maxN = n
		}
	}
	t.ID = FormatID(maxN + 1)
	deps, err := normalizeDeps(t.Dependencies)
	if err != nil {
		return Ticket{}, err
	}
	t.Dependencies = deps
	byID[t.ID] = t
	if err := checkDeps(&t, byID); err != nil {
		return Ticket{}, err
	}
	now := s.now().UTC()
	t.Status = StatusPending
	t.CreatedAt, t.UpdatedAt = now, now
	if len(t.Mandate.Permissions) == 0 {
		t.Mandate.Permissions = DefaultPermissions()
	}
	if err := s.write(&t); err != nil {
		return Ticket{}, err
	}
	return t, nil
}

func (s *Store) Get(id string) (Ticket, error) {
	nid, err := NormalizeID(id)
	if err != nil {
		return Ticket{}, err
	}
	return s.read(nid)
}

// List returns tickets sorted by ID.
func (s *Store) List(f Filter) ([]Ticket, error) {
	all, err := s.loadAll()
	if err != nil {
		return nil, err
	}
	dep := ""
	if f.DependsOn != "" {
		if dep, err = NormalizeID(f.DependsOn); err != nil {
			return nil, err
		}
	}
	var out []Ticket
	for i := range all {
		t := &all[i]
		if len(f.Statuses) > 0 && !hasStatus(f.Statuses, t.Status) {
			continue
		}
		if f.Worker != "" && t.Worker != f.Worker {
			continue
		}
		if dep != "" && !contains(t.Dependencies, dep) {
			continue
		}
		out = append(out, *t)
	}
	return out, nil
}

// Update does read-modify-write under the lock. Status changes are validated
// against the state machine; dependencies are revalidated.
func (s *Store) Update(id string, fn func(*Ticket) error) (Ticket, error) {
	nid, err := NormalizeID(id)
	if err != nil {
		return Ticket{}, err
	}
	unlock, err := s.lock()
	if err != nil {
		return Ticket{}, err
	}
	defer unlock()

	cur, err := s.read(nid)
	if err != nil {
		return Ticket{}, err
	}
	next := cur.clone()
	if err := fn(&next); err != nil {
		return Ticket{}, err
	}
	if next.ID != cur.ID {
		return Ticket{}, fmt.Errorf("ticket ID is immutable")
	}
	if err := next.Validate(); err != nil {
		return Ticket{}, err
	}
	if next.Status != cur.Status && !CanTransition(cur.Status, next.Status) {
		return Ticket{}, fmt.Errorf("invalid transition %s -> %s for %s", cur.Status, next.Status, cur.ID)
	}
	if !equalStrings(cur.Dependencies, next.Dependencies) {
		deps, err := normalizeDeps(next.Dependencies)
		if err != nil {
			return Ticket{}, err
		}
		next.Dependencies = deps
		all, err := s.loadAll()
		if err != nil {
			return Ticket{}, err
		}
		byID := map[string]Ticket{}
		for i := range all {
			byID[all[i].ID] = all[i]
		}
		byID[next.ID] = next
		if err := checkDeps(&next, byID); err != nil {
			return Ticket{}, err
		}
	}
	next.UpdatedAt = s.now().UTC()
	if err := s.write(&next); err != nil {
		return Ticket{}, err
	}
	return next, nil
}

func (s *Store) Transition(id string, to Status) (Ticket, error) {
	return s.Update(id, func(t *Ticket) error {
		t.Status = to
		return nil
	})
}

// Cancel only applies to pending/running tickets.
func (s *Store) Cancel(id string) (Ticket, error) {
	return s.Update(id, func(t *Ticket) error {
		if t.Status != StatusPending && t.Status != StatusRunning {
			return fmt.Errorf("ticket %s is %s; only pending or running can be canceled", t.ID, t.Status)
		}
		t.Status = StatusCanceled
		t.Subscription = nil
		return nil
	})
}

// Retry returns a failed/canceled ticket to pending, clearing worker and
// subscription (Attempts is preserved).
func (s *Store) Retry(id string) (Ticket, error) {
	return s.Update(id, func(t *Ticket) error {
		if t.Status != StatusFailed && t.Status != StatusCanceled {
			return fmt.Errorf("ticket %s is %s; retry only applies to failed or canceled", t.ID, t.Status)
		}
		t.Status = StatusPending
		t.Worker = ""
		t.Subscription = nil
		return nil
	})
}

func (s *Store) read(id string) (Ticket, error) {
	data, err := os.ReadFile(s.Path(id))
	if errors.Is(err, os.ErrNotExist) {
		return Ticket{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if err != nil {
		return Ticket{}, err
	}
	var t Ticket
	if err := yaml.Unmarshal(data, &t); err != nil {
		return Ticket{}, fmt.Errorf("ticket %s unreadable: %w", id, err)
	}
	if t.Version > FormatVersion {
		return Ticket{}, fmt.Errorf("ticket %s unreadable: format version %d is newer than this tb understands (%d); upgrade tb", id, t.Version, FormatVersion)
	}
	if t.ID != id {
		return Ticket{}, fmt.Errorf("ticket %s unreadable: id %q in YAML differs from file name %q", id, t.ID, id)
	}
	return t, nil
}

func (s *Store) loadAll() ([]Ticket, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Ticket
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".yaml") {
			continue
		}
		id := strings.TrimSuffix(name, ".yaml")
		if !idRe.MatchString(id) {
			continue
		}
		t, err := s.read(id)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return idNumber(out[i].ID) < idNumber(out[j].ID) })
	return out, nil
}

func (s *Store) write(t *Ticket) error {
	t.Version = FormatVersion
	data, err := yaml.Marshal(t)
	if err != nil {
		return err
	}
	return WriteFileAtomic(s.Path(t.ID), data)
}

// WriteFileAtomic writes via a temp file + rename, with mode 0600.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(name, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

func normalizeDeps(deps []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, d := range deps {
		n, err := NormalizeID(d)
		if err != nil {
			return nil, err
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out, nil
}

// checkDeps rejects unknown dependencies, self-dependencies and cycles
// through t.
func checkDeps(t *Ticket, byID map[string]Ticket) error {
	for _, d := range t.Dependencies {
		if d == t.ID {
			return fmt.Errorf("ticket %s cannot depend on itself", t.ID)
		}
		if _, ok := byID[d]; !ok {
			return fmt.Errorf("unknown dependency: %s", d)
		}
	}
	all := make([]Ticket, 0, len(byID))
	for id := range byID {
		all = append(all, byID[id])
	}
	for _, cyc := range FindCycles(all) {
		if contains(cyc, t.ID) {
			return fmt.Errorf("dependency cycle: %s", strings.Join(cyc, " -> "))
		}
	}
	return nil
}

func hasStatus(list []Status, s Status) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// CheckNew validates t as if it were created now (dependencies and cycles), without writing.
func (s *Store) CheckNew(in *Ticket) error {
	t := *in
	if err := t.Validate(); err != nil {
		return err
	}
	all, err := s.loadAll()
	if err != nil {
		return err
	}
	byID := map[string]Ticket{}
	maxN := 0
	for i := range all {
		x := &all[i]
		byID[x.ID] = *x
		if n := idNumber(x.ID); n > maxN {
			maxN = n
		}
	}
	t.ID = FormatID(maxN + 1)
	if t.Dependencies, err = normalizeDeps(t.Dependencies); err != nil {
		return err
	}
	byID[t.ID] = t
	return checkDeps(&t, byID)
}

// clone deep-copies t (slices and pointers) so Update callbacks cannot alter
// the "current" state through aliasing.
func (t *Ticket) clone() Ticket {
	c := *t
	c.Mandate.Scope = append([]string(nil), t.Mandate.Scope...)
	c.Mandate.Forbidden = append([]string(nil), t.Mandate.Forbidden...)
	c.Mandate.Permissions = append([]string(nil), t.Mandate.Permissions...)
	c.Acceptance.Criteria = append([]string(nil), t.Acceptance.Criteria...)
	c.Dependencies = append([]string(nil), t.Dependencies...)
	if t.Subscription != nil {
		v := *t.Subscription
		c.Subscription = &v
	}
	if t.Result != nil {
		v := *t.Result
		if v.StartedAt != nil {
			x := *v.StartedAt
			v.StartedAt = &x
		}
		if v.FinishedAt != nil {
			x := *v.FinishedAt
			v.FinishedAt = &x
		}
		c.Result = &v
	}
	if t.MergedAt != nil {
		x := *t.MergedAt
		c.MergedAt = &x
	}
	return c
}
