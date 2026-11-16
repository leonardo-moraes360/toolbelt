package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectTestCmd(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantCmd string
	}{
		{
			name: "go.mod",
			files: map[string]string{
				"go.mod": "module example.com/x\n",
			},
			wantCmd: "go test ./...",
		},
		{
			name: "go.mod wins over package.json and Makefile",
			files: map[string]string{
				"go.mod":       "module example.com/x\n",
				"package.json": `{"scripts":{"test":"vitest"}}`,
				"Makefile":     "test:\n\tgo test ./...\n",
			},
			wantCmd: "go test ./...",
		},
		{
			name: "npm via package.json scripts.test",
			files: map[string]string{
				"package.json": `{"scripts":{"test":"mocha"}}`,
			},
			wantCmd: "npm test",
		},
		{
			name: "pnpm via lockfile",
			files: map[string]string{
				"package.json":   `{"scripts":{"test":"vitest"}}`,
				"pnpm-lock.yaml": "lockfileVersion: '9.0'\n",
			},
			wantCmd: "pnpm test",
		},
		{
			name: "yarn via lockfile",
			files: map[string]string{
				"package.json": `{"scripts":{"test":"jest"}}`,
				"yarn.lock":    "# yarn lockfile v1\n",
			},
			wantCmd: "yarn test",
		},
		{
			name: "packageManager field pnpm",
			files: map[string]string{
				"package.json": `{"packageManager":"pnpm@9.0.0","scripts":{"test":"vitest"}}`,
			},
			wantCmd: "pnpm test",
		},
		{
			name: "package.json without test script ignored",
			files: map[string]string{
				"package.json": `{"scripts":{"lint":"eslint ."}}`,
			},
			wantCmd: "",
		},
		{
			name: "cargo",
			files: map[string]string{
				"Cargo.toml": "[package]\nname = \"x\"\n",
			},
			wantCmd: "cargo test",
		},
		{
			name: "pytest via pyproject",
			files: map[string]string{
				"pyproject.toml": "[project]\nname = \"x\"\n",
			},
			wantCmd: "pytest",
		},
		{
			name: "uv run pytest when uv.lock present",
			files: map[string]string{
				"pyproject.toml": "[project]\nname = \"x\"\n",
				"uv.lock":        "version = 1\n",
			},
			wantCmd: "uv run pytest",
		},
		{
			name: "pytest.ini alone",
			files: map[string]string{
				"pytest.ini": "[pytest]\n",
			},
			wantCmd: "pytest",
		},
		{
			name: "makefile test target",
			files: map[string]string{
				"Makefile": "all:\n\t@echo hi\ntest:\n\t./run-tests\n",
			},
			wantCmd: "make test",
		},
		{
			name: "makefile check when no test",
			files: map[string]string{
				"Makefile": "check:\n\t./run-tests\n",
			},
			wantCmd: "make check",
		},
		{
			name:    "empty root",
			files:   map[string]string{},
			wantCmd: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range tt.files {
				path := filepath.Join(root, name)
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatalf("write %s: %v", name, err)
				}
			}
			got := DetectTestCmd(root)
			if got != tt.wantCmd {
				t.Fatalf("DetectTestCmd() = %q, want %q", got, tt.wantCmd)
			}
		})
	}
}

func TestInitDetectsToolchain(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantCmd string
	}{
		{
			name: "npm fixture like express/commander-js",
			files: map[string]string{
				"package.json": `{"name":"express","scripts":{"test":"mocha --recursive"}}`,
			},
			wantCmd: "npm test",
		},
		{
			name: "python fixture like typer/click",
			files: map[string]string{
				"pyproject.toml": "[project]\nname = \"typer\"\n",
			},
			wantCmd: "pytest",
		},
		{
			name: "rust fixture like bat/ripgrep",
			files: map[string]string{
				"Cargo.toml": "[package]\nname = \"bat\"\nversion = \"0.1.0\"\n",
			},
			wantCmd: "cargo test",
		},
		{
			name: "c makefile like jq",
			files: map[string]string{
				"Makefile": "check:\n\t./tests/jqtest\n",
			},
			wantCmd: "make check",
		},
		{
			name: "go fixture unchanged",
			files: map[string]string{
				"go.mod": "module github.com/spf13/cobra\n",
			},
			wantCmd: "go test ./...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
					t.Fatalf("write %s: %v", name, err)
				}
			}
			configDir := filepath.Join(root, DefaultConfigDir)
			if err := Init(configDir); err != nil {
				t.Fatalf("Init: %v", err)
			}
			loaded := DefaultConfig()
			if err := loadFromFile(filepath.Join(configDir, DefaultConfigFile), loaded); err != nil {
				t.Fatalf("load: %v", err)
			}
			if loaded.Verify.TestCmd != tt.wantCmd {
				t.Fatalf("verify.test_cmd = %q, want %q", loaded.Verify.TestCmd, tt.wantCmd)
			}
		})
	}
}

func TestInitGlobalSkipsDetection(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "package.json"), []byte(`{"scripts":{"test":"mocha"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(home, ".config", "toolbelt")
	if err := Init(configDir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	loaded := DefaultConfig()
	if err := loadFromFile(filepath.Join(configDir, DefaultConfigFile), loaded); err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Verify.TestCmd != "go test ./..." {
		t.Fatalf("global init should keep go default, got %q", loaded.Verify.TestCmd)
	}
}
