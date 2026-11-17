package ticket

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s := Open(t.TempDir())
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	n := 0
	s.SetClock(func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		n++
		return base.Add(time.Duration(n) * time.Second)
	})
	return s
}

func mk(t *testing.T, s *Store, title string, mod func(*Ticket)) Ticket {
	t.Helper()
	tk := Ticket{Title: title}
	if mod != nil {
		mod(&tk)
	}
	out, err := s.Create(&tk)
	if err != nil {
		t.Fatalf("create %q: %v", title, err)
	}
	return out
}

func TestCreateAllocatesIDsAndDefaults(t *testing.T) {
	s := newStore(t)
	a := mk(t, s, "a", nil)
	b := mk(t, s, "b", nil)
	if a.ID != "TKT-001" || b.ID != "TKT-002" {
		t.Fatalf("ids = %s, %s", a.ID, b.ID)
	}
	if a.Status != StatusPending {
		t.Fatalf("status = %s", a.Status)
	}
	if !reflect.DeepEqual(a.Mandate.Permissions, DefaultPermissions()) {
		t.Fatalf("perms = %v", a.Mandate.Permissions)
	}
	st, err := os.Stat(s.Path(a.ID))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v", st.Mode().Perm())
	}
	got, err := s.Get("2")
	if err != nil || got.ID != "TKT-002" {
		t.Fatalf("get: %v %v", got.ID, err)
	}
	if _, err := s.Get("TKT-099"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if _, err := s.Get("../x"); err == nil {
		t.Fatal("want error for invalid id")
	}
}

func TestCreateValidation(t *testing.T) {
	s := newStore(t)
	mk(t, s, "base", nil)
	cases := []struct {
		name string
		tk   Ticket
		want string
	}{
		{"sem título", Ticket{}, "título"},
		{"dep desconhecida", Ticket{Title: "x", Dependencies: []string{"TKT-009"}}, "desconhecida"},
		{"dep malformada", Ticket{Title: "x", Dependencies: []string{"abc"}}, "inválido"},
		{"timeout inválido", Ticket{Title: "x", Acceptance: Acceptance{Timeout: "abc"}}, "timeout"},
		{"permissão inválida", Ticket{Title: "x", Mandate: Mandate{Permissions: []string{"root"}}}, "permissão"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := s.Create(&c.tk)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want contains %q", err, c.want)
			}
		})
	}
	list, _ := s.List(Filter{})
	if len(list) != 1 {
		t.Fatalf("falhas não devem gravar tickets, len = %d", len(list))
	}
}

func TestTransitions(t *testing.T) {
	ok := [][2]Status{
		{StatusPending, StatusRunning}, {StatusPending, StatusCanceled},
		{StatusRunning, StatusDone}, {StatusRunning, StatusFailed},
		{StatusRunning, StatusPending}, {StatusRunning, StatusCanceled},
		{StatusFailed, StatusPending}, {StatusCanceled, StatusPending},
	}
	for _, p := range ok {
		if !CanTransition(p[0], p[1]) {
			t.Errorf("%s -> %s deveria ser válido", p[0], p[1])
		}
	}
	bad := [][2]Status{
		{StatusPending, StatusDone}, {StatusPending, StatusFailed},
		{StatusDone, StatusPending}, {StatusDone, StatusFailed},
		{StatusFailed, StatusRunning}, {StatusCanceled, StatusRunning},
	}
	for _, p := range bad {
		if CanTransition(p[0], p[1]) {
			t.Errorf("%s -> %s deveria ser inválido", p[0], p[1])
		}
	}
	s := newStore(t)
	a := mk(t, s, "a", nil)
	if _, err := s.Transition(a.ID, StatusDone); err == nil {
		t.Fatal("pending->done deveria falhar")
	}
	if _, err := s.Transition(a.ID, StatusRunning); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Transition(a.ID, StatusFailed); err != nil {
		t.Fatal(err)
	}
	r, err := s.Retry(a.ID)
	if err != nil || r.Status != StatusPending {
		t.Fatalf("retry: %v %v", r.Status, err)
	}
	if _, err := s.Retry(a.ID); err == nil {
		t.Fatal("retry em pending deveria falhar")
	}
	c, err := s.Cancel(a.ID)
	if err != nil || c.Status != StatusCanceled {
		t.Fatalf("cancel: %v %v", c.Status, err)
	}
	if _, err := s.Retry(a.ID); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateRejectsCycleAndUnknownDep(t *testing.T) {
	s := newStore(t)
	a := mk(t, s, "a", nil)
	b := mk(t, s, "b", func(x *Ticket) { x.Dependencies = []string{a.ID} })
	c := mk(t, s, "c", func(x *Ticket) { x.Dependencies = []string{b.ID} })
	_, err := s.Update(a.ID, func(x *Ticket) error { x.Dependencies = []string{c.ID}; return nil })
	if err == nil || !strings.Contains(err.Error(), "ciclo") {
		t.Fatalf("want ciclo, got %v", err)
	}
	_, err = s.Update(a.ID, func(x *Ticket) error { x.Dependencies = []string{a.ID}; return nil })
	if err == nil {
		t.Fatal("auto-dependência deveria falhar")
	}
	_, err = s.Update(a.ID, func(x *Ticket) error { x.Dependencies = []string{"TKT-777"}; return nil })
	if err == nil || !strings.Contains(err.Error(), "desconhecida") {
		t.Fatalf("want desconhecida, got %v", err)
	}
	got, _ := s.Get(a.ID)
	if len(got.Dependencies) != 0 {
		t.Fatalf("estado deveria permanecer intacto: %v", got.Dependencies)
	}
}

func TestListFilters(t *testing.T) {
	s := newStore(t)
	a := mk(t, s, "a", nil)
	b := mk(t, s, "b", func(x *Ticket) { x.Dependencies = []string{a.ID} })
	mk(t, s, "c", nil)
	if _, err := s.Update(a.ID, func(x *Ticket) error { x.Status = StatusRunning; x.Worker = "w1"; return nil }); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		f    Filter
		want []string
	}{
		{"todos", Filter{}, []string{"TKT-001", "TKT-002", "TKT-003"}},
		{"status", Filter{Statuses: []Status{StatusPending}}, []string{"TKT-002", "TKT-003"}},
		{"worker", Filter{Worker: "w1"}, []string{"TKT-001"}},
		{"deps", Filter{DependsOn: a.ID}, []string{b.ID}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := s.List(c.f)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, x := range got {
				ids = append(ids, x.ID)
			}
			if !reflect.DeepEqual(ids, c.want) {
				t.Fatalf("ids = %v, want %v", ids, c.want)
			}
		})
	}
}

