//go:build linux

package sandbox

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	oPath                      = 0x200000
	landlockRulePathBeneath    = 1
	sysLandlockCreateRuleset   = 444
	sysLandlockAddRule         = 445
	sysLandlockRestrictSelf    = 446
	landlockAccessFSExecute    = 1 << 0
	landlockAccessFSWriteFile  = 1 << 1
	landlockAccessFSReadFile   = 1 << 2
	landlockAccessFSReadDir    = 1 << 3
	landlockAccessFSRemoveDir  = 1 << 4
	landlockAccessFSRemoveFile = 1 << 5
	landlockAccessFSMakeDir    = 1 << 7
	landlockAccessFSMakeReg    = 1 << 8
)

type rulesetAttr struct {
	handledAccessFs uint64
}

type pathBeneathAttr struct {
	allowedAccess uint64
	parentFd      int32
	_             [4]byte
}

func CheckLandlockSupport() (supported bool, version int, err error) {
	_, _, errno := syscall.Syscall(sysLandlockCreateRuleset, 0, 0, 0)
	if errno == syscall.ENOSYS || errno == syscall.EOPNOTSUPP {
		return false, 0, fmt.Errorf("kernel sem Landlock: %v", errno)
	}
	if errno != 0 {
		return false, 0, errno
	}
	return true, 1, nil
}

func applyLandlock(workDir string, allowed []PathAccess, forbidden []string) error {
	supported, _, err := CheckLandlockSupport()
	if !supported {
		slog.Warn("Landlock não aplicado", "error", err)
		return nil
	}
	attr := rulesetAttr{handledAccessFs: landlockAccessFSExecute | landlockAccessFSWriteFile | landlockAccessFSReadFile | landlockAccessFSReadDir | landlockAccessFSRemoveDir | landlockAccessFSRemoveFile | landlockAccessFSMakeDir | landlockAccessFSMakeReg}
	fd, _, errno := syscall.Syscall(sysLandlockCreateRuleset, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		return errno
	}
	for _, p := range []string{"/lib", "/lib64", "/usr/lib", "/usr/lib64"} {
		if _, statErr := os.Stat(p); statErr == nil {
			_ = addRule(int(fd), p, true)
		}
	}
	_ = addRule(int(fd), workDir, false)
	for _, a := range allowed {
		_ = addRule(int(fd), a.Path, a.ReadOnly)
	}
	_ = forbidden
	_, _, errno = syscall.Syscall(sysLandlockRestrictSelf, fd, 0, 0)
	if errno != 0 {
		slog.Warn("landlock_restrict_self falhou", "error", errno)
	}
	return nil
}

func addRule(ruleset int, path string, readOnly bool) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	parent, err := syscall.Open(abs, oPath|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil
	}
	defer syscall.Close(parent)

	mask := uint64(landlockAccessFSReadFile | landlockAccessFSReadDir | landlockAccessFSExecute)
	if !readOnly {
		mask |= landlockAccessFSWriteFile | landlockAccessFSRemoveDir | landlockAccessFSRemoveFile | landlockAccessFSMakeDir | landlockAccessFSMakeReg
	}
	pba := pathBeneathAttr{allowedAccess: mask, parentFd: int32(parent)} //nolint:gosec // descritor fd do kernel
	_, _, errno := syscall.Syscall6(sysLandlockAddRule, uintptr(ruleset), landlockRulePathBeneath, uintptr(unsafe.Pointer(&pba)), 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
