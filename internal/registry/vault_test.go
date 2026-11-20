package registry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultToolsNewIsInferred(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".toolbelt", "vault"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if res, err := Default().Call(ctx, "tb_vault_new", map[string]any{"cwd": root, "title": "x"}); err != nil || !res.IsError {
		t.Fatalf("new without confidence accepted: %+v %v", res, err)
	}
	res, err := Default().Call(ctx, "tb_vault_new", map[string]any{"cwd": root, "title": "Agent note", "confidence": 0.4, "source": "manual"})
	if err != nil || res.IsError {
		t.Fatalf("%+v %v", res, err)
	}
	b, _ := os.ReadFile(filepath.Join(root, ".toolbelt", "vault", "agent-note.md"))
	if !strings.Contains(string(b), "source: inferred") || !strings.Contains(string(b), "confidence: 0.4") {
		t.Fatalf("note:\n%s\nresult %v", b, res)
	}
	show, err := Default().Call(ctx, "tb_vault_show", map[string]any{"cwd": root, "note": "agent-note"})
	if err != nil || show.IsError {
		t.Fatalf("%+v %v", show, err)
	}
	out, _ := json.Marshal(show.Content)
	if !strings.Contains(string(out), `data`) || !strings.Contains(string(out), "Agent note") {
		t.Fatalf("show: %s", out)
	}
	if res, err := Default().Call(ctx, "tb_vault_check", map[string]any{"cwd": "relative"}); err != nil || !res.IsError {
		t.Fatalf("relative cwd accepted: %+v %v", res, err)
	}
}
