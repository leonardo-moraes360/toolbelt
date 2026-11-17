//go:build !windows

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func groupSandbox(t *testing.T, dir string) *Sandbox {
	t.Helper()
	cfg := Config{WorkDir: dir, ProcessGroup: true}
	if runtime.GOOS != "linux" || !kernelLandlockPresent() {
		cfg.AllowUnsandboxed = true
	}
	sb, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return sb
}

func processGone(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil {
		return true
	}
	if data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
		// zumbi (estado Z) conta como encerrado
		if i := strings.LastIndexByte(string(data), ')'); i >= 0 && len(data) > i+2 && data[i+2] == 'Z' {
			return true
		}
	}
	return false
}

func TestSandboxBackgroundChildExit0IsSuccessAndKilled(t *testing.T) {
	dir := t.TempDir()
	sb := groupSandbox(t, dir)
	res, err := sb.Run(context.Background(), "sh", "-c", "sleep 30 & echo $! > bg.pid; exit 0")
	if err != nil {
		t.Fatalf("saída 0 com filho em segundo plano deveria ser sucesso: %v", err)
	}
	if res.ExitCode != 0 || res.Canceled || res.TimedOut {
		t.Fatalf("resultado: %+v", res)
	}
	data, err := os.ReadFile(filepath.Join(dir, "bg.pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if pid <= 0 {
		t.Fatalf("pid inválido %q", data)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !processGone(pid) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("filho %d sobreviveu ao agente", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSandboxOutputCappedToTail(t *testing.T) {
	dir := t.TempDir()
	sb := groupSandbox(t, dir)
	// ~3 MiB de 'a' seguidos de um marcador final
	res, err := sb.Run(context.Background(), "sh", "-c", "head -c 3145728 /dev/zero | tr '\\0' a; echo FIM")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Output) > MaxOutputBytes+len(truncatedMarker) {
		t.Fatalf("saída não limitada: %d bytes", len(res.Output))
	}
	if !strings.HasPrefix(res.Output, "[...") || !strings.HasSuffix(res.Output, "FIM\n") {
		t.Fatalf("esperava marcador de truncamento e o tail preservado")
	}
}
