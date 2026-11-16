package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	makeTargetRe = regexp.MustCompile(`(?m)^([a-zA-Z0-9][^:\n]*):`)
	lookPath     = exec.LookPath
)

// DetectedCmd is the test command chosen for a repo plus an optional warning.
type DetectedCmd struct {
	Cmd     string
	Warning string
}

// DetectTestCmd inspeciona a raiz do projeto e devolve o test_cmd adequado
// ao toolchain detectado. Retorna string vazia se nada for reconhecido.
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
		if cmd := detectNodeTestCmd(root); cmd != "" {
			return resolveNodeCmd(cmd)
		}
	}

	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return resolveCmd("cargo test")
	}

	if d := detectPythonTestCmd(root); d.Cmd != "" {
		return d
	}

	if cmd := detectMakeTestCmd(root); cmd != "" {
		return resolveCmd(cmd)
	}

	return DetectedCmd{}
}

func detectNodeTestCmd(root string) string {
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
	if _, ok := pkg.Scripts["test"]; !ok {
		return ""
	}

	pm := nodePackageManager(root, pkg.PackageManager)
	switch pm {
	case "pnpm":
		return "pnpm test"
	case "yarn":
		return "yarn test"
	case "bun":
		return "bun test"
	default:
		return "npm test"
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
	hasPyProject := fileExists(filepath.Join(root, "pyproject.toml"))
	hasPytestIni := fileExists(filepath.Join(root, "pytest.ini"))
	hasTox := fileExists(filepath.Join(root, "tox.ini"))
	hasSetupCfg := fileExists(filepath.Join(root, "setup.cfg"))
	hasUvLock := fileExists(filepath.Join(root, "uv.lock"))

	if !hasPyProject && !hasPytestIni && !hasTox && !hasSetupCfg && !hasUvLock {
		return DetectedCmd{}
	}

	if hasUvLock {
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
				Warning: fmt.Sprintf("%s não encontrado no PATH; usando %s", commandBinary(preferred), cmd),
			}
		}
	}
	if preferred == "" {
		return DetectedCmd{}
	}
	return DetectedCmd{
		Cmd:     preferred,
		Warning: fmt.Sprintf("%s não encontrado no PATH; verify.test_cmd %q pode falhar", commandBinary(preferred), preferred),
	}
}

func commandBinary(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func detectMakeTestCmd(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		return ""
	}
	targets := makeTargets(string(data))
	if targets["test"] {
		return "make test"
	}
	if targets["check"] {
		return "make check"
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
