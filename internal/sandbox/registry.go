package sandbox

import (
	"os/exec"
	"sync"
)

// Registry tracks the child processes THIS process started (agent leaders and
// the router's git subprocesses) and, on Linux with PR_SET_CHILD_SUBREAPER
// active (AcquireSubreaper), identifies and kills descendants that escaped the
// process group (setsid/setpgid) and were reparented to the router.
//
// Policy: agents must not detach; any untracked direct child is terminated
// (SIGKILL) and reaped (wait4). On platforms without a subreaper (macOS/Windows)
// Sweep is a no-op: only process-group behavior applies (documented
// limitation).
//
// The mutex is held only during cmd.Start+registration and during Sweep: so a
// freshly started child is never seen as an orphan and Sweep's wait4 never
// races os/exec's Wait (only untracked pids are reaped). Code that starts
// processes elsewhere (e.g. libraries calling exec directly) must route them
// through Run to be tracked; no lock is held for the child's lifetime, so Sweep
// is never blocked by a slow git.
type Registry struct {
	mu       sync.Mutex
	live     map[int]struct{}        // tracked direct children (between Start and Wait)
	groups   map[groupKey]*groupInfo // agent groups, keyed by (pid, starttime)
	found    []Orphan                // detected escapes not yet delivered
	vars     []string                // env vars to capture from each escaped process
	ownerVar string                  // variable naming a group's owner (ticket)
	windows  []*spawnWindow          // spawn windows of non-agent processes (e.g. git worktree add)
}

// groupKey identifies an agent group: the leader pid alone may be reused by
// the kernel after the group disappears; starttime disambiguates.
type groupKey struct {
	pid   int
	start uint64
}

type groupInfo struct {
	owner    string // expected ownerVar value in members ("" = do not check)
	finished bool   // the leader agent exited and the group was already killed
}

type spawnWindow struct {
	from, to uint64 // ticks since boot (same base as starttime)
	open     bool
}

// Orphan is an escaped descendant terminated by Sweep.
type Orphan struct {
	PID  int
	PGID int
	// Start is the process starttime (ticks since boot; 0 if unknown).
	Start uint64
	// Vars holds only the env vars requested in NewRegistry (ticket
	// attribution); never the whole environment (it may contain secrets).
	// nil = unreadable environment.
	Vars map[string]string
	// ForeignGroup: the process was found inside ANOTHER agent's group (setpgid
	// to a foreign pgid, to hide), with an owner different from the group's;
	// GroupOwner is the group's owner.
	ForeignGroup bool
	GroupOwner   string
	// InSpawnWindow: born inside a spawn window (BeginSpawnWindow), i.e. it may
	// be a daemon from a user git hook/fsmonitor rather than from an agent.
	InSpawnWindow bool
}

// NewRegistry: vars lists env vars to capture from each escaped process
// before killing it (ticket attribution).
func NewRegistry(vars ...string) *Registry {
	return &Registry{live: map[int]struct{}{}, groups: map[groupKey]*groupInfo{}, vars: vars}
}

// SetOwnerVar sets which variable (one of those passed to NewRegistry)
// identifies the owner of each agent group started with RunOwned. An orphan
// found in an agent group whose value differs from the group's owner is treated
// as an escape (see sweepLocked).
func (r *Registry) SetOwnerVar(name string) {
	r.mu.Lock()
	r.ownerVar = name
	r.mu.Unlock()
}

// BeginSpawnWindow opens a time window in which the caller runs third-party
// code (e.g. `git worktree add`, which may trigger user hooks/fsmonitor) whose
// detached daemons are no agent's fault. Escaped descendants born inside a
// window are flagged with Orphan.InSpawnWindow. The returned function closes
// the window.
func (r *Registry) BeginSpawnWindow() (end func()) {
	w := &spawnWindow{from: sat(nowTicks(), 2), open: true}
	r.mu.Lock()
	r.windows = append(r.windows, w)
	if len(r.windows) > 64 {
		r.windows = append([]*spawnWindow(nil), r.windows[len(r.windows)-64:]...)
	}
	r.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			t := nowTicks() + 2 // margin: starttime has 10 ms resolution
			r.mu.Lock()
			w.to, w.open = t, false
			r.mu.Unlock()
		})
	}
}

// sat subtracts d from v without underflow.
func sat(v, d uint64) uint64 {
	if v < d {
		return 0
	}
	return v - d
}

func (r *Registry) inWindowLocked(start uint64) bool {
	if start == 0 {
		return false
	}
	for _, w := range r.windows {
		if start >= w.from && (w.open || start <= w.to) {
			return true
		}
	}
	return false
}

// Run starts cmd (registering the pid under the mutex), waits and removes the
// entry. group means the child leads its own group (Setpgid).
func (r *Registry) Run(cmd *exec.Cmd, group bool) error {
	return r.RunOwned(cmd, group, "")
}

// RunOwned is Run with an owner: owner is the expected ownerVar value (ticket)
// in the group's processes; only effective with group=true.
func (r *Registry) RunOwned(cmd *exec.Cmd, group bool, owner string) error {
	if err := r.start(cmd, group, owner); err != nil {
		return err
	}
	err := cmd.Wait()
	r.exited(cmd)
	return err
}

