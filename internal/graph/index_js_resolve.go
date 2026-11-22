package graph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type jsPathConfig struct {
	baseURL   string
	paths     map[string][]string
	workspace *jsWorkspace
}

func loadJSPathConfig(root string) *jsPathConfig {
	ws := loadJSWorkspace(root)
	for _, name := range []string{"tsconfig.json", "jsconfig.json"} {
		cfg, ok := readJSConfigFile(filepath.Join(root, name), root, map[string]bool{})
		if ok {
			cfg.workspace = ws
			return cfg
		}
	}
	return &jsPathConfig{paths: map[string][]string{}, workspace: ws}
}

type jsConfigFile struct {
	Extends         string `json:"extends"`
	CompilerOptions struct {
		BaseURL string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

func readJSConfigFile(path, root string, seen map[string]bool) (*jsPathConfig, bool) {
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, path)
	}
	abs = filepath.Clean(abs)
	if seen[abs] {
		return nil, false
	}
	seen[abs] = true
	data, err := readConfigBytes(root, abs)
	if err != nil {
		return nil, false
	}
	var raw jsConfigFile
	if err := json.Unmarshal(stripJSONC(data), &raw); err != nil {
		return nil, false
	}
	cfg := &jsPathConfig{
		baseURL: strings.TrimSpace(raw.CompilerOptions.BaseURL),
		paths:   map[string][]string{},
	}
	for k, v := range raw.CompilerOptions.Paths {
		cfg.paths[k] = append([]string{}, v...)
	}
	if raw.Extends != "" {
		parentPath := raw.Extends
		if !strings.HasSuffix(parentPath, ".json") {
			parentPath += ".json"
		}
		if !filepath.IsAbs(parentPath) {
			parentPath = filepath.Join(filepath.Dir(abs), parentPath)
		}
		if parent, ok := readJSConfigFile(parentPath, root, seen); ok {
			cfg = mergeJSPathConfig(parent, cfg)
		}
	}
	return cfg, true
}

func mergeJSPathConfig(parent, child *jsPathConfig) *jsPathConfig {
	out := &jsPathConfig{
		baseURL: parent.baseURL,
		paths:   map[string][]string{},
	}
	for k, v := range parent.paths {
		out.paths[k] = append([]string{}, v...)
	}
	if child.baseURL != "" {
		out.baseURL = child.baseURL
	}
	for k, v := range child.paths {
		out.paths[k] = append([]string{}, v...)
	}
	return out
}

func stripJSONC(data []byte) []byte {
	var b strings.Builder
	b.Grow(len(data))
	inStr := false
	esc := false
	i := 0
	for i < len(data) {
		if inStr {
			inStr, esc = stripJSONCInString(&b, data, i, esc)
			i++
			continue
		}
		next, skipped := stripJSONCOutside(data, i)
		if skipped {
			i = next
			continue
		}
		if data[i] == '"' {
			inStr = true
		}
		b.WriteByte(data[i])
		i++
	}
	return []byte(b.String())
}

func stripJSONCInString(b *strings.Builder, data []byte, i int, esc bool) (inStr, nextEsc bool) {
	c := data[i]
	b.WriteByte(c)
	switch {
	case esc:
		return true, false
	case c == '\\':
		return true, true
	case c == '"':
		return false, false
	default:
		return true, false
	}
}

func stripJSONCOutside(data []byte, i int) (next int, skipped bool) {
	if i+1 < len(data) && data[i] == '/' && data[i+1] == '/' {
		i += 2
		for i < len(data) && data[i] != '\n' {
			i++
		}
		return i, true
	}
	if i+1 < len(data) && data[i] == '/' && data[i+1] == '*' {
		i += 2
		for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
			i++
		}
		if i+1 < len(data) {
			i += 2
		}
		return i, true
	}
	if data[i] == ',' && jsoncTrailingComma(data, i+1) {
		return i + 1, true
	}
	return i, false
}

func jsoncTrailingComma(data []byte, j int) bool {
	for j < len(data) && (data[j] == ' ' || data[j] == '\t' || data[j] == '\n' || data[j] == '\r') {
		j++
	}
	return j < len(data) && (data[j] == '}' || data[j] == ']')
}

