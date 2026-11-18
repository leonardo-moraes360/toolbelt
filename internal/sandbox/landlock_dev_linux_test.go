//go:build linux

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newLandlockSandbox(t *testing.T, work string) *Sandbox {
	t.Helper()
	if !kernelLandlockPresent() {
		t.Skip("kernel sem Landlock")
	}
	if ok, _, _ := CheckLandlockSupport(); !ok {
		t.Skip("Landlock indisponível")
	}
	sb, err := New(Config{WorkDir: work, Timeout: 20 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return sb
}

func TestSandboxDevNullWritable(t *testing.T) {
	sb := newLandlockSandbox(t, t.TempDir())
	res, err := sb.Run(context.Background(), "sh", "-c", "echo x > /dev/null")
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("escrita em /dev/null deveria funcionar: exit=%d output=%q", res.ExitCode, res.Output)
	}
}

func TestSandboxDevRandomReadable(t *testing.T) {
	sb := newLandlockSandbox(t, t.TempDir())
	res, err := sb.Run(context.Background(), "sh", "-c", "head -c1 /dev/urandom | wc -c")
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("leitura de /dev/urandom falhou: exit=%d output=%q", res.ExitCode, res.Output)
	}
}

func TestSandboxDevDoesNotOpenOtherWrites(t *testing.T) {
	sb := newLandlockSandbox(t, t.TempDir())
	// /dev as a whole stays read-only: creating a file in /dev and writing to /dev/urandom fail.
	for _, script := range []string{
		"echo x > /dev/tb-sandbox-probe",
		"echo x > /dev/urandom",
		"echo x > /etc/tb-sandbox-probe",
	} {
		res, err := sb.Run(context.Background(), "sh", "-c", script)
		if err != nil {
			t.Fatal(err)
		}
		if res.ExitCode == 0 {
			t.Fatalf("%q deveria falhar no sandbox (output=%q)", script, res.Output)
		}
	}
	for _, p := range []string{"/dev/tb-sandbox-probe", "/etc/tb-sandbox-probe"} {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("%s foi criado", p)
		}
	}
}

func TestSandboxGitInitStatus(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git não instalado")
	}
	work := t.TempDir()
	if !kernelLandlockPresent() {
		t.Skip("kernel sem Landlock")
	}
	// HOME points at the workdir: the real ~/.gitconfig is not (and must not be) reachable in the sandbox.
	sb, err := New(Config{WorkDir: work, Timeout: 20 * time.Second, Env: []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + work, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
	}})
	if err != nil {
		t.Fatal(err)
	}
	script := "git init -q && git status --porcelain"
	res, err := sb.Run(context.Background(), "sh", "-c", script)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("git init/status falhou no sandbox: exit=%d output=%q", res.ExitCode, res.Output)
	}
	if _, err := os.Stat(filepath.Join(work, ".git")); err != nil {
		t.Fatalf(".git ausente: %v", err)
	}
}

func TestDeviceRuleMaskFileScoped(t *testing.T) {
	const fileRights = landlockAccessFSExecute | landlockAccessFSWriteFile | landlockAccessFSReadFile |
		landlockAccessFSTruncate
	for abi := 1; abi <= 7; abi++ {
		for _, readOnly := range []bool{false, true} {
			m := deviceFileMask(readOnly, abi)
			if m&^uint64(fileRights) != 0 {
				t.Fatalf("abi=%d ro=%v: máscara com direitos de diretório/ioctl: %#x", abi, readOnly, m)
			}
			if m&landlockAccessFSIoctlDev != 0 {
				t.Fatalf("abi=%d ro=%v: ioctl_dev não deve ser concedido", abi, readOnly)
			}
		}
	}
	for _, d := range deviceRules {
		if strings.HasPrefix(d.path, "/dev/tty") || strings.HasPrefix(d.path, "/dev/pts") || d.path == "/dev/ptmx" {
			t.Fatalf("regra de tty não permitida: %s", d.path)
		}
	}
}

func TestSandboxDevTTYNotWritable(t *testing.T) {
	sb := newLandlockSandbox(t, t.TempDir())
	var targets []string
	// /dev/tty only counts if it opens outside the sandbox (there is a controlling terminal).
	if f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0); err == nil {
		f.Close()
		targets = append(targets, "/dev/tty")
	}
	pts, _ := filepath.Glob("/dev/pts/*")
	for _, p := range pts {
		if filepath.Base(p) == "ptmx" {
			continue
		}
		if f, err := os.OpenFile(p, os.O_WRONLY, 0); err == nil {
			f.Close()
			targets = append(targets, p)
		}
	}
	if len(targets) == 0 {
		t.Skip("nenhum tty gravável fora do sandbox")
	}
	for _, p := range targets {
		res, err := sb.Run(context.Background(), "sh", "-c", "printf '' > "+p)
		if err != nil {
			t.Fatal(err)
		}
		if res.ExitCode == 0 {
			t.Fatalf("escrita em %s deveria falhar no sandbox (output=%q)", p, res.Output)
		}
	}
}
