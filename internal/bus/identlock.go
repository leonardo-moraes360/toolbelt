package bus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ErrSessionInUse: another process already holds the identity <channel>/<name>.
var ErrSessionInUse = errors.New("session_in_use")

// SessionInUseError names the holder so the human knows what to stop.
type SessionInUseError struct {
	Channel, Name string
	PID           int    // 0 when the holder had not written its pid yet
	Kind          string // "tb mcp --bus-as", "tb bus run"
}

func (e *SessionInUseError) Error() string {
	holder := "another process"
	if e.PID > 0 {
		holder = fmt.Sprintf("pid %d", e.PID)
		if e.Kind != "" {
			holder += " (" + e.Kind + ")"
		}
	}
	return fmt.Sprintf("session_in_use: %s/%s is held by %s; stop it first, or use another agent name (a bus identity has one live client)",
		e.Channel, e.Name, holder)
}

func (e *SessionInUseError) Is(target error) bool { return target == ErrSessionInUse }

// Two locks guard an identity <channel>/<name>, both under <private>/run/<channel>:
//
//	<name>.harness.lock  one `tb bus run` per agent
//	<name>.lock          one MCP/CLI client per agent: whatever serves the
//	                     identity (the GUI client in notify mode, the wake
//	                     child in harness mode, a mutating CLI one-shot)
//
// flock is host-local: on an NFS mount mounted with nolock it only covers
// processes of one machine.
const probeKind = "probe"

const (
	clientSuffix  = ".lock"
	harnessSuffix = ".harness.lock"
)

func lockPath(root, channel, name, suffix string) (string, error) {
	if err := ValidName("channel", channel); err != nil {
		return "", err
	}
	if err := ValidName("agent name", name); err != nil {
		return "", err
	}
	priv, err := PrivateDir(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(priv, "run", channel, name+suffix), nil
}

// ClientLockPath is the file a sandboxed wake child is granted so it can
// take the client lock itself (flock needs only an open descriptor).
// EnsureClientLockFile creates it first: a path rule needs an existing file.
func ClientLockPath(root, channel, name string) (string, error) {
	return lockPath(root, channel, name, clientSuffix)
}

func EnsureClientLockFile(root, channel, name string) (string, error) {
	p, err := ClientLockPath(root, channel, name)
	if err != nil {
		return "", err
	}
	if fileExistenceIsLock {
		return p, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return "", err
	}
	return p, f.Close()
}

func lockNamed(root, channel, name, suffix, kind string) (release func(), err error) {
	p, err := lockPath(root, channel, name, suffix)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return nil, err
	}
	var rel func()
	for try := 0; ; try++ {
		rel, err = lockFile(p, kind)
		if !errors.Is(err, errLockHeld) {
			return rel, err
		}
		// A probe holds the lock for microseconds and leaves the previous
		// holder's record in place, so retry over ~50 ms before blaming the
		// pid in the file.
		if try < 5 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		pid, k := readHolder(p)
		return nil, &SessionInUseError{Channel: channel, Name: name, PID: pid, Kind: k}
	}
}

// LockClient takes the client lock without waiting. kind is recorded with
// the pid for the next contender's error. The lock dies with the process.
func LockClient(root, channel, name, kind string) (release func(), err error) {
	return lockNamed(root, channel, name, clientSuffix, kind)
}

// LockHarness takes the one-harness-per-agent lock.
func LockHarness(root, channel, name string) (release func(), err error) {
	return lockNamed(root, channel, name, harnessSuffix, "tb bus run")
}

// ProbeClient reports session_in_use when a client holds the identity,
// without keeping the lock.
func ProbeClient(root, channel, name string) error {
	rel, err := LockClient(root, channel, name, probeKind)
	if err == nil {
		rel()
	}
	return err
}

// ProbeHarness is ProbeClient for the harness lock.
func ProbeHarness(root, channel, name string) error {
	rel, err := lockNamed(root, channel, name, harnessSuffix, probeKind)
	if err == nil {
		rel()
	}
	return err
}

func holderRecord(kind string) string {
	return strconv.Itoa(os.Getpid()) + " " + kind + "\n"
}

func readHolder(path string) (pid int, kind string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, ""
	}
	pidStr, kind, _ := strings.Cut(strings.TrimSpace(string(b)), " ")
	pid, err = strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return 0, ""
	}
	return pid, kind
}

var errLockHeld = errors.New("lock held")
