package ticket

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type Strategy string

const (
	StrategyFIFO       Strategy = "fifo"
	StrategyPriority   Strategy = "priority"
	StrategyDependency Strategy = "dependency"
)

// ParseStrategy treats "" as the dependency strategy.
func ParseStrategy(s string) (Strategy, error) {
	switch Strategy(s) {
	case "":
		return StrategyDependency, nil
	case StrategyFIFO, StrategyPriority, StrategyDependency:
		return Strategy(s), nil
	}
	return "", fmt.Errorf("estratégia inválida %q (use fifo, priority ou dependency)", s)
}

// Control is persisted in .toolbelt/board.yaml.
type Control struct {
	Paused   bool `yaml:"paused" json:"paused"`
	Draining bool `yaml:"draining" json:"draining"`
}

func ControlPath(root string) string { return filepath.Join(root, ".toolbelt", "board.yaml") }

// LoadControl returns the zero value when the file is missing.
func LoadControl(root string) (Control, error) {
	data, err := os.ReadFile(ControlPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return Control{}, nil
	}
	if err != nil {
		return Control{}, err
	}
	var c Control
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Control{}, fmt.Errorf("board.yaml ilegível: %w", err)
	}
	return c, nil
}

// SaveControl writes atomically.
func SaveControl(root string, c Control) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return WriteFileAtomic(ControlPath(root), data)
}

// controlLock serializes read-modify-write of board.yaml across goroutines
// and processes.
func controlLock(root string) (func(), error) {
	dir := filepath.Dir(ControlPath(root))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return acquireLock(filepath.Join(dir, "board.lock"))
}

func Pause(root string) (Control, error) {
	unlock, err := controlLock(root)
	if err != nil {
		return Control{}, err
	}
	defer unlock()
	c, err := LoadControl(root)
	if err != nil {
		return c, err
	}
	c.Paused = true
	return c, SaveControl(root, c)
}

// Resume clears both pause and drain.
func Resume(root string) (Control, error) {
	unlock, err := controlLock(root)
	if err != nil {
		return Control{}, err
	}
	defer unlock()
	c := Control{}
	return c, SaveControl(root, c)
}

// Drain lets running tickets finish but assigns no new ones.
func Drain(root string) (Control, error) {
	unlock, err := controlLock(root)
	if err != nil {
		return Control{}, err
	}
	defer unlock()
	c, err := LoadControl(root)
	if err != nil {
		return c, err
	}
	c.Draining = true
	return c, SaveControl(root, c)
}

type Board struct {
	Tickets []Ticket
	Control Control
}

func LoadBoard(s *Store) (*Board, error) {
	list, err := s.List(Filter{})
	if err != nil {
		return nil, err
	}
	c, err := LoadControl(s.Root())
	if err != nil {
		return nil, err
	}
	return &Board{Tickets: list, Control: c}, nil
}

type Blocked struct {
	Ticket  Ticket   `json:"ticket"`
	Waiting []string `json:"waiting_on"`
	Reasons []string `json:"reasons"`
}

func (b *Board) byID() map[string]Ticket {
	m := make(map[string]Ticket, len(b.Tickets))
	for i := range b.Tickets {
		m[b.Tickets[i].ID] = b.Tickets[i]
	}
	return m
}

func (b *Board) unmet(t *Ticket, byID map[string]Ticket) (waiting, reasons []string) {
	for _, d := range t.Dependencies {
		dep, ok := byID[d]
		switch {
		case !ok:
			waiting = append(waiting, d)
			reasons = append(reasons, d+": inexistente")
		case dep.Status != StatusDone:
			waiting = append(waiting, d)
			reasons = append(reasons, fmt.Sprintf("%s: %s", d, dep.Status))
		}
	}
	return
}

// Candidates ignores pause/drain (useful for display).
func (b *Board) Candidates(strategy Strategy) []Ticket {
	byID := b.byID()
	var out []Ticket
	for i := range b.Tickets {
		t := &b.Tickets[i]
		if t.Status != StatusPending {
			continue
		}
		if w, _ := b.unmet(t, byID); len(w) == 0 {
			out = append(out, *t)
		}
	}
	SortTickets(out, strategy, b.Tickets)
	return out
}

func (b *Board) Assignable() bool { return !b.Control.Paused && !b.Control.Draining }

// Ready is empty while the board is paused.
func (b *Board) Ready(strategy Strategy) []Ticket {
	if b.Control.Paused {
		return nil
	}
	return b.Candidates(strategy)
}

func (b *Board) Blocked() []Blocked {
	byID := b.byID()
	var out []Blocked
	for i := range b.Tickets {
		t := &b.Tickets[i]
		if t.Status != StatusPending {
			continue
		}
		if w, r := b.unmet(t, byID); len(w) > 0 {
			out = append(out, Blocked{Ticket: *t, Waiting: w, Reasons: r})
		}
	}
	return out
}

// WithStatus returns tickets sorted by ID.
func (b *Board) WithStatus(st Status) []Ticket {
	var out []Ticket
	for i := range b.Tickets {
		if b.Tickets[i].Status == st {
			out = append(out, b.Tickets[i])
		}
	}
	return out
}

func (b *Board) Cycles() [][]string { return FindCycles(b.Tickets) }

