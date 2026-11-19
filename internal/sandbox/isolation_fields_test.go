package sandbox

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func requireLandlock(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("Landlock só no Linux")
	}
	if !kernelLandlockPresent() {
		t.Skip("kernel sem Landlock; teste de isolamento pulado")
	}
}

func runSh(t *testing.T, cfg Config, script string) *Result {
	t.Helper()
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	sb, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	res, err := sb.Run(context.Background(), "sh", "-c", script)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestReadOnlyWorkDirBlocksWrite(t *testing.T) {
	requireLandlock(t)
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "in.txt"), []byte("dado"), 0o600); err != nil {
		t.Fatal(err)
	}
	priv := t.TempDir()
	res := runSh(t, Config{WorkDir: work, ReadOnlyWorkDir: true, PrivateTmp: priv},
		"cat in.txt && echo x > novo.txt")
	if !strings.Contains(res.Output, "dado") {
		t.Fatalf("leitura do workdir deveria funcionar: %q", res.Output)
	}
	if _, err := os.Stat(filepath.Join(work, "novo.txt")); err == nil {
		t.Fatalf("escreveu num workdir só-leitura (exit=%d output=%q)", res.ExitCode, res.Output)
	}
}

func TestPrivateTmpIsTheOnlyTmp(t *testing.T) {
	requireLandlock(t)
	// O workdir fica fora de os.TempDir() para não depender da regra de tmp
	// compartilhado. Hoje essa regra nunca é aplicada (isSubpath trata "../x"
	// como subcaminho), então o teste só afirma o que PrivateTmp garante:
	// TMPDIR privado gravável, e nada mais em os.TempDir().
	work, err := os.MkdirTemp(".", "work-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(work) })
	priv := t.TempDir()
	other := t.TempDir()
	res := runSh(t, Config{WorkDir: work, ReadOnlyWorkDir: true, PrivateTmp: priv},
		`echo ok > "$TMPDIR/a"; echo pwned > '`+filepath.Join(other, "b")+`'`)
	if _, err := os.Stat(filepath.Join(priv, "a")); err != nil {
		t.Fatalf("TMPDIR privado deveria ser gravável: %v (output=%q)", err, res.Output)
	}
	if _, err := os.Stat(filepath.Join(other, "b")); err == nil {
		t.Fatalf("com PrivateTmp, escreveu fora dele (output=%q)", res.Output)
	}
}

func TestHomeAndPathOverrideEnv(t *testing.T) {
	requireLandlock(t)
	work := t.TempDir()
	home := t.TempDir()
	res := runSh(t, Config{
		WorkDir: work,
		Home:    home,
		Path:    "/usr/bin:/bin",
		Env:     []string{"HOME=/nao/deve/aparecer", "PATH=/nao/deve", "FOO=bar"},
	}, `echo "H=$HOME P=$PATH X=$XDG_CONFIG_HOME F=$FOO"; echo ok > "$HOME/escrito"`)
	want := "H=" + home + " P=/usr/bin:/bin X=" + filepath.Join(home, ".config") + " F=bar"
	if !strings.Contains(res.Output, want) {
		t.Fatalf("env: esperado %q em %q", want, res.Output)
	}
	if _, err := os.Stat(filepath.Join(home, "escrito")); err != nil {
		t.Fatalf("Home deveria ser gravável: %v", err)
	}
}

func TestStdinAndSeparateStdout(t *testing.T) {
	requireLandlock(t)
	var out bytes.Buffer
	res := runSh(t, Config{
		WorkDir: t.TempDir(),
		Stdin:   strings.NewReader("entrada\n"),
		Stdout:  &out,
	}, `cat; echo '{"falso":true}' >&2`)
	if out.String() != "entrada\n" {
		t.Fatalf("stdout = %q, esperado só a entrada", out.String())
	}
	if strings.Contains(res.Output, "entrada") || !strings.Contains(res.Output, `{"falso":true}`) {
		t.Fatalf("stderr deveria ficar separado em Output: %q", res.Output)
	}
	if res.StdoutOverflow {
		t.Fatal("overflow inesperado")
	}
}

func TestStdoutOverflowDrainsWithoutBlocking(t *testing.T) {
	requireLandlock(t)
	var out bytes.Buffer
	res := runSh(t, Config{
		WorkDir:        t.TempDir(),
		Stdout:         &out,
		MaxStdoutBytes: 100,
	}, `head -c 200000 /dev/zero; echo fim >&2`)
	if res.TimedOut || res.ExitCode != 0 {
		t.Fatalf("o filho deveria terminar normalmente: exit=%d timeout=%v", res.ExitCode, res.TimedOut)
	}
	if !res.StdoutOverflow || out.Len() != 100 {
		t.Fatalf("overflow=%v len=%d, esperado true e 100", res.StdoutOverflow, out.Len())
	}
}

func TestAllowedRegularFileRule(t *testing.T) {
	requireLandlock(t)
	dir := t.TempDir()
	f := filepath.Join(dir, "unico.txt")
	if err := os.WriteFile(f, []byte("conteudo"), 0o600); err != nil {
		t.Fatal(err)
	}
	vizinho := filepath.Join(dir, "vizinho.txt")
	if err := os.WriteFile(vizinho, []byte("segredo"), 0o600); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	res := runSh(t, Config{
		WorkDir:         work,
		ReadOnlyWorkDir: true,
		PrivateTmp:      t.TempDir(),
		AllowedPaths:    []PathAccess{{Path: f, ReadOnly: true}},
	}, "cat '"+f+"'; cat '"+vizinho+"'")
	if !strings.Contains(res.Output, "conteudo") {
		t.Fatalf("regra de arquivo deveria liberar a leitura (sem EINVAL): %q", res.Output)
	}
	if strings.Contains(res.Output, "segredo") {
		t.Fatalf("regra de arquivo não deveria liberar o diretório pai: %q", res.Output)
	}
}

func TestMinimalDevHidesPtsAndShm(t *testing.T) {
	requireLandlock(t)
	res := runSh(t, Config{WorkDir: t.TempDir(), MinimalDev: true},
		`echo x > /dev/null && echo null-ok; head -c 4 /dev/urandom >/dev/null && echo urandom-ok; ls /dev/shm >/dev/null 2>&1 && echo shm-visivel; ls /dev/pts >/dev/null 2>&1 && echo pts-visivel; true`)
	for _, want := range []string{"null-ok", "urandom-ok"} {
		if !strings.Contains(res.Output, want) {
			t.Fatalf("esperado %q em %q", want, res.Output)
		}
	}
	for _, bad := range []string{"shm-visivel", "pts-visivel"} {
		if strings.Contains(res.Output, bad) {
			t.Fatalf("MinimalDev deveria esconder: %q em %q", bad, res.Output)
		}
	}
}

func TestAbsoluteCommandSkipsLookPath(t *testing.T) {
	requireLandlock(t)
	sb, err := New(Config{WorkDir: t.TempDir(), Timeout: 5 * time.Second, Path: "/nao/existe"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := sb.Run(context.Background(), "/bin/sh", "-c", "echo absoluto")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Output, "absoluto") {
		t.Fatalf("comando absoluto deveria rodar mesmo com PATH inútil: %q", res.Output)
	}
}
