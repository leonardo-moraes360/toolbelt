package verify

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestHiddenPassedNullWhenUnavailableJSON(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "a@b.c")
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "commit", "--allow-empty", "-m", "c")

	cfg := DefaultConfig(dir)
	cfg.BaseRef = "HEAD"
	cfg.HeadRef = "HEAD"
	cfg.Timeout = 30 * time.Second
	cfg.MaxIterations = 1
	cfg.VisibleTest = "true"
	cfg.HiddenTest = ""

	res, err := Run(context.Background(), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["hidden_available"] != false {
		t.Fatalf("hidden_available=%v", payload["hidden_available"])
	}
	if payload["hidden_passed"] != nil {
		t.Fatalf("hidden_passed=%v want null in %s", payload["hidden_passed"], raw)
	}
}
