//go:build linux

package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const (
	oPath                        = 0x200000
	landlockRulePathBeneath      = 1
	landlockCreateRulesetVersion = 1
	sysLandlockCreateRuleset     = 444
	sysLandlockAddRule           = 445
	sysLandlockRestrictSelf      = 446
	prSetNoNewPrivs              = 38
	landlockAccessFSExecute      = 1 << 0
	landlockAccessFSWriteFile    = 1 << 1
	landlockAccessFSReadFile     = 1 << 2
	landlockAccessFSReadDir      = 1 << 3
	landlockAccessFSRemoveDir    = 1 << 4
	landlockAccessFSRemoveFile   = 1 << 5
	landlockAccessFSMakeChar     = 1 << 6
	landlockAccessFSMakeDir      = 1 << 7
	landlockAccessFSMakeReg      = 1 << 8
	landlockAccessFSMakeSock     = 1 << 9
	landlockAccessFSMakeFifo     = 1 << 10
	landlockAccessFSMakeBlock    = 1 << 11
	landlockAccessFSMakeSym      = 1 << 12
	landlockAccessFSRefer        = 1 << 13
	landlockAccessFSTruncate     = 1 << 14
	landlockAccessFSIoctlDev     = 1 << 15
	envLandlockChild             = "TB_SANDBOX_LANDLOCK_CHILD"
	envLandlockPayload           = "TB_SANDBOX_LANDLOCK_PAYLOAD"
)

type rulesetAttr struct {
	handledAccessFs uint64
}

type pathBeneathAttr struct {
	allowedAccess uint64
	parentFd      int32
}

type childPayload struct {
	WorkDir   string       `json:"work_dir"`
	Command   string       `json:"command"`
	Args      []string     `json:"args"`
	Allowed   []PathAccess `json:"allowed"`
	Forbidden []string     `json:"forbidden"`
}

func init() {
	if os.Getenv(envLandlockChild) != "1" {
		return
	}
	if err := runLandlockChild(); err != nil {
		fmt.Fprintf(os.Stderr, "tb sandbox: falha ao aplicar Landlock: %v\n", err)
		os.Exit(127)
	}
}

func CheckLandlockSupport() (supported bool, version int, err error) {
	abi, _, errno := syscall.Syscall(sysLandlockCreateRuleset, 0, 0, landlockCreateRulesetVersion)
	if errno == syscall.ENOSYS || errno == syscall.EOPNOTSUPP {
		return false, 0, fmt.Errorf("kernel sem Landlock: %v", errno)
	}
	if errno != 0 {
		return false, 0, errno
	}
	return true, int(abi), nil
}

func handledAccessFS(abi int) uint64 {
	mask := uint64(landlockAccessFSExecute |
		landlockAccessFSWriteFile |
		landlockAccessFSReadFile |
		landlockAccessFSReadDir |
		landlockAccessFSRemoveDir |
		landlockAccessFSRemoveFile |
		landlockAccessFSMakeChar |
		landlockAccessFSMakeDir |
		landlockAccessFSMakeReg |
		landlockAccessFSMakeSock |
		landlockAccessFSMakeFifo |
		landlockAccessFSMakeBlock |
		landlockAccessFSMakeSym)
	if abi >= 2 {
		mask |= landlockAccessFSRefer
	}
	if abi >= 3 {
		mask |= landlockAccessFSTruncate
	}
	if abi >= 5 { // LANDLOCK_ACCESS_FS_IOCTL_DEV arrived in ABI 5 (ABI 4 is networking)
		mask |= landlockAccessFSIoctlDev
	}
	return mask
}

