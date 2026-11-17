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

// AcquireSubreaper torna este processo "subreaper": descendentes órfãos (cujo
// pai morreu) são reparentados a ele em vez de ao init, o que permite ao
// Registry encontrá-los mesmo após setsid/setpgid. Contado por referência; a
// função devolvida libera. ok=false se o kernel recusar.
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
	start     uint64 // starttime (campo 22 de /proc/<pid>/stat, em ticks desde o boot)
}

// listChildren varre /proc procurando filhos diretos deste processo.
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
		data, err := os.ReadFile("/proc/" + e.Name() + "/stat") //nolint:gosec // procfs
		if err != nil {
			continue
		}
		s := string(data)
		i := strings.LastIndexByte(s, ')')
		if i < 0 {
			continue
		}
		f := strings.Fields(s[i+1:]) // state ppid pgrp session ...
		if len(f) < 3 {
			continue
		}
		ppid, err1 := strconv.Atoi(f[1])
		pgrp, err2 := strconv.Atoi(f[2])
		if err1 != nil || err2 != nil || ppid != self {
			continue
		}
		cp := childProc{pid: pid, pgid: pgrp, zombie: f[0] == "Z"}
		if len(f) > 19 {
			cp.start, _ = strconv.ParseUint(f[19], 10, 64)
		}
		out = append(out, cp)
	}
	return out
}

// readStartTime devolve o starttime (ticks desde o boot) do pid, 0 se ilegível.
func readStartTime(pid int) uint64 {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat") //nolint:gosec // procfs
	if err != nil {
		return 0
	}
	s := string(data)
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return 0
	}
	f := strings.Fields(s[i+1:])
	if len(f) <= 19 {
		return 0
	}
	v, _ := strconv.ParseUint(f[19], 10, 64)
	return v
}

// nowTicks devolve o "agora" na mesma base de starttime (ticks de 10 ms desde o
// boot, via /proc/uptime; USER_HZ é 100 no Linux). 0 se ilegível.
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

// readEnvVars lê só as variáveis pedidas de /proc/<pid>/environ. ok=false
// quando o arquivo é ilegível (processo já morto, ou não-dumpable).
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

// procLive informa se o pid existe e não é zumbi.
func procLive(pid int) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat") //nolint:gosec // procfs
	if err != nil {
		return false
	}
	s := string(data)
	i := strings.LastIndexByte(s, ')')
	if i < 0 || i+2 >= len(s) {
		return false
	}
	return s[i+2] != 'Z' && s[i+2] != 'X'
}

// killAndReap mata o órfão (e seu grupo, se ele o lidera e não é o do router) e
// colhe só esse pid com wait4. Chamado com o mutex do Registry mantido, logo o
// pid nunca é um filho rastreado pelo os/exec.
func killAndReap(c childProc) {
	if !c.zombie {
		if c.pgid == c.pid && c.pgid != syscall.Getpgrp() {
			_ = syscall.Kill(-c.pgid, syscall.SIGKILL)
		}
		_ = syscall.Kill(c.pid, syscall.SIGKILL)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		var ws syscall.WaitStatus
		wpid, err := syscall.Wait4(c.pid, &ws, syscall.WNOHANG, nil)
		if wpid == c.pid || (err != nil && err != syscall.EINTR) {
			return
		}
		if time.Now().After(deadline) {
			return // processo em estado D; a próxima varredura tenta de novo
		}
		time.Sleep(2 * time.Millisecond)
	}
}
