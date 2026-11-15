package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var makeTargetRe = regexp.MustCompile(`(?m)^([a-zA-Z0-9][^:\n]*):`)

// DetectTestCmd inspeciona a raiz do projeto e devolve o test_cmd adequado
// ao toolchain detectado. Retorna string vazia se nada for reconhecido.
func DetectTestCmd(root string) string {
	if root == "" {
		return ""
	}

	if fileExists(filepath.Join(root, "go.mod")) {
		return "go test ./..."
	}

	if fileExists(filepath.Join(root, "package.json")) {
		if cmd := detectNodeTestCmd(root); cmd != "" {
			return cmd
		}
	}

	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return "cargo test"
	}

	if cmd := detectPythonTestCmd(root); cmd != "" {
		return cmd
	}

	if cmd := detectMakeTestCmd(root); cmd != "" {
		return cmd
	}

	return ""
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

func detectPythonTestCmd(root string) string {
	hasPyProject := fileExists(filepath.Join(root, "pyproject.toml"))
	hasPytestIni := fileExists(filepath.Join(root, "pytest.ini"))
	hasTox := fileExists(filepath.Join(root, "tox.ini"))
	hasSetupCfg := fileExists(filepath.Join(root, "setup.cfg"))
	hasUvLock := fileExists(filepath.Join(root, "uv.lock"))

	if !hasPyProject && !hasPytestIni && !hasTox && !hasSetupCfg && !hasUvLock {
		return ""
	}

	if hasUvLock {
		return "uv run pytest"
	}
	return "pytest"
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
