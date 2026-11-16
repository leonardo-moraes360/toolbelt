package guard

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type ViolationType string

const (
	ViolationTestDeletion  ViolationType = "test_deletion"
	ViolationTestSkip      ViolationType = "test_skip"
	ViolationTrivialAssert ViolationType = "trivial_assert"
	ViolationEnvDetection  ViolationType = "env_detection"
	ViolationProtectedPath ViolationType = "protected_path"
	ViolationEqOverride    ViolationType = "eq_override"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type ViolationSource string

const (
	SourceDefault ViolationSource = "default"
	SourceCustom  ViolationSource = "custom"
)

type Violation struct {
	Type     ViolationType   `json:"type"`
	File     string          `json:"file"`
	Line     int             `json:"line,omitempty"`
	Message  string          `json:"message"`
	Severity Severity        `json:"severity"`
	Context  string          `json:"context,omitempty"`
	Source   ViolationSource `json:"source,omitempty"`
}

type CheckResult struct {
	BaseRef    string      `json:"base_ref"`
	HeadRef    string      `json:"head_ref"`
	Violations []Violation `json:"violations"`
}

type protectedEntry struct {
	pattern string
	custom  bool
}

type Checker struct {
	repoDir        string
	protectedPaths []protectedEntry
}

var defaultProtectedPaths = []string{
	"tests/hidden/*",
	"benchmarks/*",
	".toolbelt/thresholds.yaml",
}

func NewChecker(repoDir string) *Checker {
	return &Checker{
		repoDir:        repoDir,
		protectedPaths: loadProtectedPaths(repoDir),
	}
}

func loadProtectedPaths(repoDir string) []protectedEntry {
	paths := make([]protectedEntry, 0, len(defaultProtectedPaths))
	seen := make(map[string]struct{}, len(defaultProtectedPaths))
	for _, p := range defaultProtectedPaths {
		seen[p] = struct{}{}
		paths = append(paths, protectedEntry{pattern: p})
	}
	data, err := os.ReadFile(filepath.Join(repoDir, ".toolbelt", "guard.yaml"))
	if err != nil {
		return paths
	}
	var raw struct {
		ProtectedPaths []any `yaml:"protected_paths"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return paths
	}
	for _, item := range raw.ProtectedPaths {
		pattern := protectedPattern(item)
		if pattern == "" {
			continue
		}
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		paths = append(paths, protectedEntry{pattern: pattern, custom: true})
	}
	return paths
}

func protectedPattern(item any) string {
	switch v := item.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]any:
		if p, ok := v["pattern"].(string); ok {
			return strings.TrimSpace(p)
		}
	}
	return ""
}

func (c *Checker) Check(baseRef, headRef string) (*CheckResult, error) {
	diff, err := c.gitDiff(baseRef, headRef)
	if err != nil {
		return nil, err
	}
	res := &CheckResult{BaseRef: baseRef, HeadRef: headRef}
	for _, cf := range parseDiff(diff) {
		if cf.deleted && isTestFile(cf.path) {
			res.Violations = append(res.Violations, Violation{
				Type: ViolationTestDeletion, File: cf.path, Message: "teste removido", Severity: SeverityError,
			})
		}
		if entry, ok := c.protectedMatch(cf.path); ok {
			src := SourceDefault
			if entry.custom {
				src = SourceCustom
			}
			res.Violations = append(res.Violations, Violation{
				Type: ViolationProtectedPath, File: cf.path, Message: "path protegido alterado",
				Severity: SeverityError, Source: src,
			})
		}
		if cf.deleted {
			continue
		}
		full := filepath.Join(c.repoDir, cf.path)
		if _, statErr := os.Stat(full); statErr != nil {
			continue
		}
		res.Violations = append(res.Violations, c.scanFile(full, cf.path)...)
	}
	return res, nil
}

type changed struct {
	path    string
	deleted bool
}

func (c *Checker) gitDiff(base, head string) (string, error) {
	cmd := exec.Command("git", "diff", "--name-status", base+".."+head) //nolint:gosec // refs validados pelo operador
	cmd.Dir = c.repoDir
	out, err := cmd.Output()
	return string(out), err
}

func parseDiff(diff string) []changed {
	var out []changed
	s := bufio.NewScanner(strings.NewReader(diff))
	for s.Scan() {
		line := s.Text()
		if len(line) < 2 {
			continue
		}
		ch := changed{path: strings.TrimSpace(line[1:])}
		if line[0] == 'D' {
			ch.deleted = true
		}
		out = append(out, ch)
	}
	return out
}

func isTestFile(path string) bool {
	b := filepath.Base(path)
	p := filepath.ToSlash(path)
	return strings.HasSuffix(b, "_test.go") || strings.Contains(p, "/tests/") || strings.Contains(p, "/test/")
}

func (c *Checker) protectedMatch(path string) (protectedEntry, bool) {
	for _, entry := range c.protectedPaths {
		if matchProtectedPattern(entry.pattern, path) {
			return entry, true
		}
	}
	return protectedEntry{}, false
}

func matchProtectedPattern(pattern, path string) bool {
	path = filepath.ToSlash(path)
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return false
	}
	if ok, _ := filepath.Match(pattern, path); ok {
		return true
	}
	if strings.HasSuffix(pattern, "/") {
		dir := strings.TrimSuffix(pattern, "/")
		return path == dir || strings.HasPrefix(path, pattern)
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	return false
}

func (c *Checker) scanFile(full, rel string) []Violation {
	switch filepath.Ext(full) {
	case ".go":
		return c.scanGo(full, rel)
	default:
		return nil
	}
}

func (c *Checker) scanGo(full, rel string) []Violation {
	var v []Violation
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, full, nil, 0)
	if err != nil {
		return v
	}
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "Skip" {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "t" {
				pos := fset.Position(call.Pos())
				v = append(v, Violation{Type: ViolationTestSkip, File: rel, Line: pos.Line, Message: "t.Skip()", Severity: SeverityWarning})
			}
		}
		return true
	})
	data, _ := os.ReadFile(full)
	v = append(v, scanEnv(string(data), rel)...)
	return v
}

var envPatterns = []*regexp.Regexp{
	regexp.MustCompile(`os\.Getenv\s*\(\s*["']CI["']\s*\)`),
	regexp.MustCompile(`testing\.Testing\s*\(\s*\)`),
}

func scanEnv(content, rel string) []Violation {
	var v []Violation
	for i, line := range strings.Split(content, "\n") {
		for _, re := range envPatterns {
			if re.MatchString(line) {
				v = append(v, Violation{
					Type: ViolationEnvDetection, File: rel, Line: i + 1,
					Message: "detecção de ambiente de teste", Severity: SeverityWarning, Context: strings.TrimSpace(line),
				})
			}
		}
	}
	return v
}
