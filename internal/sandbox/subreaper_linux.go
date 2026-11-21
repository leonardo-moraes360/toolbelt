//go:build linux

package sandbox

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const prSetChildSubreaper = 36 // PR_SET_CHILD_SUBREAPER

var (
	subreaperMu    sync.Mutex
	subreaperCount int
)

// AcquireSubreaper makes this process a "subreaper": orphaned descendants
// (whose parent died) are reparented to it instead of init, which lets the
// Registry find them even after setsid/setpgid. Reference-counted; the returned
// function releases it. ok=false if the kernel refuses.
func AcquireSubreaper() (release func(), ok bool) {
	subreaperMu.Lock()
	defer subreaperMu.Unlock()
	if subreaperCount == 0 {
		if _, _, e := syscall.RawSyscall(syscall.SYS_PRCTL, prSetChildSubreaper, 1, 0); e != 0 {
			return func() {}, false
		}
	}
	subreaperCount++
	var once sync.Once
	return func() {
		once.Do(func() {
			subreaperMu.Lock()
			defer subreaperMu.Unlock()
			subreaperCount--
			if subreaperCount == 0 {
				_, _, _ = syscall.RawSyscall(syscall.SYS_PRCTL, prSetChildSubreaper, 0, 0)
			}
		})
	}, true
}

type childProc struct {
	pid, pgid int
	zombie    bool
	start     uint64 // starttime (field 22 of /proc/<pid>/stat, in ticks since boot)
	// task is the /proc entry to read the environment and liveness from: pid
	// itself, or a live thread when the leader thread already exited (see
	// listChildren).
	task int
}

// listChildren scans /proc for direct children of this process.
func listChildren() []childProc {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	self := os.Getpid()
	var out []childProc
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		f := readStat(pid) // state ppid pgrp session ...
		if len(f) < 3 {
			continue
		}
		ppid, err1 := strconv.Atoi(f[1])
		pgrp, err2 := strconv.Atoi(f[2])
		if err1 != nil || err2 != nil || ppid != self {
			continue
		}
		cp := childProc{pid: pid, pgid: pgrp, task: pid}
		if f[0] == "Z" {
			// The leader of a multithreaded process that called exit(2) (not
			// exit_group) shows Z while its other threads keep running: it is
			// live, wait4 cannot reap it, and its own environ is gone (no mm).
			// Read through a live thread instead; only a group with no live
			// thread is a real zombie.
			if t := liveThread(pid); t != 0 {
				cp.task = t
			} else {
				cp.zombie = true
			}
		}
		if len(f) > 19 {
			cp.start, _ = strconv.ParseUint(f[19], 10, 64)
		}
		out = append(out, cp)
	}
	return out
}

// liveThread returns a live thread of pid other than the leader, 0 if none.
func liveThread(pid int) int {
	dir := "/proc/" + strconv.Itoa(pid) + "/task"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		tid, err := strconv.Atoi(e.Name())
		if err != nil || tid == pid {
			continue
		}
		data, err := os.ReadFile(dir + "/" + e.Name() + "/stat") //nolint:gosec // procfs
		if err == nil && statLive(string(data)) {
			return tid
		}
	}
	return 0
}

// readStat returns the fields of /proc/<pid>/stat after the comm (state ppid
// pgrp ...), nil if unreadable.
func readStat(pid int) []string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat") //nolint:gosec // procfs
	if err != nil {
		return nil
	}
	s := string(data)
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return nil
	}
	return strings.Fields(s[i+1:])
}

// readStartTime returns pid's starttime (ticks since boot), 0 if unreadable.
func readStartTime(pid int) uint64 {
	f := readStat(pid)
	if len(f) <= 19 {
		return 0
	}
	v, _ := strconv.ParseUint(f[19], 10, 64)
	return v
}

// nowTicks returns "now" in the same base as starttime (10 ms ticks since
// boot, via /proc/uptime; USER_HZ is 100 on Linux). 0 if unreadable.
func nowTicks() uint64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	f := strings.Fields(string(data))
	if len(f) == 0 {
		return 0
	}
	sec, err := strconv.ParseFloat(f[0], 64)
	if err != nil || sec < 0 {
		return 0
	}
	return uint64(sec * 100)
}

// readEnvVars reads only the requested variables from /proc/<pid>/environ.
// ok=false when the file is unreadable (process already dead, or non-dumpable).
func readEnvVars(pid int, keys []string) (vars map[string]string, ok bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ") //nolint:gosec // procfs
	if err != nil {
		return nil, false
	}
	out := map[string]string{}
	for _, kv := range strings.Split(string(data), "\x00") {
		k, v, found := strings.Cut(kv, "=")
		if !found {
			continue
		}
		for _, want := range keys {
			if k == want {
				out[k] = v
			}
		}
	}
	return out, true
}

// pfExiting is PF_EXITING (include/linux/sched.h): set by the kernel as soon
// as a task starts exiting, before it drops its mm and well before it turns
// into a zombie.
const pfExiting = 0x4

// procLive reports whether pid exists and is neither a zombie nor exiting.
func procLive(pid int) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat") //nolint:gosec // procfs
	if err != nil {
		return false
	}
	return statLive(string(data))
}

// statLive parses a /proc/<pid>/stat line. A task that is exiting (e.g.
// SIGKILLed with its group) still shows state R/D for a moment after its mm
// is gone, so its environ is already unreadable: the PF_EXITING flag tells it
// apart from a live process that hides its environment.
func statLive(s string) bool {
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return false
	}
	f := strings.Fields(s[i+1:]) // state ppid pgrp session tty_nr tpgid flags ...
	if len(f) < 7 || f[0] == "Z" || f[0] == "X" {
		return false
	}
	flags, err := strconv.ParseUint(f[6], 10, 64)
	return err == nil && flags&pfExiting == 0
}

// killAndReap kills the orphan (and its group, if it leads one that is not the
// router's) and reaps only that pid with wait4. Called with the Registry mutex
// held, so the pid is never a child tracked by os/exec.
func killAndReap(c childProc) {
	if !c.zombie && c.pgid == c.pid && c.pgid != syscall.Getpgrp() {
		_ = syscall.Kill(-c.pgid, syscall.SIGKILL)
	}
	// Always signal the pid itself: a no-op for a real zombie, and it kills the
	// whole thread group of a process whose leader thread exited but whose
	// other threads still run (wait4 could never reap it otherwise).
	_ = syscall.Kill(c.pid, syscall.SIGKILL)
	deadline := time.Now().Add(2 * time.Second)
	for {
		var ws syscall.WaitStatus
		wpid, err := syscall.Wait4(c.pid, &ws, syscall.WNOHANG, nil)
		if wpid == c.pid || (err != nil && err != syscall.EINTR) {
			return
		}
		if time.Now().After(deadline) {
			return // process in D state; the next sweep retries
		}
		time.Sleep(2 * time.Millisecond)
	}
}
