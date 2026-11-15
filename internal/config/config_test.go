package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Defaults.Timeout != "30s" {
		t.Errorf("Default timeout = %q, want %q", cfg.Defaults.Timeout, "30s")
	}
	if cfg.Defaults.Sandbox != false {
		t.Error("Default sandbox should be false")
	}
	if cfg.Sandbox.CPULimit != 100 {
		t.Errorf("Default CPU limit = %d, want %d", cfg.Sandbox.CPULimit, 100)
	}
	if cfg.Bench.Repetitions != 5 {
		t.Errorf("Default repetitions = %d, want %d", cfg.Bench.Repetitions, 5)
	}
	if cfg.Verify.TestCmd != "go test ./..." {
		t.Errorf("Default verify test_cmd = %q", cfg.Verify.TestCmd)
	}
	if cfg.Verify.HiddenTestCmd != "" {
		t.Errorf("Default verify hidden_test_cmd = %q, want empty", cfg.Verify.HiddenTestCmd)
	}
	if len(cfg.Graph.Languages) < 12 {
		t.Errorf("expected default graph languages, got %v", cfg.Graph.Languages)
	}
	if len(cfg.Graph.Docs) != 3 {
		t.Errorf("expected markdown/html/txt docs, got %v", cfg.Graph.Docs)
	}
	if len(cfg.Graph.Frameworks) < 7 {
		t.Errorf("expected default frameworks, got %v", cfg.Graph.Frameworks)
	}
	if cfg.Graph.MaxFileBytes != 512*1024 {
		t.Errorf("unexpected max_file_bytes %d", cfg.Graph.MaxFileBytes)
	}
}

func TestVerifyConfigLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := DefaultConfig()
	cfg.Verify.TestCmd = "pnpm --filter @auditaja/core test"
	cfg.Verify.HiddenTestCmd = "pnpm --filter @auditaja/core test:hidden"
	if err := Save(cfg, configPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded := DefaultConfig()
	if err := loadFromFile(configPath, loaded); err != nil {
		t.Fatalf("loadFromFile failed: %v", err)
	}
	if loaded.Verify.TestCmd != cfg.Verify.TestCmd {
		t.Errorf("test_cmd = %q", loaded.Verify.TestCmd)
	}
	if loaded.Verify.HiddenTestCmd != cfg.Verify.HiddenTestCmd {
		t.Errorf("hidden_test_cmd = %q", loaded.Verify.HiddenTestCmd)
	}
}

func TestGraphConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	cfg := DefaultConfig()
	cfg.Graph.Languages = []string{"go", "python"}
	cfg.Graph.Docs = []string{"markdown"}
	cfg.Graph.Frameworks = []string{"react", "vue"}
	cfg.Graph.MaxFileBytes = 1024
	if err := Save(cfg, configPath); err != nil {
		t.Fatal(err)
	}
	loaded := DefaultConfig()
	if err := loadFromFile(configPath, loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Graph.Languages) != 2 || loaded.Graph.Languages[1] != "python" {
		t.Fatalf("languages=%v", loaded.Graph.Languages)
	}
	if len(loaded.Graph.Docs) != 1 || loaded.Graph.Docs[0] != "markdown" {
		t.Fatalf("docs=%v", loaded.Graph.Docs)
	}
	if len(loaded.Graph.Frameworks) != 2 || loaded.Graph.Frameworks[0] != "react" {
		t.Fatalf("frameworks=%v", loaded.Graph.Frameworks)
	}
	if loaded.Graph.MaxFileBytes != 1024 {
		t.Fatalf("max_file_bytes=%d", loaded.Graph.MaxFileBytes)
	}
}

func TestInit(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".toolbelt")

	err := Init(configDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verifica se o diretório foi criado
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Error(".toolbelt directory was not created")
	}

	// Verifica se o arquivo de configuração foi criado
	configFile := filepath.Join(configDir, DefaultConfigFile)
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("config.yaml was not created")
	}

	// Verifica subdiretórios
	subdirs := []string{"vault", "logs", "bench"}
	for _, sub := range subdirs {
		subpath := filepath.Join(configDir, sub)
		if _, err := os.Stat(subpath); os.IsNotExist(err) {
			t.Errorf("subdirectory %s was not created", sub)
		}
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".toolbelt")

	// Antes de Init
	if Exists(configDir) {
		t.Error("Exists should return false before Init")
	}

	// Depois de Init
	if err := Init(configDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if !Exists(configDir) {
		t.Error("Exists should return true after Init")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Cria configuração personalizada
	cfg := DefaultConfig()
	cfg.Defaults.Timeout = "60s"
	cfg.Bench.Repetitions = 10

	// Salva
	if err := Save(cfg, configPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Carrega
	loaded := DefaultConfig()
	if err := loadFromFile(configPath, loaded); err != nil {
		t.Fatalf("loadFromFile failed: %v", err)
	}

	if loaded.Defaults.Timeout != "60s" {
		t.Errorf("Loaded timeout = %q, want %q", loaded.Defaults.Timeout, "60s")
	}
	if loaded.Bench.Repetitions != 10 {
		t.Errorf("Loaded repetitions = %d, want %d", loaded.Bench.Repetitions, 10)
	}
}

func TestLocalOverridesGlobal(t *testing.T) {
	tmpDir := t.TempDir()

	// Cria configuração "global"
	globalDir := filepath.Join(tmpDir, "global")
	os.MkdirAll(globalDir, 0o755)
	globalCfg := DefaultConfig()
	globalCfg.Defaults.Timeout = "30s"
	globalCfg.Bench.Repetitions = 5
	Save(globalCfg, filepath.Join(globalDir, "config.yaml"))

	// Cria configuração "local"
	localDir := filepath.Join(tmpDir, "local")
	os.MkdirAll(localDir, 0o755)
	localCfg := DefaultConfig()
	localCfg.Defaults.Timeout = "60s" // Override
	Save(localCfg, filepath.Join(localDir, "config.yaml"))

	// Carrega global primeiro, depois local
	cfg := DefaultConfig()
	loadFromFile(filepath.Join(globalDir, "config.yaml"), cfg)
	loadFromFile(filepath.Join(localDir, "config.yaml"), cfg)

	// Local deve sobrepor global
	if cfg.Defaults.Timeout != "60s" {
		t.Errorf("Local should override global, got timeout = %q", cfg.Defaults.Timeout)
	}
}
