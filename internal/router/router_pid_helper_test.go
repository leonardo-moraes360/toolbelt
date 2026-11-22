//go:build !windows

package router

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// pidGone waits until pid no longer exists. A killed process that the test
// process (the router's subreaper) has not reaped yet is still a zombie that
// answers kill(pid, 0), so the check polls with a deadline instead of
// sampling once.
func pidGone(t *testing.T, pid int, what string) {
	t.Helper()
	if pid <= 0 {
		t.Fatalf("%s: no pid was recorded", what)
	}
	waitFor(t, fmt.Sprintf("%s (pid %d) to be killed", what, pid), func() bool {
		return syscall.Kill(pid, 0) != nil
	})
}

// readPid waits for the pidfile an escaped process writes asynchronously and
// returns its pid.
func readPid(t *testing.T, path string) int {
	t.Helper()
	pid := 0
	waitFor(t, "pidfile "+filepath.Base(path), func() bool {
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		return pid > 0
	})
	return pid
}
