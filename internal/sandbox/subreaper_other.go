//go:build !linux

package sandbox

// No PR_SET_CHILD_SUBREAPER outside Linux: descendants that escape the process
// group are not detected (documented limitation).

// AcquireSubreaper is a no-op outside Linux (ok=false).
func AcquireSubreaper() (release func(), ok bool) { return func() {}, false }

type childProc struct {
	pid, pgid int
	zombie    bool
	start     uint64
	task      int
}

func listChildren() []childProc                           { return nil }
func readEnvVars(int, []string) (map[string]string, bool) { return nil, false }
func procLive(int) bool                                   { return false }
func liveThread(int) int                                  { return 0 }
func killAndReap(childProc)                               {}
func readStartTime(int) uint64                            { return 0 }
func nowTicks() uint64                                    { return 0 }