func TestConcurrentCreateUniqueIDs(t *testing.T) {
	s := Open(t.TempDir())
	const n = 20
	var wg sync.WaitGroup
	ids := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tk, err := s.Create(&Ticket{Title: "x"})
			if err != nil {
				t.Error(err)
				return
			}
			ids <- tk.ID
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("ID duplicado %s", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Fatalf("len = %d", len(seen))
	}
	entries, _ := os.ReadDir(s.Dir())
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Fatalf("temp residual: %s", e.Name())
		}
	}
}

func TestBoardOrdering(t *testing.T) {
	s := newStore(t)
	a := mk(t, s, "a", func(x *Ticket) { x.Priority = 1 })                                  // depth 0
	b := mk(t, s, "b", func(x *Ticket) { x.Priority = 9; x.Dependencies = []string{a.ID} }) // depth 1
	c := mk(t, s, "c", func(x *Ticket) { x.Priority = 5 })                                  // depth 0
	d := mk(t, s, "d", func(x *Ticket) { x.Dependencies = []string{b.ID} })                 // blocked
	// a done => b ready
	for _, st := range []Status{StatusRunning, StatusDone} {
		if _, err := s.Transition(a.ID, st); err != nil {
			t.Fatal(err)
		}
	}
	bd, err := LoadBoard(s)
	if err != nil {
		t.Fatal(err)
	}
	ids := func(l []Ticket) []string {
		var o []string
		for _, x := range l {
			o = append(o, x.ID)
		}
		return o
	}
	cases := []struct {
		st   Strategy
		want []string
	}{
		{StrategyFIFO, []string{b.ID, c.ID}},
		{StrategyPriority, []string{b.ID, c.ID}},
		{StrategyDependency, []string{c.ID, b.ID}},
	}
	for _, tc := range cases {
		if got := ids(bd.Ready(tc.st)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %v, want %v", tc.st, got, tc.want)
		}
	}
	bl := bd.Blocked()
	if len(bl) != 1 || bl[0].Ticket.ID != d.ID || !reflect.DeepEqual(bl[0].Waiting, []string{b.ID}) {
		t.Fatalf("blocked = %+v", bl)
	}
	if _, err := Pause(s.Root()); err != nil {
		t.Fatal(err)
	}
	bd, _ = LoadBoard(s)
	if len(bd.Ready(StrategyFIFO)) != 0 || bd.Assignable() {
		t.Fatal("pausado: Ready deve ser vazio")
	}
	if len(bd.Candidates(StrategyFIFO)) != 2 {
		t.Fatal("Candidates ignora pause")
	}
}

func TestBoardBlockedByFailedDep(t *testing.T) {
	s := newStore(t)
	a := mk(t, s, "a", nil)
	mk(t, s, "b", func(x *Ticket) { x.Dependencies = []string{a.ID} })
	_, _ = s.Transition(a.ID, StatusCanceled)
	bd, _ := LoadBoard(s)
	bl := bd.Blocked()
	if len(bl) != 1 || bl[0].Reasons[0] != "TKT-001: canceled" {
		t.Fatalf("blocked = %+v", bl)
	}
}

