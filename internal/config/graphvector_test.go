package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

const testSHA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// isolate points the global config at an empty dir and the cwd at an empty repo.
func isolate(t *testing.T) (global, repo string) {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", t.TempDir())
	repo = t.TempDir()
	t.Chdir(repo)
	SetFile("")
	t.Cleanup(func() { SetFile("") })
	return filepath.Join(xdg, "toolbelt", DefaultConfigFile), repo
}

func TestGraphVectorDefaults(t *testing.T) {
	cfg := DefaultConfig()
	v, e := cfg.Graph.Vector, cfg.Graph.Embed
	if v != (GraphVectorConfig{MaxChunks: 50000, ChunkLines: 40, ChunkBytes: 1536, OverlapLines: 8}) {
		t.Fatalf("vector defaults = %+v", v)
	}
	want := GraphEmbedConfig{Threads: min(4, runtime.NumCPU()), Timeout: 2 * time.Minute}
	if e != want {
		t.Fatalf("embed defaults = %+v, want %+v", e, want)
	}
	if err := v.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestGraphVectorValidate(t *testing.T) {
	vec := []func(*GraphVectorConfig){
		func(v *GraphVectorConfig) { v.MaxChunks = 0 },
		func(v *GraphVectorConfig) { v.ChunkLines = 0 },
		func(v *GraphVectorConfig) { v.ChunkBytes = 0 },
		func(v *GraphVectorConfig) { v.OverlapLines = -1 },
		func(v *GraphVectorConfig) { v.OverlapLines = v.ChunkLines },
	}
	for i, mut := range vec {
		v := DefaultGraphVectorConfig()
		mut(&v)
		if err := v.Validate(); err == nil || !strings.HasPrefix(err.Error(), "graph.vector.") {
			t.Errorf("vector case %d: err = %v", i, err)
		}
	}
	emb := []func(*GraphEmbedConfig){
		func(e *GraphEmbedConfig) { e.Model = "models/m.gguf" },
		func(e *GraphEmbedConfig) { e.ModelSHA256 = strings.ToUpper(testSHA) },
		func(e *GraphEmbedConfig) { e.ModelSHA256 = testSHA[:63] },
		func(e *GraphEmbedConfig) { e.ModelSHA256 = testSHA[:63] + "g" },
		func(e *GraphEmbedConfig) { e.Threads = 0 },
		func(e *GraphEmbedConfig) { e.Timeout = 0 },
		func(e *GraphEmbedConfig) { e.Timeout = -time.Second },
	}
	for i, mut := range emb {
		e := DefaultGraphEmbedConfig()
		mut(&e)
		if err := e.Validate(); err == nil || !strings.HasPrefix(err.Error(), "graph.embed.") {
			t.Errorf("embed case %d: err = %v", i, err)
		}
	}
	ok := DefaultGraphEmbedConfig()
	ok.Model, ok.ModelSHA256 = "/opt/m.gguf", testSHA
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
}

const userYAML = `graph:
  vector:
    enabled: true
    max_chunks: 10
  embed:
    model: /opt/models/m.gguf
    model_sha256: ` + testSHA + `
    query_prefix: "search_query: "
    timeout: 30s
`

func TestLoadUserGlobal(t *testing.T) {
	global, repo := isolate(t)
	writeFile(t, global, userYAML)
	writeFile(t, filepath.Join(repo, ".toolbelt", "config.yaml"), "graph:\n  embed:\n    model: /evil.gguf\n  vector:\n    max_chunks: 99\n")
	cfg, err := LoadUser("")
	if err != nil {
		t.Fatal(err)
	}
	v, e := cfg.Graph.Vector, cfg.Graph.Embed
	if !v.Enabled || v.MaxChunks != 10 || v.ChunkLines != 40 {
		t.Fatalf("vector = %+v", v)
	}
	if e.Model != "/opt/models/m.gguf" || e.ModelSHA256 != testSHA || e.QueryPrefix != "search_query: " || e.Timeout != 30*time.Second {
		t.Fatalf("embed = %+v", e)
	}
}

func TestLoadUserMissingDefault(t *testing.T) {
	_, repo := isolate(t)
	writeFile(t, filepath.Join(repo, ".toolbelt", "config.yaml"), "graph:\n  vector:\n    enabled: true\n")
	cfg, err := LoadUser("")
	if err != nil {
		t.Fatal(err)
	}
	want := DefaultConfig()
	want.Merge = DefaultMergeConfig() // LoadUser fills the user-only merge section
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("want defaults, got %+v", cfg.Graph)
	}
}

