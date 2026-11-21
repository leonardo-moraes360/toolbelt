package embed

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
)

func init() {
	fakeModes["hostile"] = runHostile
}

// runHostile tries to escape before serving and answers every request with
// what it managed, as JSON in an error reply.
func runHostile(sha string, args []string) {
	repoFile, homeDir, homeFile := args[0], args[1], args[2]
	try := func(err error) string {
		if err != nil {
			return "denied"
		}
		return "ok"
	}
	res := map[string]string{}
	_, err := os.ReadFile(repoFile)
	res["read_repo"] = try(err)
	_, err = os.ReadDir(filepath.Dir(repoFile))
	res["list_repo"] = try(err)
	_, err = os.ReadDir(homeDir)
	res["list_home"] = try(err)
	if homeFile != "" {
		_, err = os.ReadFile(homeFile)
		res["read_home"] = try(err)
	}
	tmp := os.Getenv("TMPDIR")
	res["write_tmp"] = try(os.WriteFile(filepath.Join(tmp, "inside"), []byte("x"), 0o600))
	res["write_outside_tmp"] = try(os.WriteFile(filepath.Join(filepath.Dir(tmp), "escape"), []byte("x"), 0o600))
	res["write_repo"] = try(os.WriteFile(filepath.Join(filepath.Dir(repoFile), "escape"), []byte("x"), 0o600))
	res["write_workdir"] = try(os.WriteFile("escape", []byte("x"), 0o600))
	for name, fam := range map[string]int{"socket_inet": syscall.AF_INET, "socket_inet6": syscall.AF_INET6, "socket_unix": syscall.AF_UNIX} {
		fd, err := syscall.Socket(fam, syscall.SOCK_STREAM, 0)
		if err == nil {
			_ = syscall.Close(fd)
		}
		res[name] = try(err)
	}
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err == nil {
		_, _ = syscall.Close(fds[0]), syscall.Close(fds[1])
	}
	res["socketpair"] = try(err)
	// io_uring can open sockets without socket(2), so seccomp blocks its setup.
	var params [120]byte
	if r, _, e := syscall.Syscall(unix.SYS_IO_URING_SETUP, 1, uintptr(unsafe.Pointer(&params[0])), 0); e == 0 {
		_ = syscall.Close(int(r))
		res["io_uring"] = "ok"
	} else {
		res["io_uring"] = "denied"
	}
	// Signals to processes outside the sandbox (the parent tb) are scoped
	// out by Landlock ABI >= 6; signal 0 only checks permission.
	res["signal_parent"] = try(syscall.Kill(os.Getppid(), 0))
	b, _ := json.Marshal(res)
	_ = serve(context.Background(), os.Stdin, os.Stdout, fakeInfo(sha), funcEngine(func(string) ([]float32, bool, error) {
		return nil, false, errors.New(string(b))
	}))
}

// Acceptance 14: under Landlock and seccomp the child reads neither the
// repository nor HOME, writes only its private tmp and opens no socket.
func TestHostileChildIsContained(t *testing.T) {
	if !sandbox.NetworkDenyAvailable() {
		t.Skip("Landlock with seccomp network denial unavailable on this host")
	}
	repo := t.TempDir()
	repoFile := filepath.Join(repo, "secret.go")
	if err := os.WriteFile(repoFile, []byte("package secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	homeFile := readableHomeFile(home)
	o := fakeOptions(t, "hostile", repoFile, home, homeFile)
	o.AllowUnsandboxed = false
	c, err := Start(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.Unsandboxed() {
		t.Fatal("child reported unsandboxed")
	}
	_, _, err = c.Embed(context.Background(), KindDoc, []string{"a"})
	if err == nil {
		t.Fatal("hostile child sent no report")
	}
	var res map[string]string
	if err := json.Unmarshal([]byte(childMsg(t, err)), &res); err != nil {
		t.Fatalf("report %q: %v", childMsg(t, err), err)
	}
	if res["write_tmp"] != "ok" {
		t.Fatalf("the child cannot write its own tmp, the test proves nothing: %v", res)
	}
	delete(res, "write_tmp")
	for _, k := range wantDenied(homeFile != "") {
		if res[k] != "denied" {
			t.Errorf("%s = %q, want denied", k, res[k])
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "escape")); err == nil {
		t.Error("the child wrote into the repository")
	}
}

// readableHomeFile returns a regular file in home the test process can
// open, or "" when there is none, so read_home is only checked if it can
// be denied.
func readableHomeFile(home string) string {
	entries, err := os.ReadDir(home)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		if f, err := os.Open(filepath.Join(home, e.Name())); err == nil {
			_ = f.Close()
			return filepath.Join(home, e.Name())
		}
	}
	return ""
}

// wantDenied lists the probes the sandbox must deny on this host.
func wantDenied(haveHomeFile bool) []string {
	want := []string{"read_repo", "list_repo", "list_home", "write_outside_tmp", "write_repo", "write_workdir", "socket_inet", "socket_inet6", "socket_unix", "socketpair"}
	want = append(want, "io_uring")
	if haveHomeFile {
		want = append(want, "read_home")
	}
	if _, abi, _ := sandbox.CheckLandlockSupport(); abi >= 6 {
		want = append(want, "signal_parent")
	}
	return want
}
