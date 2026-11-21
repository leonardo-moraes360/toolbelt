package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// vectorRepo is a minimal repository with the graph enabled and an isolated
// user config (HOME/XDG_CONFIG_HOME), so the real ~/.config never leaks in.
type vectorRepo struct {
	dir, home, userCfg string
}

func newVectorRepo(t *testing.T) *vectorRepo {
	t.Helper()
	r := &vectorRepo{dir: t.TempDir(), home: t.TempDir()}
	mustWrite(t, filepath.Join(r.dir, "go.mod"), "module example.com/v\n\ngo 1.22\n")
	mustWrite(t, filepath.Join(r.dir, "auth.go"), "package v\n\n// lstatChain refuses sources reached through a symlink.\nfunc lstatChain() {}\n")
	if err := os.MkdirAll(filepath.Join(r.dir, ".toolbelt"), 0o700); err != nil {
		t.Fatal(err)
	}
	r.userCfg = filepath.Join(r.home, ".config", "toolbelt", "config.yaml")
	return r
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (r *vectorRepo) run(t *testing.T, tbBin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(tbBin, args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "TB_GRAPH=on", "HOME="+r.home, "XDG_CONFIG_HOME="+filepath.Join(r.home, ".config"))
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		code = ee.ExitCode()
	default:
		t.Fatalf("running tb: %v", err)
	}
	return out.String(), errb.String(), code
}

func TestGraphVectorsDisabledByDefault(t *testing.T) {
	tbBin := buildTB(t)
	r := newVectorRepo(t)
	if _, se, code := r.run(t, tbBin, "graph", "index"); code != 0 {
		t.Fatalf("graph index: %d\n%s", code, se)
	}
	for _, args := range [][]string{
		{"graph", "index", "--vectors"},
		{"graph", "search", "symlink", "--mode", "semantic"},
		{"graph", "search", "symlink", "--mode", "hybrid"},
	} {
		_, se, code := r.run(t, tbBin, args...)
		if code == 0 || !strings.Contains(se, "vectors_disabled") {
			t.Fatalf("%v: want vectors_disabled, got exit %d\n%s", args, code, se)
		}
	}
	if _, err := os.Stat(filepath.Join(r.dir, ".toolbelt", "graph.vec")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("graph.vec must not exist when vectors are disabled: %v", err)
	}
}

// Acceptance 8: the default mode is the release-before-V1 lexical output.
func TestGraphSearchLexicalModeUnchanged(t *testing.T) {
	tbBin := buildTB(t)
	r := newVectorRepo(t)
	if _, se, code := r.run(t, tbBin, "graph", "index"); code != 0 {
		t.Fatalf("graph index: %d\n%s", code, se)
	}
	def, _, c1 := r.run(t, tbBin, "--json", "graph", "search", "lstatChain")
	lex, _, c2 := r.run(t, tbBin, "--json", "graph", "search", "lstatChain", "--mode", "lexical")
	if c1 != 0 || c2 != 0 || def != lex {
		t.Fatalf("lexical mode differs from the default:\n%s\n---\n%s", def, lex)
	}
	for _, k := range []string{"score_semantic", "lines", "stale_vectors", `"mode"`} {
		if strings.Contains(def, k) {
			t.Fatalf("lexical JSON gained %s:\n%s", k, def)
		}
	}
	if _, se, code := r.run(t, tbBin, "graph", "search", "x", "--mode", "fuzzy"); code == 0 || !strings.Contains(se, "invalid --mode") {
		t.Fatalf("invalid mode accepted: %d\n%s", code, se)
	}
}

// A repository cannot turn vectors on or choose the model.
func TestGraphVectorsRepoConfigUntrusted(t *testing.T) {
	tbBin := buildTB(t)
	r := newVectorRepo(t)
	mustWrite(t, filepath.Join(r.dir, ".toolbelt", "config.yaml"),
		"graph:\n  vector:\n    enabled: true\n  embed:\n    model: /tmp/evil.gguf\n")
	_, se, code := r.run(t, tbBin, "graph", "index", "--vectors")
	if code == 0 || !strings.Contains(se, "vectors_disabled") {
		t.Fatalf("repo config enabled vectors: exit %d\n%s", code, se)
	}
	if !strings.Contains(se, "config_untrusted") || !strings.Contains(se, "graph.embed.model") || !strings.Contains(se, "graph.vector.enabled") {
		t.Fatalf("missing config_untrusted warning naming the keys:\n%s", se)
	}
}

// With vectors on and a well-formed model, an untagged build reaches the
// child, which has no engine: the failure is reported, nothing is written.
func TestGraphVectorsUntaggedChild(t *testing.T) {
	tbBin := buildTB(t)
	r := newVectorRepo(t)
	modelDir := t.TempDir()
	model := filepath.Join(modelDir, "m.gguf")
	mustWrite(t, model, "not a real model")
	sum := sha256.Sum256([]byte("not a real model"))
	mustWrite(t, r.userCfg, "graph:\n  vector:\n    enabled: true\n  embed:\n    model: "+model+
		"\n    model_sha256: "+hex.EncodeToString(sum[:])+"\n")
	_, se, code := r.run(t, tbBin, "graph", "index", "--vectors")
	if code == 0 {
		t.Fatalf("untagged child must fail:\n%s", se)
	}
	if !strings.Contains(se, "embed_unavailable") && !strings.Contains(se, "embed_unsandboxed") && !strings.Contains(se, "model_") {
		t.Fatalf("want an embed_*/model_* reason:\n%s", se)
	}
	if _, err := os.Stat(filepath.Join(r.dir, ".toolbelt", "graph.vec")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("graph.vec written after a child failure: %v", err)
	}
	// The wrong hash is refused before any child starts (acceptance 9).
	mustWrite(t, r.userCfg, "graph:\n  vector:\n    enabled: true\n  embed:\n    model: "+model+
		"\n    model_sha256: "+strings.Repeat("0", 64)+"\n")
	if _, se, code := r.run(t, tbBin, "graph", "index", "--vectors"); code == 0 || !strings.Contains(se, "model_mismatch") {
		t.Fatalf("want model_mismatch: %d\n%s", code, se)
	}
}

// Acceptance 9: a model inside the repository is refused even with the
// right hash, since the repository would choose the bytes the parser reads.
func TestGraphVectorsRefusesRepoLocalModel(t *testing.T) {
	tbBin := buildTB(t)
	r := newVectorRepo(t)
	model := filepath.Join(r.dir, "models", "m.gguf")
	mustWrite(t, model, "repo model")
	sum := sha256.Sum256([]byte("repo model"))
	mustWrite(t, r.userCfg, "graph:\n  vector:\n    enabled: true\n  embed:\n    model: "+model+
		"\n    model_sha256: "+hex.EncodeToString(sum[:])+"\n")
	_, se, code := r.run(t, tbBin, "graph", "index", "--vectors")
	if code == 0 || !strings.Contains(se, "model_path") || !strings.Contains(se, "inside the repository") {
		t.Fatalf("want model_path for a repo-local model: %d\n%s", code, se)
	}
}