func applyLandlock(workDir string, allowed []PathAccess, forbidden []string) error {
	supported, abi, err := CheckLandlockSupport()
	if !supported {
		if err == nil {
			err = ErrUnsupported
		}
		return fmt.Errorf("Landlock indisponível: %w", err)
	}

	ruleset, err := createRuleset(abi)
	if err != nil {
		return err
	}
	defer syscall.Close(ruleset)

	if err := addSystemReadRules(ruleset, abi); err != nil {
		return err
	}
	if err := addRule(ruleset, workDir, false, abi); err != nil {
		return fmt.Errorf("regra do workdir %q: %w", workDir, err)
	}
	// No rule for os.TempDir(): the shared /tmp gets no access (writes only in
	// the workdir and AllowedPaths). Anything that needs temp space should point
	// TMPDIR inside the workdir.
	for _, a := range allowed {
		if err := addRule(ruleset, a.Path, a.ReadOnly, abi); err != nil {
			return fmt.Errorf("regra allow %q: %w", a.Path, err)
		}
	}
	_ = forbidden

	return restrictSelf(ruleset)
}

func createRuleset(abi int) (int, error) {
	attr := rulesetAttr{handledAccessFs: handledAccessFS(abi)}
	fd, _, errno := syscall.Syscall(sysLandlockCreateRuleset, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		return -1, fmt.Errorf("landlock_create_ruleset: %v", errno)
	}
	return int(fd), nil
}

func addSystemReadRules(ruleset, abi int) error {
	roPaths := []string{
		"/bin", "/sbin", "/usr/bin", "/usr/sbin",
		"/lib", "/lib64", "/usr/lib", "/usr/lib64",
		"/etc", "/dev", "/proc", "/sys",
		"/usr/libexec", "/usr/share",
	}
	for _, p := range roPaths {
		_ = addRuleOptional(ruleset, p, true, abi)
	}
	// /etc/resolv.conf is often a symlink outside /etc (e.g.
	// /run/systemd/resolve/stub-resolv.conf); without reading the target, the child
	// cannot resolve DNS. Allow only the target's directory, read-only.
	if target, err := filepath.EvalSymlinks("/etc/resolv.conf"); err == nil && !strings.HasPrefix(target, "/etc/") {
		_ = addRuleOptional(ruleset, filepath.Dir(target), true, abi)
	}
	addDeviceRules(ruleset, abi)
	return nil
}

// deviceRule is a /dev node allowed for writing. /dev as a whole stays
// read-only; only these nodes get write access (git, shells and agents need
// /dev/null etc.). Optional: missing nodes are skipped.
//
// Terminals (/dev/tty, /dev/ptmx, /dev/pts/*) are left out on purpose: write
// access to /dev/pts would allow injecting escape sequences into the user's
// other terminals, and ioctl on a tty enables TIOCSTI injection where
// legacy_tiocsti is enabled.
type deviceRule struct {
	path     string
	readOnly bool // random/urandom: read access is enough
}

var deviceRules = []deviceRule{
	{path: "/dev/null"},
	{path: "/dev/zero"},
	{path: "/dev/full"},
	{path: "/dev/random", readOnly: true},
	{path: "/dev/urandom", readOnly: true},
}

func addDeviceRules(ruleset, abi int) {
	for _, d := range deviceRules {
		if _, err := os.Stat(d.path); err != nil {
			continue
		}
		_ = addRuleMask(ruleset, d.path, deviceFileMask(d.readOnly, abi))
	}
}

// deviceFileMask returns only rights valid for a file (non-directory): the
// kernel returns EINVAL if a file rule includes directory rights. TRUNCATE
// requires ABI>=3. Never includes IOCTL_DEV (no ttys).
func deviceFileMask(readOnly bool, abi int) uint64 {
	mask := uint64(landlockAccessFSReadFile)
	if !readOnly {
		mask |= landlockAccessFSWriteFile
		if abi >= 3 {
			mask |= landlockAccessFSTruncate
		}
	}
	return mask
}

func restrictSelf(ruleset int) error {
	if _, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, prSetNoNewPrivs, 1, 0, 0, 0, 0); errno != 0 {
		return fmt.Errorf("prctl(PR_SET_NO_NEW_PRIVS): %v", errno)
	}
	if _, _, errno := syscall.Syscall(sysLandlockRestrictSelf, uintptr(ruleset), 0, 0); errno != 0 {
		return fmt.Errorf("landlock_restrict_self: %v", errno)
	}
	return nil
}

