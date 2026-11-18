//go:build linux

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// O diretório temporário do sistema não entra no ruleset: com o workdir fora
// de os.TempDir(), o filho não lê nem escreve em TMPDIR. Pulado (como os
// demais testes Landlock) em host sem Landlock.
func TestSandboxSharedTmpNoAccess(t *testing.T) {
	work := t.TempDir()
	tmp := t.TempDir() // irmão do workdir: não é ancestral nem descendente
	if err := os.WriteFile(filepath.Join(tmp, "seed.txt"), []byte("seed"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	if got := os.TempDir(); got != tmp {
		t.Fatalf("os.TempDir() = %q, want %q", got, tmp)
	}
	sb := newLandlockSandbox(t, work)

	// Controle positivo: mesmo sandbox, mesmo shell, escrita no workdir passa
	// e TMPDIR chega ao filho.
	res, err := sb.Run(context.Background(), "sh", "-c", `echo ok > ./ok.txt && printf '%s' "$TMPDIR"`)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || strings.TrimSpace(res.Output) != tmp {
		t.Fatalf("controle: exit=%d output=%q (want TMPDIR=%q)", res.ExitCode, res.Output, tmp)
	}
	if _, statErr := os.Stat(filepath.Join(work, "ok.txt")); statErr != nil {
		t.Fatalf("controle: arquivo no workdir ausente: %v", statErr)
	}

	cases := []struct{ name, script string }{
		{"escrita", `echo x > "$TMPDIR/leak.txt"`},
		{"leitura", `cat "$TMPDIR/seed.txt"`},
	}
	for _, c := range cases {
		res, err := sb.Run(context.Background(), "sh", "-c", c.script)
		if err != nil {
			t.Fatal(err)
		}
		if res.ExitCode == 0 || !strings.Contains(res.Output, "Permission denied") {
			t.Errorf("%s em TMPDIR deveria dar EACCES: exit=%d output=%q", c.name, res.ExitCode, res.Output)
		}
	}
	if _, statErr := os.Stat(filepath.Join(tmp, "leak.txt")); statErr == nil {
		t.Fatal("escreveu em os.TempDir() fora do workdir")
	}
}
