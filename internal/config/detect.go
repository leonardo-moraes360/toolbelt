package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	makeTargetRe = regexp.MustCompile(`(?m)^([a-zA-Z0-9][^:\n]*):`)
	lookPath     = exec.LookPath
	// cargoClippyWorks is a var so tests need no Rust toolchain.
	cargoClippyWorks = defaultCargoClippyWorks
)

// DetectedCmd is the test command chosen for a repo plus an optional warning.
type DetectedCmd struct {
	Cmd     string
	Warning string
}

var golangciConfigFiles = []string{
	".golangci.yml",
	".golangci.yaml",
	".golangci.toml",
	".golangci.json",
}

// DetectTestCmd returns "" when no toolchain is recognized.
func DetectTestCmd(root string) string {
	return DetectTestCmdDetailed(root).Cmd
}

// DetectTestCmdDetailed is like DetectTestCmd but also reports PATH fallbacks.
func DetectTestCmdDetailed(root string) DetectedCmd {
	if root == "" {
		return DetectedCmd{}
	}

	if fileExists(filepath.Join(root, "go.mod")) {
		return resolveCmd("go test ./...")
	}

	if fileExists(filepath.Join(root, "package.json")) {
		if cmd := detectNodeScriptCmd(root, "test"); cmd != "" {
			return resolveNodeCmd(cmd)
		}
	}

	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return resolveCmd("cargo test")
	}

	if d := detectPythonTestCmd(root); d.Cmd != "" {
		return d
	}

	if cmd := detectMakeTargetCmd(root, "test", "check"); cmd != "" {
		return resolveCmd(cmd)
	}

	return DetectedCmd{}
}

// DetectBuildCmd is conservative: it fills in only on a clear marker (go.mod,
// scripts.build, Cargo.toml, make build target), else returns "".
func DetectBuildCmd(root string) string {
	if root == "" {
		return ""
	}

	if fileExists(filepath.Join(root, "go.mod")) {
		return "go build ./..."
	}

	if fileExists(filepath.Join(root, "package.json")) {
		return detectNodeBuildCmd(root)
	}

	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return "cargo build"
	}

	if isPythonProject(root) {
		return ""
	}

	return detectMakeTargetCmd(root, "build")
}

// DetectLintCmd is conservative: golangci config / make lint, scripts.lint,
// cargo clippy, ruff in pyproject; else returns "". mypy is a typecheck
// (DetectTypecheckCmd), not a lint.
func DetectLintCmd(root string) string {
	if root == "" {
		return ""
	}

	if fileExists(filepath.Join(root, "go.mod")) {
		return detectGoLintCmd(root)
	}

	if fileExists(filepath.Join(root, "package.json")) {
		return detectNodeScriptCmd(root, "lint")
	}

	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return "cargo clippy"
	}

	if cmd := detectPythonLintCmd(root); cmd != "" {
		return cmd
	}

	return detectMakeTargetCmd(root, "lint")
}

// golangci-lint is written only when the binary is on PATH; with a golangci
// config but no binary it falls back to `go vet ./...` (the make lint target
// would almost always call the same missing golangci-lint).
func detectGoLintCmd(root string) string {
	for _, name := range golangciConfigFiles {
		if fileExists(filepath.Join(root, name)) {
			if _, err := lookPath("golangci-lint"); err == nil {
				return "golangci-lint run"
			}
			return "go vet ./..."
		}
	}
	return detectMakeTargetCmd(root, "lint")
}

// Only the build script is a build: typecheck scripts go to
// verify.typecheck_cmd (DetectTypecheckCmd).
func detectNodeBuildCmd(root string) string {
	return detectNodeScriptCmd(root, "build")
}

// DetectTypecheckCmd is conservative: a Node typecheck script, a tsconfig.json
// with a local tsc, or a configured mypy/pyright that resolves on PATH (or in
// .venv/bin); else "". Go and Rust get none because the build type-checks.
func DetectTypecheckCmd(root string) string {
	if root == "" {
		return ""
	}
	if fileExists(filepath.Join(root, "go.mod")) {
		return ""
	}
	if fileExists(filepath.Join(root, "package.json")) {
		return detectNodeTypecheckCmd(root)
	}
	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return ""
	}
	return detectPythonTypecheckCmd(root)
}