func addRuleOptional(ruleset int, path string, readOnly bool, abi int) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	_ = addRule(ruleset, path, readOnly, abi)
	return nil
}

func addRule(ruleset int, path string, readOnly bool, abi int) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	parent, err := syscall.Open(abs, oPath|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", abs, err)
	}
	defer syscall.Close(parent)

	return addRuleMask2(ruleset, abs, parent, accessMask(readOnly, abi))
}

// addRuleMask adds a path-beneath rule with exactly the given mask.
func addRuleMask(ruleset int, path string, mask uint64) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	parent, err := syscall.Open(abs, oPath|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", abs, err)
	}
	defer syscall.Close(parent)
	return addRuleMask2(ruleset, abs, parent, mask)
}

func addRuleMask2(ruleset int, abs string, parent int, mask uint64) error {
	pba := pathBeneathAttr{allowedAccess: mask, parentFd: int32(parent)} //nolint:gosec // kernel file descriptor
	_, _, errno := syscall.Syscall6(sysLandlockAddRule, uintptr(ruleset), landlockRulePathBeneath, uintptr(unsafe.Pointer(&pba)), 0, 0, 0)
	if errno != 0 {
		return fmt.Errorf("landlock_add_rule %s: %v", abs, errno)
	}
	return nil
}

func accessMask(readOnly bool, abi int) uint64 {
	mask := uint64(landlockAccessFSReadFile | landlockAccessFSReadDir | landlockAccessFSExecute)
	if abi >= 5 {
		mask |= landlockAccessFSIoctlDev
	}
	if readOnly {
		return mask
	}
	mask |= landlockAccessFSWriteFile |
		landlockAccessFSRemoveDir |
		landlockAccessFSRemoveFile |
		landlockAccessFSMakeChar |
		landlockAccessFSMakeDir |
		landlockAccessFSMakeReg |
		landlockAccessFSMakeSock |
		landlockAccessFSMakeFifo |
		landlockAccessFSMakeBlock |
		landlockAccessFSMakeSym
	if abi >= 2 {
		mask |= landlockAccessFSRefer
	}
	if abi >= 3 {
		mask |= landlockAccessFSTruncate
	}
	return mask
}

func prepareChildEnv(cfg *Config, command string, args []string) ([]string, error) {
	payload, err := json.Marshal(childPayload{
		WorkDir:   cfg.WorkDir,
		Command:   command,
		Args:      args,
		Allowed:   cfg.AllowedPaths,
		Forbidden: cfg.ForbiddenPaths,
	})
	if err != nil {
		return nil, err
	}
	base := os.Environ()
	if cfg.Env != nil {
		base = append([]string{}, cfg.Env...)
	}
	base = append(base,
		envLandlockChild+"=1",
		envLandlockPayload+"="+string(payload),
	)
	return base, nil
}

func runLandlockChild() error {
	raw := os.Getenv(envLandlockPayload)
	if raw == "" {
		return fmt.Errorf("payload Landlock ausente")
	}
	var p childPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return fmt.Errorf("payload Landlock inválido: %w", err)
	}
	if p.WorkDir == "" || p.Command == "" {
		return fmt.Errorf("workdir/comando obrigatórios no payload")
	}

	runtime.LockOSThread()
	if err := applyLandlock(p.WorkDir, p.Allowed, p.Forbidden); err != nil {
		return err
	}

	if err := os.Chdir(p.WorkDir); err != nil {
		return fmt.Errorf("chdir: %w", err)
	}

	argv := append([]string{p.Command}, p.Args...)
	return syscall.Exec(p.Command, argv, childCleanEnv()) //nolint:gosec // command resolved via LookPath in the parent
}

func childCleanEnv() []string {
	out := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, envLandlockChild+"=") || strings.HasPrefix(e, envLandlockPayload+"=") {
			continue
		}
		out = append(out, e)
	}
	return out
}
