package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withLookPath(t *testing.T, available map[string]bool) {
	t.Helper()
	orig := lookPath
	lookPath = func(file string) (string, error) {
		if available[file] {
			return "/fake/bin/" + file, nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() { lookPath = orig })
}

func TestDetectTestCmd(t *testing.T) {
	withLookPath(t, map[string]bool{
		"go": true, "npm": true, "pnpm": true, "yarn": true, "bun": true,
		"cargo": true, "uv": true, "pytest": true, "make": true,
		"python": true, "python3": true,
	})

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

func TestDetectTestCmd_UvMissingFallsBack(t *testing.T) {
	withLookPath(t, map[string]bool{"pytest": true, "python": true, "python3": true})

	root := t.TempDir()
	for name, content := range map[string]string{
		"pyproject.toml": "[project]\nname = \"typer\"\n",
		"uv.lock":        "version = 1\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := DetectTestCmdDetailed(root)
	if got.Cmd != "pytest" {
		t.Fatalf("cmd = %q, want pytest fallback", got.Cmd)
	}
	if !strings.Contains(got.Warning, "uv") {
		t.Fatalf("expected uv PATH warning, got %q", got.Warning)
	}
}

func TestDetectTestCmd_UvMissingFallsBackToPythonModule(t *testing.T) {
	withLookPath(t, map[string]bool{"python3": true})

	root := t.TempDir()
	for name, content := range map[string]string{
		"pyproject.toml": "[project]\nname = \"typer\"\n",
		"uv.lock":        "version = 1\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := DetectTestCmdDetailed(root)
	if got.Cmd != "python3 -m pytest" {
		t.Fatalf("cmd = %q, want python3 -m pytest", got.Cmd)
	}
	if got.Warning == "" {
		t.Fatal("expected warning when falling back from uv")
	}
}

func TestDetectTestCmd_PnpmMissingFallsBackToNpm(t *testing.T) {
	withLookPath(t, map[string]bool{"npm": true})

	root := t.TempDir()
	for name, content := range map[string]string{
		"package.json":   `{"scripts":{"test":"vitest"}}`,
		"pnpm-lock.yaml": "lockfileVersion: '9.0'\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := DetectTestCmdDetailed(root)
	if got.Cmd != "npm test" {
		t.Fatalf("cmd = %q, want npm test", got.Cmd)
	}
	if !strings.Contains(got.Warning, "pnpm") {
		t.Fatalf("expected pnpm warning, got %q", got.Warning)
	}
}

func TestDetectBuildLintCmd(t *testing.T) {
	withLookPath(t, map[string]bool{"golangci-lint": true})
	tests := []struct {
		name      string
		files     map[string]string
		wantBuild string
		wantLint  string
	}{
		{
			name: "go with golangci config",
			files: map[string]string{
				"go.mod":        "module example.com/x\n",
				".golangci.yml": "run:\n  timeout: 5m\n",
			},
			wantBuild: "go build ./...",
			wantLint:  "golangci-lint run",
		},
		{
			name: "go with Makefile lint fallback",
			files: map[string]string{
				"go.mod":   "module example.com/x\n",
				"Makefile": "lint:\n\tgolangci-lint run ./...\n",
			},
			wantBuild: "go build ./...",
			wantLint:  "make lint",
		},
		{
			name: "go without lint markers",
			files: map[string]string{
				"go.mod": "module example.com/x\n",
			},
			wantBuild: "go build ./...",
			wantLint:  "",
		},
		{
			name: "npm build and lint scripts like express",
			files: map[string]string{
				"package.json": `{"scripts":{"test":"mocha","lint":"eslint .","build":"node scripts/build.js"}}`,
			},
			wantBuild: "npm run build",
			wantLint:  "npm run lint",
		},
		{
			name: "npm typecheck when no build script",
			files: map[string]string{
				"package.json":      `{"scripts":{"typecheck":"tsc --noEmit","lint":"eslint ."}}`,
				"package-lock.json": "{}\n",
			},
			wantBuild: "npm run typecheck",
			wantLint:  "npm run lint",
		},
		{
			name: "npm type-check when no build or typecheck",
			files: map[string]string{
				"package.json": `{"scripts":{"type-check":"tsc","lint":"next lint"}}`,
			},
			wantBuild: "npm run type-check",
			wantLint:  "npm run lint",
		},
		{
			name: "build preferred over typecheck",
			files: map[string]string{
				"package.json": `{"scripts":{"build":"next build","typecheck":"tsc --noEmit"}}`,
			},
			wantBuild: "npm run build",
			wantLint:  "",
		},
		{
			name: "npm test only no build/lint scripts",
			files: map[string]string{
				"package.json": `{"scripts":{"test":"mocha"}}`,
			},
			wantBuild: "",
			wantLint:  "",
		},
		{
			name: "pnpm via lockfile",
			files: map[string]string{
				"package.json":   `{"scripts":{"build":"tsc","lint":"eslint .","test":"vitest"}}`,
				"pnpm-lock.yaml": "lockfileVersion: '9.0'\n",
			},
			wantBuild: "pnpm run build",
			wantLint:  "pnpm run lint",
		},
		{
			name: "yarn via lockfile",
			files: map[string]string{
				"package.json": `{"scripts":{"build":"tsc","lint":"eslint .","test":"jest"}}`,
				"yarn.lock":    "# yarn lockfile v1\n",
			},
			wantBuild: "yarn build",
			wantLint:  "yarn lint",
		},
		{
			name: "cargo build and clippy",
			files: map[string]string{
				"Cargo.toml": "[package]\nname = \"bat\"\n",
			},
			wantBuild: "cargo build",
			wantLint:  "cargo clippy",
		},
		{
			name: "python ruff via pyproject",
			files: map[string]string{
				"pyproject.toml": "[project]\nname = \"x\"\n[tool.ruff]\nline-length = 88\n",
			},
			wantBuild: "",
			wantLint:  "ruff check .",
		},
		{
			name: "python ruff with uv.lock",
			files: map[string]string{
				"pyproject.toml": "[project]\nname = \"x\"\n[tool.ruff.lint]\nselect = [\"E\"]\n",
				"uv.lock":        "version = 1\n",
			},
			wantBuild: "",
			wantLint:  "uv run ruff check .",
		},
		{
			name: "python mypy when no ruff",
			files: map[string]string{
				"pyproject.toml": "[project]\nname = \"x\"\n[tool.mypy]\nstrict = true\n",
			},
			wantBuild: "",
			wantLint:  "mypy .",
		},
		{
			name: "python ruff.toml without tool section",
			files: map[string]string{
				"pyproject.toml": "[project]\nname = \"x\"\n",
				"ruff.toml":      "line-length = 100\n",
			},
			wantBuild: "",
			wantLint:  "ruff check .",
		},
		{
			name: "makefile build and lint",
			files: map[string]string{
				"Makefile": "build:\n\t$(CC) -o jq\nlint:\n\tshellcheck *.sh\n",
			},
			wantBuild: "make build",
			wantLint:  "make lint",
		},
		{
			name: "go wins over package.json scripts",
			files: map[string]string{
				"go.mod":         "module example.com/x\n",
				".golangci.yaml": "run: {}\n",
				"package.json":   `{"scripts":{"build":"webpack","lint":"eslint ."}}`,
			},
			wantBuild: "go build ./...",
			wantLint:  "golangci-lint run",
		},
		{
			name:      "empty root",
			files:     map[string]string{},
			wantBuild: "",
			wantLint:  "",
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
			if got := DetectBuildCmd(root); got != tt.wantBuild {
				t.Fatalf("DetectBuildCmd() = %q, want %q", got, tt.wantBuild)
			}
			if got := DetectLintCmd(root); got != tt.wantLint {
				t.Fatalf("DetectLintCmd() = %q, want %q", got, tt.wantLint)
			}
		})
	}
}

func TestInitDetectsToolchain(t *testing.T) {
	withLookPath(t, map[string]bool{
		"go": true, "npm": true, "pnpm": true, "yarn": true, "bun": true,
		"pytest": true, "cargo": true, "make": true, "uv": true,
		"python": true, "python3": true, "golangci-lint": true,
	})

	tests := []struct {
		name      string
		files     map[string]string
		wantTest  string
		wantBuild string
		wantLint  string
	}{
		{
			name: "npm fixture like express/commander-js",
			files: map[string]string{
				"package.json": `{"name":"express","scripts":{"test":"mocha --recursive","lint":"eslint ."}}`,
			},
			wantTest:  "npm test",
			wantBuild: "",
			wantLint:  "npm run lint",
		},
		{
			name: "npm with build like ky/excalidraw",
			files: map[string]string{
				"package.json": `{"scripts":{"test":"ava","build":"tsc","lint":"xo"}}`,
			},
			wantTest:  "npm test",
			wantBuild: "npm run build",
			wantLint:  "npm run lint",
		},
		{
			name: "next with-typescript: package.json without test must not keep go test",
			files: map[string]string{
				"package.json": `{
  "private": true,
  "scripts": {
    "dev": "next",
    "build": "next build",
    "start": "next start",
    "type-check": "tsc"
  }
}`,
			},
			wantTest:  "",
			wantBuild: "npm run build",
			wantLint:  "",
		},
		{
			name: "node typecheck+lint without test clears go default",
			files: map[string]string{
				"package.json":      `{"scripts":{"typecheck":"tsc --noEmit","lint":"eslint ."}}`,
				"package-lock.json": "{}\n",
			},
			wantTest:  "",
			wantBuild: "npm run typecheck",
			wantLint:  "npm run lint",
		},
		{
			name: "empty repo without markers clears go default",
			files: map[string]string{
				"README.md": "# hello\n",
			},
			wantTest:  "",
			wantBuild: "",
			wantLint:  "",
		},
		{
			name: "python fixture like typer/click",
			files: map[string]string{
				"pyproject.toml": "[project]\nname = \"typer\"\n",
			},
			wantTest:  "pytest",
			wantBuild: "",
			wantLint:  "",
		},
		{
			name: "python ruff like sentry-python",
			files: map[string]string{
				"pyproject.toml": "[project]\nname = \"sentry\"\n[tool.ruff]\nline-length = 100\n",
				"uv.lock":        "version = 1\n",
			},
			wantTest:  "uv run pytest",
			wantBuild: "",
			wantLint:  "uv run ruff check .",
		},
		{
			name: "rust fixture like bat/ripgrep",
			files: map[string]string{
				"Cargo.toml": "[package]\nname = \"bat\"\nversion = \"0.1.0\"\n",
			},
			wantTest:  "cargo test",
			wantBuild: "cargo build",
			wantLint:  "cargo clippy",
		},
		{
			name: "c makefile like jq",
			files: map[string]string{
				"Makefile": "check:\n\t./tests/jqtest\n",
			},
			wantTest:  "make check",
			wantBuild: "",
			wantLint:  "",
		},
		{
			name: "go with golangci like cobra/toolbelt",
			files: map[string]string{
				"go.mod":        "module github.com/spf13/cobra\n",
				".golangci.yml": "linters:\n  enable:\n    - govet\n",
			},
			wantTest:  "go test ./...",
			wantBuild: "go build ./...",
			wantLint:  "golangci-lint run",
		},
		{
			name: "go fixture without lint config",
			files: map[string]string{
				"go.mod": "module github.com/spf13/cobra\n",
			},
			wantTest:  "go test ./...",
			wantBuild: "go build ./...",
			wantLint:  "",
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
			if loaded.Verify.TestCmd != tt.wantTest {
				t.Fatalf("verify.test_cmd = %q, want %q", loaded.Verify.TestCmd, tt.wantTest)
			}
			if loaded.Verify.BuildCmd != tt.wantBuild {
				t.Fatalf("verify.build_cmd = %q, want %q", loaded.Verify.BuildCmd, tt.wantBuild)
			}
			if loaded.Verify.LintCmd != tt.wantLint {
				t.Fatalf("verify.lint_cmd = %q, want %q", loaded.Verify.LintCmd, tt.wantLint)
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

func TestInitDetailed_WarnsWhenNoTestToolchain(t *testing.T) {
	withLookPath(t, map[string]bool{"npm": true})

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"build":"next build"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	detected, err := InitDetailed(filepath.Join(root, DefaultConfigDir))
	if err != nil {
		t.Fatalf("InitDetailed: %v", err)
	}
	if detected.Cmd != "" {
		t.Fatalf("cmd = %q, want empty", detected.Cmd)
	}
	if detected.Warning == "" {
		t.Fatal("expected warning when no test toolchain is detectable")
	}
	if !strings.Contains(detected.Warning, "empty") {
		t.Fatalf("warning should explain empty test_cmd, got %q", detected.Warning)
	}
	loaded := DefaultConfig()
	if err := loadFromFile(filepath.Join(root, DefaultConfigDir, DefaultConfigFile), loaded); err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Verify.TestCmd != "" {
		t.Fatalf("test_cmd = %q, want empty (never fall back to go without go.mod)", loaded.Verify.TestCmd)
	}
	if loaded.Verify.BuildCmd != "npm run build" {
		t.Fatalf("build_cmd = %q, want npm run build", loaded.Verify.BuildCmd)
	}
}

// Dogfood #1: with a golangci config but no binary on PATH, init does not
// write `golangci-lint run` (tb verify would fail); it falls back to go vet.
func TestDetectLintCmd_GolangciMissingFromPath(t *testing.T) {
	withLookPath(t, map[string]bool{"go": true, "make": true})
	root := t.TempDir()
	files := map[string]string{
		"go.mod":        "module example.com/x\n",
		".golangci.yml": "run: {}\n",
		"Makefile":      "lint:\n\tgolangci-lint run\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := DetectLintCmd(root); got != "go vet ./..." {
		t.Fatalf("DetectLintCmd() = %q, want %q", got, "go vet ./...")
	}
}

// Dogfood #7: config.yaml is versioned, so it does not get mode 0600.
func TestSaveUsesVersionedFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultConfigFile)
	if err := Save(DefaultConfig(), path); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o044 == 0 {
		t.Fatalf("config.yaml com modo %v; esperado legível pelo grupo/outros (umask permitindo)", st.Mode().Perm())
	}
}