// SortTickets sorts in place. all provides the full graph used to compute
// depth (dependency strategy).
func SortTickets(list []Ticket, strategy Strategy, all []Ticket) {
	fifo := func(a, b Ticket) bool {
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return idNumber(a.ID) < idNumber(b.ID)
	}
	switch strategy {
	case StrategyPriority:
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Priority != list[j].Priority {
				return list[i].Priority > list[j].Priority
			}
			return fifo(list[i], list[j])
		})
	case StrategyDependency:
		depth := Depths(all)
		sort.SliceStable(list, func(i, j int) bool {
			di, dj := depth[list[i].ID], depth[list[j].ID]
			if di != dj {
				return di < dj
			}
			if list[i].Priority != list[j].Priority {
				return list[i].Priority > list[j].Priority
			}
			return fifo(list[i], list[j])
		})
	default:
		sort.SliceStable(list, func(i, j int) bool { return fifo(list[i], list[j]) })
	}
}

// Depths computes topological depth (0 = no dependencies). Cycles and
// missing dependencies are tolerated.
func Depths(all []Ticket) map[string]int {
	byID := make(map[string]Ticket, len(all))
	for i := range all {
		byID[all[i].ID] = all[i]
	}
	depth := map[string]int{}
	visiting := map[string]bool{}
	var walk func(id string) int
	walk = func(id string) int {
		if d, ok := depth[id]; ok {
			return d
		}
		if visiting[id] {
			return 0
		}
		visiting[id] = true
		d := 0
		for _, dep := range byID[id].Dependencies {
			if _, ok := byID[dep]; !ok {
				continue
			}
			if x := walk(dep) + 1; x > d {
				d = x
			}
		}
		visiting[id] = false
		depth[id] = d
		return d
	}
	for id := range byID {
		walk(id)
	}
	return depth
}

// FindCycles returns each cycle as a list of IDs starting and ending at the
// same node (e.g. A, B, A).
func FindCycles(all []Ticket) [][]string {
	byID := make(map[string]Ticket, len(all))
	ids := make([]string, 0, len(all))
	for i := range all {
		byID[all[i].ID] = all[i]
		ids = append(ids, all[i].ID)
	}
	sort.Slice(ids, func(i, j int) bool { return idNumber(ids[i]) < idNumber(ids[j]) })
	const (
		white = iota
		gray
		black
	)
	color := map[string]int{}
	var stack []string
	var out [][]string
	var visit func(id string)
	visit = func(id string) {
		color[id] = gray
		stack = append(stack, id)
		for _, d := range byID[id].Dependencies {
			if _, ok := byID[d]; !ok {
				continue
			}
			switch color[d] {
			case white:
				visit(d)
			case gray:
				i := len(stack) - 1
				for i >= 0 && stack[i] != d {
					i--
				}
				cyc := append([]string{}, stack[i:]...)
				cyc = append(cyc, d)
				out = append(out, cyc)
			}
		}
		stack = stack[:len(stack)-1]
		color[id] = black
	}
	for _, id := range ids {
		if color[id] == white {
			visit(id)
		}
	}
	return out
}

// View backs `tb board list` and its --json output.
type View struct {
	Paused   bool       `json:"paused"`
	Draining bool       `json:"draining"`
	Strategy Strategy   `json:"strategy"`
	Running  []Ticket   `json:"running"`
	Ready    []Ticket   `json:"ready"`
	Blocked  []Blocked  `json:"blocked"`
	Done     []Ticket   `json:"done"`
	Failed   []Ticket   `json:"failed"`
	Canceled []Ticket   `json:"canceled"`
	Cycles   [][]string `json:"cycles"`
}

// Ready uses Candidates (ignoring pause, which is reported in Paused) so the
// queue stays visible while the board is paused. A non-empty worker keeps only
// that worker's tickets (active/final sections; Ready and Blocked have no
// worker and end up empty).
func (b *Board) View(strategy Strategy, statuses []Status, worker string) View {
	keep := func(list []Ticket) []Ticket {
		out := []Ticket{}
		for i := range list {
			if worker != "" && list[i].Worker != worker {
				continue
			}
			out = append(out, list[i])
		}
		return out
	}
	want := func(st Status) bool { return len(statuses) == 0 || hasStatus(statuses, st) }
	v := View{
		Paused: b.Control.Paused, Draining: b.Control.Draining, Strategy: strategy,
		Running: []Ticket{}, Ready: []Ticket{}, Blocked: []Blocked{},
		Done: []Ticket{}, Failed: []Ticket{}, Canceled: []Ticket{}, Cycles: b.Cycles(),
	}
	if v.Cycles == nil {
		v.Cycles = [][]string{}
	}
	if want(StatusRunning) {
		v.Running = keep(b.WithStatus(StatusRunning))
	}
	if want(StatusPending) && worker == "" {
		v.Ready = b.Candidates(strategy)
		v.Blocked = b.Blocked()
	}
	if want(StatusDone) {
		v.Done = keep(b.WithStatus(StatusDone))
	}
	if want(StatusFailed) {
		v.Failed = keep(b.WithStatus(StatusFailed))
	}
	if want(StatusCanceled) {
		v.Canceled = keep(b.WithStatus(StatusCanceled))
	}
	return v
}

// FindRoot walks up from start to the first directory containing .toolbelt,
// falling back to start.
func FindRoot(start string) string {
	if start == "" {
		start, _ = os.Getwd()
	}
	dir := start
	for {
		if st, err := os.Stat(filepath.Join(dir, ".toolbelt")); err == nil && st.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}