func (r *Registry) start(cmd *exec.Cmd, group bool, owner string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := cmd.Start(); err != nil {
		return err
	}
	pid := cmd.Process.Pid
	r.live[pid] = struct{}{}
	if group {
		r.groups[groupKey{pid, readStartTime(pid)}] = &groupInfo{owner: owner}
	}
	return nil
}

func (r *Registry) exited(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	r.mu.Lock()
	delete(r.live, cmd.Process.Pid)
	r.mu.Unlock()
}

// findGroupLocked finds the agent group led by pgid (the newest, if a
// previous leader with the same pid left a stale entry).
func (r *Registry) findGroupLocked(pgid int) *groupInfo {
	var best *groupInfo
	var bestStart uint64
	for k, g := range r.groups {
		if k.pid == pgid && (best == nil || k.start >= bestStart) {
			best, bestStart = g, k.start
		}
	}
	return best
}

// finishGroup marks the agent group as finished: orphans in it are then
// reaped silently (reapProcessGroup already killed them).
func (r *Registry) finishGroup(pgid int) {
	r.mu.Lock()
	if g := r.findGroupLocked(pgid); g != nil {
		g.finished = true
	}
	r.mu.Unlock()
}

// pruneGroupsLocked removes finished groups whose process group is gone. The
// kernel does not reuse a pid while a group with that pgid exists, so "group
// gone" is the safe moment to forget the entry (no buildup, no collision from
// pid reuse).
func (r *Registry) pruneGroupsLocked() {
	for k, g := range r.groups {
		if g.finished && groupGone(k.pid) {
			delete(r.groups, k)
		}
	}
}

// Sweep kills and reaps every untracked direct child and returns the escapes
// (including those found by internal sweeps since the last call).
func (r *Registry) Sweep() []Orphan {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	out := r.found
	r.found = nil
	return out
}

// sweepLocked classifies each untracked direct child:
//
//   - outside any agent group: escape (setsid/setpgid out).
//   - in an agent group: an agent descendant whose parent died is reparented
//     to the router EVEN without escaping (e.g. `(sleep 30 &)`), so being in a
//     live agent group is, by itself, normal and left to the group. But a
//     process only reaches a "live" group of ticket T by inheritance (it
//     descends from T) or by an explicit setpgid to the foreign pgid, a trick to
//     hide from the sweep. So we compare the process's owner variable
//     (TB_TICKET_ID) with the group's owner: different, missing or unreadable
//     (with the process still alive) is treated as an escape, attributed to the
//     owner declared in the process's own environment (or unattributable when
//     missing). A process whose leader thread exited while other threads still
//     run shows Z but is live: it is classified through a live thread.
//     Zombies and exiting processes (nothing to hide, environment
//     inaccessible) are only reaped: a process killed with its escaped group
//     is reparented here while still exiting, and reading its environ then
//     fails, which must not count as an unattributable escape.
func (r *Registry) sweepLocked() {
	r.pruneGroupsLocked()
	for _, c := range listChildren() {
		if _, tracked := r.live[c.pid]; tracked {
			continue
		}
		g := r.findGroupLocked(c.pgid)
		o := Orphan{PID: c.pid, PGID: c.pgid, Start: c.start}
		// A zombie is already dead and has no environment: nothing to attribute or
		// kill, only reap (a live descendant of it shows up as its own orphan).
		escape := false
		if !c.zombie {
			escape = r.classifyLocked(&o, g, c.task)
		}
		if !escape && g != nil && !g.finished {
			continue // orphaned descendant of a still-running agent: the group kills it at the end
		}
		o.InSpawnWindow = r.inWindowLocked(c.start)
		killAndReap(c)
		if escape {
			r.found = append(r.found, o)
		}
	}
}

// classifyLocked reads the environment of a live untracked child into o and
// reports whether it is an escape (see sweepLocked). task is the /proc entry
// to read it from: o.PID, or a live thread when the leader thread exited.
func (r *Registry) classifyLocked(o *Orphan, g *groupInfo, task int) bool {
	if g == nil {
		vars, ok, task := r.taskEnv(o.PID, task)
		// An exiting task has no mm: its environ fails to open (or reads
		// empty). It is dying and cannot hide; treat it like a zombie.
		if (!ok || len(vars) == 0) && !procLive(task) {
			return false
		}
		o.Vars = vars
		return true
	}
	if g.owner == "" || r.ownerVar == "" {
		return false
	}
	vars, ok, task := r.taskEnv(o.PID, task)
	foreign := false
	switch {
	case ok:
		foreign = vars[r.ownerVar] != g.owner
	default:
		foreign = procLive(task) // unreadable but alive: the owner cannot be proven
	}
	if foreign {
		o.Vars, o.ForeignGroup, o.GroupOwner = vars, true, g.owner
	}
	return foreign
}

// taskEnv reads the captured vars through task. When task is a thread (the
// leader already exited) and it exits before the read, it retries once through
// another live thread, so a process that churns threads still gets attributed.
// It returns the task finally read from.
func (r *Registry) taskEnv(pid, task int) (vars map[string]string, ok bool, used int) {
	vars, ok = readEnvVars(task, r.vars)
	if !ok && task != pid {
		if t := liveThread(pid); t != 0 {
			task = t
			vars, ok = readEnvVars(t, r.vars)
		}
	}
	return vars, ok, task
}