func detectNodeTypecheckCmd(root string) string {
	for _, script := range []string{"typecheck", "type-check"} {
		if cmd := detectNodeScriptCmd(root, script); cmd != "" {
			return cmd
		}
	}
	if !fileExists(filepath.Join(root, "tsconfig.json")) ||
		!fileExists(filepath.Join(root, "node_modules", ".bin", "tsc")) {
		return ""
	}
	return nodeTscCommand(nodePackageManager(root, readPackageManagerField(root)))
}

// nodeTscCommand runs the local tsc through the package manager: npm exec
// needs "--" so --noEmit reaches tsc, and yarn/bun have no exec subcommand
// with that meaning.
func nodeTscCommand(pm string) string {
	switch pm {
	case "pnpm":
		return "pnpm exec tsc --noEmit"
	case "yarn":
		return "yarn tsc --noEmit"
	case "bun":
		return "bunx tsc --noEmit"
	default:
		return "npm exec -- tsc --noEmit"
	}
}

func readPackageManagerField(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		PackageManager string `json:"packageManager"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return pkg.PackageManager
}

func detectPythonTypecheckCmd(root string) string {
	if !isPythonProject(root) &&
		!fileExists(filepath.Join(root, "mypy.ini")) &&
		!fileExists(filepath.Join(root, "pyrightconfig.json")) {
		return ""
	}
	pyproject, _ := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	content := string(pyproject)
	setupCfg, _ := os.ReadFile(filepath.Join(root, "setup.cfg"))
	useUV := fileExists(filepath.Join(root, "uv.lock"))

	mypyConfigured := hasPythonTool(content, "mypy") ||
		fileExists(filepath.Join(root, "mypy.ini")) ||
		fileExists(filepath.Join(root, ".mypy.ini")) ||
		strings.Contains(string(setupCfg), "[mypy]")
	if mypyConfigured && pythonToolResolves(root, "mypy") {
		return pythonToolCmd(useUV, "mypy .")
	}
	pyrightConfigured := hasPythonTool(content, "pyright") ||
		fileExists(filepath.Join(root, "pyrightconfig.json"))
	if pyrightConfigured && pythonToolResolves(root, "pyright") {
		return pythonToolCmd(useUV, "pyright")
	}
	return ""
}

func pythonToolResolves(root, tool string) bool {
	if _, err := lookPath(tool); err == nil {
		return true
	}
	return fileExists(filepath.Join(root, ".venv", "bin", tool))
}

// DetectSuggestedChecks returns the named checks tb init offers (R3):
// govulncheck for Go when on PATH, clippy for Rust when `cargo clippy -V`
// works. Never written unless the caller asks (tb init --write-checks).
func DetectSuggestedChecks(root string) []VerifyCheck {
	if root == "" {
		return nil
	}
	var out []VerifyCheck
	if fileExists(filepath.Join(root, "go.mod")) {
		if _, err := lookPath("govulncheck"); err == nil {
			out = append(out, VerifyCheck{Name: "govulncheck", Cmd: "govulncheck ./..."})
		}
	}
	if fileExists(filepath.Join(root, "Cargo.toml")) && cargoClippyWorks() {
		out = append(out, VerifyCheck{Name: "clippy", Cmd: "cargo clippy --all-targets -- -D warnings"})
	}
	return out
}

func defaultCargoClippyWorks() bool {
	cargo, err := lookPath("cargo")
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, cargo, "clippy", "-V").Run() == nil //nolint:gosec // fixed argv, cargo resolved via PATH
}

func detectNodeScriptCmd(root, script string) string {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return ""
	}

	var pkg struct {
		PackageManager string            `json:"packageManager"`
		Scripts        map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	if _, ok := pkg.Scripts[script]; !ok {
		return ""
	}

	pm := nodePackageManager(root, pkg.PackageManager)
	return nodeScriptCommand(pm, script)
}

func nodeScriptCommand(pm, script string) string {
	switch pm {
	case "pnpm":
		if script == "test" {
			return "pnpm test"
		}
		return "pnpm run " + script
	case "yarn":
		if script == "test" {
			return "yarn test"
		}
		return "yarn " + script
	case "bun":
		if script == "test" {
			return "bun test"
		}
		return "bun run " + script
	default:
		if script == "test" {
			return "npm test"
		}
		return "npm run " + script
	}
}

func resolveNodeCmd(preferred string) DetectedCmd {
	fallbacks := []string{preferred}
	switch strings.Fields(preferred)[0] {
	case "pnpm", "yarn", "bun":
		fallbacks = append(fallbacks, "npm test")
	}
	return resolveCmd(fallbacks...)
}

func nodePackageManager(root, packageManagerField string) string {
	if pm := packageManagerFromField(packageManagerField); pm != "" {
		return pm
	}
	switch {
	case fileExists(filepath.Join(root, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(root, "yarn.lock")):
		return "yarn"
	case fileExists(filepath.Join(root, "bun.lockb")), fileExists(filepath.Join(root, "bun.lock")):
		return "bun"
	case fileExists(filepath.Join(root, "package-lock.json")):
		return "npm"
	default:
		return "npm"
	}
}

func packageManagerFromField(field string) string {
	field = strings.TrimSpace(field)
	if field == "" {
		return ""
	}
	name, _, _ := strings.Cut(field, "@")
	switch strings.ToLower(name) {
	case "pnpm", "yarn", "npm", "bun":
		return strings.ToLower(name)
	default:
		return ""
	}
}

func detectPythonTestCmd(root string) DetectedCmd {
	if !isPythonProject(root) {
		return DetectedCmd{}
	}
	if fileExists(filepath.Join(root, "uv.lock")) {
		return resolveCmd(
			"uv run pytest",
			"pytest",
			"python -m pytest",
			"python3 -m pytest",
		)
	}
	return resolveCmd("pytest", "python -m pytest", "python3 -m pytest")
}

func resolveCmd(candidates ...string) DetectedCmd {
	var preferred string
	for _, cmd := range candidates {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		if preferred == "" {
			preferred = cmd
		}
		bin := commandBinary(cmd)
		if bin == "" {
			continue
		}
		if _, err := lookPath(bin); err == nil {
			if cmd == preferred {
				return DetectedCmd{Cmd: cmd}
			}
			return DetectedCmd{
				Cmd:     cmd,
				Warning: fmt.Sprintf("%s not found in PATH; using %s", commandBinary(preferred), cmd),
			}
		}
	}
	if preferred == "" {
		return DetectedCmd{}
	}
	return DetectedCmd{
		Cmd:     preferred,
		Warning: fmt.Sprintf("%s not found in PATH; verify.test_cmd %q may fail", commandBinary(preferred), preferred),
	}
}

func commandBinary(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func detectPythonLintCmd(root string) string {
	if !isPythonProject(root) {
		return ""
	}

	pyproject, _ := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	content := string(pyproject)
	useUV := fileExists(filepath.Join(root, "uv.lock"))

	if hasPythonTool(content, "ruff") ||
		fileExists(filepath.Join(root, "ruff.toml")) ||
		fileExists(filepath.Join(root, ".ruff.toml")) {
		return pythonToolCmd(useUV, "ruff check .")
	}
	return ""
}

func isPythonProject(root string) bool {
	return fileExists(filepath.Join(root, "pyproject.toml")) ||
		fileExists(filepath.Join(root, "pytest.ini")) ||
		fileExists(filepath.Join(root, "tox.ini")) ||
		fileExists(filepath.Join(root, "setup.cfg")) ||
		fileExists(filepath.Join(root, "uv.lock"))
}

func hasPythonTool(pyproject, tool string) bool {
	if pyproject == "" {
		return false
	}
	return strings.Contains(pyproject, "[tool."+tool+"]") ||
		strings.Contains(pyproject, "[tool."+tool+".")
}

func pythonToolCmd(useUV bool, cmd string) string {
	if useUV {
		return "uv run " + cmd
	}
	return cmd
}

func detectMakeTargetCmd(root string, names ...string) string {
	data, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		return ""
	}
	targets := makeTargets(string(data))
	for _, name := range names {
		if targets[name] {
			return "make " + name
		}
	}
	return ""
}

func makeTargets(content string) map[string]bool {
	out := make(map[string]bool)
	for _, match := range makeTargetRe.FindAllStringSubmatch(content, -1) {
		name := strings.Fields(match[1])[0]
		if strings.HasPrefix(name, ".") {
			continue
		}
		out[name] = true
	}
	return out
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func projectRootForInit(configDir string) (string, bool) {
	if filepath.Base(configDir) != DefaultConfigDir {
		return "", false
	}
	root := filepath.Dir(configDir)
	if root == "" || root == "." || root == string(filepath.Separator) {
		return "", false
	}
	return root, true
}