func TestLoadUserOverride(t *testing.T) {
	global, _ := isolate(t)
	writeFile(t, global, "graph:\n  vector:\n    max_chunks: 7\n")
	override := filepath.Join(t.TempDir(), "c.yaml")
	writeFile(t, override, userYAML)
	cfg, err := LoadUser(override)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Graph.Vector.MaxChunks != 10 {
		t.Fatalf("override not used: %+v", cfg.Graph.Vector)
	}
	if _, err := LoadUser(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("missing override must fail")
	}
}

func TestLoadUserInvalid(t *testing.T) {
	global, _ := isolate(t)
	writeFile(t, global, "graph:\n  embed:\n    model: rel/m.gguf\n")
	if _, err := LoadUser(""); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("err = %v", err)
	}
	writeFile(t, global, "graph:\n  vector:\n    chunk_lines: 0\n")
	if _, err := LoadUser(""); err == nil {
		t.Fatal("chunk_lines 0 must fail")
	}
}

func TestLoadDropsRepoGraphVector(t *testing.T) {
	global, repo := isolate(t)
	writeFile(t, global, userYAML)
	writeFile(t, filepath.Join(repo, ".toolbelt", "config.yaml"),
		"graph:\n  max_file_bytes: 1234\n  vector:\n    max_chunks: 99\n    enabled: false\n  embed:\n    model: /evil.gguf\n    allow_unsandboxed: true\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Graph.MaxFileBytes != 1234 {
		t.Fatalf("other repo graph keys must still apply: %d", cfg.Graph.MaxFileBytes)
	}
	if !cfg.Graph.Vector.Enabled || cfg.Graph.Vector.MaxChunks != 10 {
		t.Fatalf("repo vector leaked: %+v", cfg.Graph.Vector)
	}
	if cfg.Graph.Embed.Model != "/opt/models/m.gguf" || cfg.Graph.Embed.AllowUnsandboxed {
		t.Fatalf("repo embed leaked: %+v", cfg.Graph.Embed)
	}
}

func TestInitOmitsGraphVector(t *testing.T) {
	_, repo := isolate(t)
	dir := filepath.Join(repo, ".toolbelt")
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	keys, err := RepoGraphVectorKeys(repo)
	if err != nil || len(keys) != 0 {
		t.Fatalf("init wrote user-only keys: %v %v", keys, err)
	}
}

func TestGraphEmbedRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	cfg := DefaultConfig()
	cfg.Graph.Embed.Timeout = 90 * time.Second
	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}
	got, err := LoadUser(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Graph.Embed != cfg.Graph.Embed || got.Graph.Vector != cfg.Graph.Vector {
		t.Fatalf("round trip: %+v %+v", got.Graph.Embed, got.Graph.Vector)
	}
}