func (c *jsPathConfig) expandAlias(imp string) []string {
	if c == nil {
		return nil
	}
	var out []string
	for pattern, targets := range c.paths {
		star := strings.Index(pattern, "*")
		if star < 0 {
			if imp == pattern {
				out = append(out, targets...)
			}
			continue
		}
		prefix, suffix := pattern[:star], pattern[star+1:]
		if !strings.HasPrefix(imp, prefix) || !strings.HasSuffix(imp, suffix) {
			continue
		}
		captured := imp[len(prefix) : len(imp)-len(suffix)]
		for _, target := range targets {
			out = append(out, strings.Replace(target, "*", captured, 1))
		}
	}
	return out
}

func resolveJSImport(root, fromRel, imp string, cfg *jsPathConfig) (string, bool) {
	imp = strings.TrimSpace(imp)
	if imp == "" {
		return "", false
	}
	var candidates []string
	expanded := cfg.expandAlias(imp)
	switch {
	case strings.HasPrefix(imp, "."):
		candidates = append(candidates, filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(fromRel), imp))))
	case len(expanded) > 0:
		base := "."
		if cfg != nil && cfg.baseURL != "" {
			base = cfg.baseURL
		}
		for _, exp := range expanded {
			candidates = append(candidates, filepath.ToSlash(filepath.Clean(filepath.Join(base, exp))))
		}
	case isJSBarePackage(imp):
		if cfg == nil {
			return "", false
		}
		candidates = cfg.workspace.resolve(imp)
		if len(candidates) == 0 {
			return "", false
		}
	case cfg != nil && cfg.baseURL != "":
		candidates = append(candidates, filepath.ToSlash(filepath.Clean(filepath.Join(cfg.baseURL, imp))))
	default:
		return "", false
	}
	for _, cand := range candidates {
		if rel, ok := resolveJSExisting(root, cand); ok {
			return rel, true
		}
	}
	return "", false
}

func isJSBarePackage(imp string) bool {
	if strings.HasPrefix(imp, ".") || strings.HasPrefix(imp, "/") {
		return false
	}
	if strings.HasPrefix(imp, "@/") || strings.HasPrefix(imp, "#") {
		return false
	}
	return true
}

func resolveJSExisting(root, cand string) (string, bool) {
	cand = strings.TrimPrefix(filepath.ToSlash(cand), "./")
	for _, try := range jsModuleCandidates(cand) {
		full := filepath.Join(root, filepath.FromSlash(try))
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		return filepath.ToSlash(try), true
	}
	return "", false
}

func jsModuleCandidates(path string) []string {
	path = filepath.ToSlash(path)
	var out []string
	add := func(p string) {
		out = append(out, p)
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".jsx", ".mjs", ".cjs":
		base := path[:len(path)-len(ext)]
		add(base + ".ts")
		add(base + ".tsx")
		add(base + ".d.ts")
		add(path)
		add(base + ".jsx")
		add(base + ".js")
	case ".ts", ".tsx":
		add(path)
	case "":
		for _, e := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"} {
			add(path + e)
		}
		for _, e := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"} {
			add(filepath.ToSlash(filepath.Join(path, "index"+e)))
		}
	default:
		add(path)
	}
	if ext != "" {
		base := stripJSImportExt(path)
		for _, e := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"} {
			add(filepath.ToSlash(filepath.Join(base, "index"+e)))
		}
	}
	return uniqueStrings(out)
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// readConfigBytes reads a tsconfig/jsconfig through the safe reader when it
// lives under root. A config reached through `extends` outside root cannot go
// through the root-confined reader; it is read only if it is a regular file,
// so a FIFO there cannot block the walk.
func readConfigBytes(root, abs string) ([]byte, error) {
	if rel, err := filepath.Rel(root, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return readSourceBytes(root, rel)
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() || fi.Size() > maxIndexReadBytes {
		return nil, errNotRegular
	}
	return os.ReadFile(abs)
}
