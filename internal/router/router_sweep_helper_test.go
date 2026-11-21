//go:build !windows

package router

import (
	"testing"
	"time"
)

// TestSweepSparesTestGit is the regression test for CI flakes where git run by
// test helpers died with "signal: killed": procs.Sweep kills every untracked
// direct child of the process, so a helper git started with a bare exec.Command
// while a router in the same test process swept (sweepTick, or an agent's
// settleGroup) was SIGKILLed as an escape. runGit must register its child in
// procs, like the router's own git (runTracked).
func TestSweepSparesTestGit(t *testing.T) {
	dir := newRepo(t)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			for _, o := range procs.Sweep() {
				t.Errorf("sweep killed %+v", o)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	// git runs the alias through a shell; git itself stays alive (and a direct
	// child of this process) for the whole sleep.
	runGit(t, dir, "-c", "alias.slow=!sleep 0.3", "slow")
	close(stop)
	<-done
}
