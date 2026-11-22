package bus

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// identityLockPath is <private>/run/<channel>/<name>.lock, the same file for
// every kind of holder, so `tb bus run`, `tb mcp --bus-as` and a re-join
// compete for one lock.
func identityLockPath(root, channel, name string) (string, error) {
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
	return filepath.Join(priv, "run", channel, name+".lock"), nil
}

// LockIdentity takes the exclusive lock of <channel>/<name> without waiting.
// kind is recorded with the pid for the error of the next contender. The
// lock dies with the process (flock), so a crashed holder never blocks.
func LockIdentity(root, channel, name, kind string) (release func(), err error) {
	p, err := identityLockPath(root, channel, name)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return nil, err
	}
	rel, err := lockFile(p, kind)
	if errors.Is(err, errLockHeld) {
		pid, k := readHolder(p)
		return nil, &SessionInUseError{Channel: channel, Name: name, PID: pid, Kind: k}
	}
	return rel, err
}

func holderRecord(kind string) string {
	return strconv.Itoa(os.Getpid()) + " " + kind + "\n"
}

func readHolder(path string) (int, string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, ""
	}
	pidStr, kind, _ := strings.Cut(strings.TrimSpace(string(b)), " ")
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return 0, ""
	}
	return pid, kind
}

var errLockHeld = errors.New("lock held")
