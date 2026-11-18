//go:build linux

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// O diretório temporário do sistema não é gravável no sandbox: com o workdir
// fora de os.TempDir(), escrever em TMPDIR deve falhar.
func TestSandboxSharedTmpNotWritable(t *testing.T) {
	work := t.TempDir()
	tmp := t.TempDir() // irmão do workdir: não é ancestral nem descendente
	t.Setenv("TMPDIR", tmp)
	if got := os.TempDir(); got != tmp {
		t.Fatalf("os.TempDir() = %q, want %q", got, tmp)
	}
	sb := newLandlockSandbox(t, work)

	target := filepath.Join(tmp, "leak.txt")
	res, err := sb.Run(context.Background(), "sh", "-c", `echo x > "$TMPDIR/leak.txt"`)
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatalf("escreveu em os.TempDir() fora do workdir (%s): exit=%d output=%q", target, res.ExitCode, res.Output)
	}
	if res.ExitCode == 0 {
		t.Fatalf("escrita em TMPDIR deveria falhar: output=%q", res.Output)
	}
}
