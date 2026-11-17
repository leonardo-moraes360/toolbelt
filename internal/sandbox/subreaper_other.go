//go:build !linux

package sandbox

// Sem PR_SET_CHILD_SUBREAPER fora do Linux: descendentes que escapam do grupo
// de processos não são detectados (limitação documentada).

// AcquireSubreaper é no-op fora do Linux (ok=false).
func AcquireSubreaper() (release func(), ok bool) { return func() {}, false }

type childProc struct {
	pid, pgid int
	zombie    bool
	start     uint64
}

func listChildren() []childProc                           { return nil }
func readEnvVars(int, []string) (map[string]string, bool) { return nil, false }
func procLive(int) bool                                   { return false }
func killAndReap(childProc)                               {}
func readStartTime(int) uint64                            { return 0 }
func nowTicks() uint64                                    { return 0 }
