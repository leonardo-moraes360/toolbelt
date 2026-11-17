package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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
