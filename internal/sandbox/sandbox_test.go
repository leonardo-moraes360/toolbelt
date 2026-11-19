package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSandboxRun(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{WorkDir: dir, Timeout: 5 * time.Second}
	if runtime.GOOS != "linux" || !kernelLandlockPresent() {
		cfg.AllowUnsandboxed = true
	}
	sb, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	res, err := sb.Run(context.Background(), "echo", "ok")
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit %d output=%q", res.ExitCode, res.Output)
	}
	if kernelLandlockPresent() && !res.Sandboxed {
		t.Fatal("esperado Sandboxed=true com Landlock disponível")
	}
}

func TestCheckLandlockSupportABI(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Landlock só no Linux")
	}
	if !kernelLandlockPresent() {
		t.Skip("kernel sem Landlock")
	}
	ok, ver, err := CheckLandlockSupport()
	if err != nil {
		t.Fatalf("CheckLandlockSupport: %v", err)
	}
	if !ok {
		t.Fatal("Landlock deveria estar disponível")
	}
	if ver < 1 {
		t.Fatalf("ABI inválida: %d", ver)
	}
}

func TestNewRequiresAllowUnsandboxedWithoutLandlock(t *testing.T) {
	if runtime.GOOS == "linux" && kernelLandlockPresent() {
		t.Skip("kernel com Landlock; fail-closed sem suporte não exercitável aqui")
	}
	_, err := New(Config{WorkDir: t.TempDir()})
	if err == nil {
		t.Fatal("New deveria falhar sem Landlock e sem AllowUnsandboxed")
	}
	sb, err := New(Config{WorkDir: t.TempDir(), AllowUnsandboxed: true})
	if err != nil {
		t.Fatal(err)
	}
	if sb.landlockOK {
		t.Fatal("não deveria marcar landlockOK")
	}
}

func TestSandboxValidateForbidden(t *testing.T) {
	dir := t.TempDir()
	forbidden := filepath.Join(dir, "secret")
	_ = os.MkdirAll(forbidden, 0o755)
	cfg := Config{WorkDir: dir, ForbiddenPaths: []string{forbidden}}
	if runtime.GOOS != "linux" || !kernelLandlockPresent() {
		cfg.AllowUnsandboxed = true
	}
	sb, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if sb.ValidatePath(filepath.Join(forbidden, "x")) == nil {
		t.Fatal("deveria negar")
	}
}

func TestSandboxBlocksWriteOutsideWorkdir(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Landlock só no Linux")
	}
	if !kernelLandlockPresent() {
		t.Skip("kernel sem Landlock; teste de isolamento pulado")
	}

	work := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "pwned.txt")

	sb, err := New(Config{WorkDir: work, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}

	res, err := sb.Run(context.Background(), "sh", "-c", "echo pwned > '"+target+"'")
	if err != nil {
		t.Fatal(err)
	}

	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatalf("escreveu fora do workdir (%s); isolamento Landlock falhou (exit=%d sandboxed=%v output=%q)",
			target, res.ExitCode, res.Sandboxed, res.Output)
	}

	inside := filepath.Join(work, "ok.txt")
	res2, err := sb.Run(context.Background(), "sh", "-c", "echo ok > '"+inside+"'")
	if err != nil {
		t.Fatal(err)
	}
	if res2.ExitCode != 0 {
		t.Fatalf("escrita no workdir deveria ser permitida: exit=%d output=%q", res2.ExitCode, res2.Output)
	}
	if _, statErr := os.Stat(inside); statErr != nil {
		t.Fatalf("arquivo no workdir ausente: %v", statErr)
	}
}

func TestSandboxEnvAndCancelKillsProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("grupos de processo não suportados")
	}
	dir := t.TempDir()
	sb, err := New(Config{
		WorkDir: dir, AllowUnsandboxed: true, ProcessGroup: true,
		Env: []string{"PATH=" + os.Getenv("PATH"), "SO_ESTA=sim"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("NAO_HERDAR", "x")
	res, err := sb.Run(context.Background(), "sh", "-c", "echo $SO_ESTA${NAO_HERDAR}")
	if err != nil || res.Output != "sim\n" {
		t.Fatalf("env explícito não aplicado: %q %v", res.Output, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	// the grandchild (sleep) holds the output pipe; the whole group must die
	res, err = sb.Run(ctx, "sh", "-c", "sleep 30 & sleep 30")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Canceled || res.TimedOut || res.ExitCode != -1 {
		t.Fatalf("esperava Canceled: %+v", res)
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("cancelamento não matou o grupo de processos")
	}
}

// Finding 7: fail-closed is exercised on any host (including CI with
// Landlock) by simulating an "unavailable" probe.
func TestNewFailClosedWithSimulatedNoLandlock(t *testing.T) {
	restore := OverrideLandlockProbe(func() (bool, int, error) { return false, 0, errors.New("simulado: sem Landlock") })
	defer restore()
	work := t.TempDir()
	if _, err := New(Config{WorkDir: work}); err == nil {
		t.Fatal("New deveria recusar sem Landlock e sem AllowUnsandboxed")
	} else if !strings.Contains(err.Error(), "--allow-unsandboxed") {
		t.Fatalf("erro deveria orientar o opt-in explícito: %v", err)
	}
	sb, err := New(Config{WorkDir: work, AllowUnsandboxed: true})
	if err != nil {
		t.Fatalf("opt-in explícito deveria permitir: %v", err)
	}
	if sb.landlockOK {
		t.Fatal("não deveria marcar landlockOK")
	}
	res, err := sb.Run(context.Background(), "echo", "ok")
	if err != nil || res.Sandboxed || res.ExitCode != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestOverrideLandlockProbeRestores(t *testing.T) {
	restore := OverrideLandlockProbe(func() (bool, int, error) { return false, 0, errors.New("x") })
	restore()
	if runtime.GOOS == "linux" && kernelLandlockPresent() {
		sb, err := New(Config{WorkDir: t.TempDir()})
		if err != nil || !sb.landlockOK {
			t.Fatalf("sonda real não restaurada: %v", err)
		}
	}
}