func TestFindCycles(t *testing.T) {
	ts := []Ticket{
		{ID: "TKT-001", Dependencies: []string{"TKT-002"}},
		{ID: "TKT-002", Dependencies: []string{"TKT-003"}},
		{ID: "TKT-003", Dependencies: []string{"TKT-001"}},
		{ID: "TKT-004", Dependencies: []string{"TKT-001"}},
	}
	cyc := FindCycles(ts)
	if len(cyc) != 1 || !reflect.DeepEqual(cyc[0], []string{"TKT-001", "TKT-002", "TKT-003", "TKT-001"}) {
		t.Fatalf("cycles = %v", cyc)
	}
	if got := FindCycles(ts[3:]); len(got) != 0 {
		t.Fatalf("sem ciclo: %v", got)
	}
}

func TestControlPersistence(t *testing.T) {
	root := t.TempDir()
	c, err := LoadControl(root)
	if err != nil || c.Paused || c.Draining {
		t.Fatalf("default: %+v %v", c, err)
	}
	if _, err := Pause(root); err != nil {
		t.Fatal(err)
	}
	if _, err := Drain(root); err != nil {
		t.Fatal(err)
	}
	c, _ = LoadControl(root)
	if !c.Paused || !c.Draining {
		t.Fatalf("%+v", c)
	}
	if _, err := os.Stat(filepath.Join(root, ".toolbelt", "board.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := Resume(root); err != nil {
		t.Fatal(err)
	}
	c, _ = LoadControl(root)
	if c.Paused || c.Draining {
		t.Fatalf("%+v", c)
	}
}

func TestParseStrategyAndStatus(t *testing.T) {
	if s, err := ParseStrategy(""); err != nil || s != StrategyDependency {
		t.Fatal(s, err)
	}
	if _, err := ParseStrategy("x"); err == nil {
		t.Fatal("want error")
	}
	if _, err := ParseStatus("nope"); err == nil {
		t.Fatal("want error")
	}
}

func TestSubscriptionLive(t *testing.T) {
	now := time.Now()
	if (*Subscription)(nil).Live(now) {
		t.Fatal("nil")
	}
	s := &Subscription{Worker: "w1", ExpiresAt: now.Add(time.Minute)}
	if !s.Live(now) || s.Live(now.Add(2*time.Minute)) {
		t.Fatal("live")
	}
}

func TestUpdateInPlaceDependencyMutationRevalidated(t *testing.T) {
	s := newStore(t)
	a := mk(t, s, "a", nil)
	b := mk(t, s, "b", func(x *Ticket) { x.Dependencies = []string{a.ID} })
	c := mk(t, s, "c", func(x *Ticket) { x.Dependencies = []string{b.ID} })
	// muta o slice no lugar: b -> c fecha ciclo b -> c -> b
	_, err := s.Update(b.ID, func(x *Ticket) error { x.Dependencies[0] = c.ID; return nil })
	if err == nil || !strings.Contains(err.Error(), "ciclo") {
		t.Fatalf("want ciclo, got %v", err)
	}
	got, _ := s.Get(b.ID)
	if !reflect.DeepEqual(got.Dependencies, []string{a.ID}) {
		t.Fatalf("estado deveria permanecer intacto: %v", got.Dependencies)
	}
}

func TestReadRejectsIDMismatch(t *testing.T) {
	s := newStore(t)
	a := mk(t, s, "a", nil)
	b := mk(t, s, "b", nil)
	data, err := os.ReadFile(s.Path(b.ID))
	if err != nil {
		t.Fatal(err)
	}
	// conteúdo de TKT-002 dentro de TKT-001.yaml
	if err := os.WriteFile(s.Path(a.ID), data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = s.Get(a.ID)
	if err == nil || !strings.Contains(err.Error(), a.ID) || !strings.Contains(err.Error(), b.ID) {
		t.Fatalf("want erro citando %s e %s, got %v", a.ID, b.ID, err)
	}
	if _, err := s.Update(a.ID, func(x *Ticket) error { x.Title = "x"; return nil }); err == nil {
		t.Fatal("Update não pode clobberar outro ticket")
	}
	got, _ := s.Get(b.ID)
	if got.Title != "b" {
		t.Fatalf("b alterado: %+v", got)
	}
}

func TestControlConcurrentPauseDrainKeepsBoth(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 150; i++ {
		if _, err := Resume(root); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = Pause(root) }()
		go func() { defer wg.Done(); _, _ = Drain(root) }()
		wg.Wait()
		c, err := LoadControl(root)
		if err != nil {
			t.Fatal(err)
		}
		if !c.Paused || !c.Draining {
			t.Fatalf("iteração %d: atualização perdida: %+v", i, c)
		}
	}
}