func TestRepoGraphVectorKeys(t *testing.T) {
	repo := t.TempDir()
	if keys, err := RepoGraphVectorKeys(repo); err != nil || keys != nil {
		t.Fatalf("missing: %v %v", keys, err)
	}
	cfgPath := filepath.Join(repo, ".toolbelt", "config.yaml")
	writeFile(t, cfgPath, "graph:\n  max_file_bytes: 1\n  vector:\n    enabled: true\n    chunk_lines: 3\n  embed:\n    model: /m\n    timeout: 1s\nvault:\n  path: x\n")
	keys, err := RepoGraphVectorKeys(repo)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"graph.embed.model", "graph.embed.timeout", "graph.vector.chunk_lines", "graph.vector.enabled"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}

	writeFile(t, cfgPath, "base: &b\n  model: /m\ngraph:\n  vector: true\n  embed:\n    <<: *b\n")
	keys, err = RepoGraphVectorKeys(repo)
	if err != nil || !reflect.DeepEqual(keys, []string{"graph.embed.model", "graph.vector"}) {
		t.Fatalf("scalar/merge: %v %v", keys, err)
	}

	writeFile(t, cfgPath, "graph:\n  languages: [go]\n")
	if keys, err := RepoGraphVectorKeys(repo); err != nil || len(keys) != 0 {
		t.Fatalf("none: %v %v", keys, err)
	}

	writeFile(t, cfgPath, "graph: [\n")
	if _, err := RepoGraphVectorKeys(repo); err == nil {
		t.Fatal("bad yaml must fail")
	}

	writeFile(t, cfgPath, "# "+strings.Repeat("x", maxRepoConfigBytes)+"\n")
	if _, err := RepoGraphVectorKeys(repo); err == nil {
		t.Fatal("oversized must fail")
	}

	target := filepath.Join(t.TempDir(), "real.yaml")
	writeFile(t, target, "graph:\n  vector:\n    enabled: true\n")
	if err := os.Remove(cfgPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, cfgPath); err != nil {
		t.Fatal(err)
	}
	if _, err := RepoGraphVectorKeys(repo); err == nil {
		t.Fatal("symlinked config must fail")
	}
}

func TestLoadUserMergeDefaultsAndValidation(t *testing.T) {
	global, _ := isolate(t)
	cfg, err := LoadUser("")
	if err != nil || cfg.Merge.Semantic.MaxRegionLines != 2000 || cfg.Merge.Prompt.MaxNovelLines != nil {
		t.Fatalf("defaults: %+v %v", cfg.Merge, err)
	}
	writeFile(t, global, "merge:\n  prompt:\n    max_novel_lines: 0\n    attempts: 3\n")
	cfg, err = LoadUser("")
	if err != nil || cfg.Merge.Prompt.MaxNovelLines == nil || *cfg.Merge.Prompt.MaxNovelLines != 0 || cfg.Merge.Prompt.Attempts != 3 || cfg.Merge.Prompt.MaxFiles != 10 {
		t.Fatalf("overlay: %+v %v", cfg.Merge, err)
	}
	writeFile(t, global, "merge:\n  prompt:\n    attempts: 0\n")
	if _, err := LoadUser(""); err == nil {
		t.Fatal("attempts 0 must be refused")
	}
	writeFile(t, global, "merge:\n  semantic:\n    max_region_lines: 10001\n")
	if _, err := LoadUser(""); err == nil {
		t.Fatal("max_region_lines above 10000 must be refused")
	}
	writeFile(t, global, "merge:\n  semantic:\n    max_region_lines: 10000\n")
	if _, err := LoadUser(""); err != nil {
		t.Fatal(err)
	}
}

func TestRepoMergeKeys(t *testing.T) {
	_, repo := isolate(t)
	if keys, err := RepoMergeKeys(repo); err != nil || keys != nil {
		t.Fatalf("absent: %v %v", keys, err)
	}
	writeFile(t, filepath.Join(repo, ".toolbelt", "config.yaml"), "merge:\n  semantic:\n    max_file_bytes: 9\n  prompt:\n    attempts: 9\n")
	keys, err := RepoMergeKeys(repo)
	if err != nil || !reflect.DeepEqual(keys, []string{"merge.prompt.attempts", "merge.semantic.max_file_bytes"}) {
		t.Fatalf("keys: %v %v", keys, err)
	}
}

func TestRepoMergeKeysIncludesEgmAgentKeys(t *testing.T) {
	_, repo := isolate(t)
	writeFile(t, filepath.Join(repo, ".toolbelt", "config.yaml"), "egm:\n  agent_cmd: [/bin/x]\n  max_rounds: 3\n")
	keys, err := RepoMergeKeys(repo)
	if err != nil || !reflect.DeepEqual(keys, []string{"egm.agent_cmd"}) {
		t.Fatalf("keys: %v %v", keys, err)
	}
}
