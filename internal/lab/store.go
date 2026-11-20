package lab

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/mem"
)

// LabsDir is the in-repo directory holding versioned lab manifests.
const LabsDir = "labs"

// ManifestName is the per-lab manifest file.
const ManifestName = "lab.yaml"

// EnvStateDir overrides the base of the lab private state dir (tests, harness).
const EnvStateDir = "TB_LAB_STATE_DIR"

// RepoRoot returns the repository root for cwd: the parent of the nearest
// .toolbelt directory (mem.LocateRepoToolbeltDir). When none exists it falls
// back to cwd, matching the other commands.
func RepoRoot(cwd string) string {
	return filepath.Dir(mem.LocateRepoToolbeltDir(cwd))
}

// Dir is <root>/labs/<name>, the directory holding one lab's manifest.
func Dir(root, name string) string {
	return filepath.Join(root, LabsDir, name)
}

// ManifestPath is <root>/labs/<name>/lab.yaml.
func ManifestPath(root, name string) string {
	return filepath.Join(Dir(root, name), ManifestName)
}

// Load reads and validates labs/<name>/lab.yaml under root.
func Load(root, name string) (*Lab, error) {
	data, err := os.ReadFile(ManifestPath(root, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("lab %q not found (expected %s)", name, ManifestPath(root, name))
	}
	if err != nil {
		return nil, err
	}
	l, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("lab %q: %w", name, err)
	}
	if l.Name != name {
		return nil, fmt.Errorf("lab %q: manifest name is %q", name, l.Name)
	}
	return l, nil
}

// ManifestExists reports whether labs/<name>/lab.yaml is present under root,
// regardless of whether it parses. Used to tell a true orphan (no manifest)
// from a lab whose manifest merely fails to load.
func ManifestExists(root, name string) bool {
	_, err := os.Stat(ManifestPath(root, name))
	return err == nil
}

// List returns the names of labs under root/labs, sorted.
func List(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, LabsDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, LabsDir, e.Name(), ManifestName)); err == nil {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// State is the ephemeral runtime record of a provisioned lab. It lives outside
// the repo (private state), never versioned.
type State struct {
	Name          string    `json:"name"`
	Tier          Tier      `json:"tier"`
	ProvisionedAt time.Time `json:"provisioned_at"`
	// LastVerdict is the most recent run's result, surfaced by `tb lab logs`.
	LastVerdict *Verdict `json:"last_verdict,omitempty"`
}

// stateRoot is <state>/toolbelt/lab, honoring TB_LAB_STATE_DIR and
// XDG_STATE_HOME, mirroring internal/egm and internal/bus.
func stateRoot() (string, error) {
	if base := os.Getenv(EnvStateDir); base != "" {
		if !filepath.IsAbs(base) {
			return "", fmt.Errorf("%s must be absolute", EnvStateDir)
		}
		return base, nil
	}
	if x := os.Getenv("XDG_STATE_HOME"); x != "" && filepath.IsAbs(x) {
		return filepath.Join(x, "toolbelt", "lab"), nil
	}
	if runtime.GOOS == "darwin" {
		d, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(d, "toolbelt", "state", "lab"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "toolbelt", "lab"), nil
}

// stateDir is <stateRoot>/<repo-hash>, scoping runtime state to one repo so a
// lab in one checkout never collides with another.
func stateDir(root string) (string, error) {
	base, err := stateRoot()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Join(base, hex.EncodeToString(sum[:])[:16]), nil
}

func statePath(root, name string) (string, error) {
	// Validate here so every name-joining path is covered, including down/logs
	// which take a raw CLI arg without loading the manifest. A name like
	// "../../x" would otherwise escape the per-repo state dir.
	if !nameRE.MatchString(name) {
		return "", fmt.Errorf("invalid lab name %q", name)
	}
	dir, err := stateDir(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".json"), nil
}

// WriteState records a provisioned lab's runtime state.
func WriteState(root string, s *State) error {
	path, err := statePath(root, s.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// ReadState returns a provisioned lab's state, or (nil, nil) when it is down.
func ReadState(root, name string) (*State, error) {
	path, err := statePath(root, name)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("lab state %q: %w", name, err)
	}
	return &s, nil
}

// RemoveState tears down a lab's runtime state. It is idempotent: removing an
// absent state is not an error.
func RemoveState(root, name string) error {
	path, err := statePath(root, name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Provisioned returns the names of labs with live runtime state under root.
func Provisioned(root string) ([]string, error) {
	dir, err := stateDir(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		names = append(names, e.Name()[:len(e.Name())-len(".json")])
	}
	sort.Strings(names)
	return names, nil
}
