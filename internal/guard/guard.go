package guard

import (
	"bufio"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/gitref"
	"gopkg.in/yaml.v3"

	"github.com/leonardo-moraes360/toolbelt/internal/git"
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
	BaseRef      string      `json:"base_ref"`
	HeadRef      string      `json:"head_ref"`
	RulesChecked int         `json:"rules_checked"`
	Violations   []Violation `json:"violations"`
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
	".toolbelt/guard.yaml",
}

func NewChecker(repoDir string) *Checker {
	return &Checker{repoDir: repoDir}
}

func (c *Checker) Check(baseRef, headRef string) (*CheckResult, error) {
	return c.CheckDiff(baseRef, headRef, "")
}

func (c *Checker) CheckDiff(baseRef, headRef, nameStatus string) (*CheckResult, error) {
	base, head, err := gitref.ResolvePair(c.repoDir, baseRef, headRef)
	if err != nil {
		return nil, err
	}
	paths, err := c.loadProtectedPaths(base)
	if err != nil {
		return nil, err
	}
	c.protectedPaths = paths

	diff := nameStatus
	if diff == "" {
		diff, err = c.gitDiff(base, head)
		if err != nil {
			return nil, err
		}
	}
	res := &CheckResult{
		BaseRef:      base,
		HeadRef:      head,
		RulesChecked: len(c.protectedPaths),
		Violations:   []Violation{},
	}
	for _, cf := range parseDiff(diff) {
		if cf.testRemoved {
			res.Violations = append(res.Violations, Violation{
				Type: ViolationTestDeletion, File: cf.path, Message: "test removed", Severity: SeverityError,
			})
		}
		if entry, ok := c.protectedMatch(cf.path); ok {
			src := SourceDefault
			if entry.custom {
				src = SourceCustom
			}
			res.Violations = append(res.Violations, Violation{
				Type: ViolationProtectedPath, File: cf.path, Message: "protected path changed",
				Severity: SeverityError, Source: src,
			})
		}
		if cf.deleted {
			continue
		}
		content, showErr := c.gitShow(head, cf.path)
		if showErr != nil {
			continue
		}
		res.Violations = append(res.Violations, c.scanContent(content, cf.path)...)
	}
	vcv, err := c.checkVerifyConfig(base, head)
	if err != nil {
		return nil, err
	}
	res.Violations = append(res.Violations, vcv...)
	return res, nil
}

func (c *Checker) loadProtectedPaths(baseRef string) ([]protectedEntry, error) {
	paths := make([]protectedEntry, 0, len(defaultProtectedPaths))
	seen := make(map[string]struct{}, len(defaultProtectedPaths))
	for _, p := range defaultProtectedPaths {
		if !validProtectedPattern(p) {
			continue
		}
		seen[strings.ToLower(p)] = struct{}{}
		paths = append(paths, protectedEntry{pattern: p})
	}
	data, err := c.gitShow(baseRef, ".toolbelt/guard.yaml")
	if err != nil {
		if isMissingGitObject(err) {
			return paths, nil
		}
		return nil, fmt.Errorf("reading .toolbelt/guard.yaml from base %s: %w", baseRef, err)
	}
	var raw struct {
		ProtectedPaths []any `yaml:"protected_paths"`
	}
	if err := yaml.Unmarshal([]byte(data), &raw); err != nil {
		return nil, fmt.Errorf("guard.yaml malformed at base %s: %w", baseRef, err)
	}
	for _, item := range raw.ProtectedPaths {
		pattern := protectedPattern(item)
		if pattern == "" || !validProtectedPattern(pattern) {
			continue
		}
		key := strings.ToLower(pattern)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		paths = append(paths, protectedEntry{pattern: pattern, custom: true})
	}
	return paths, nil
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

func validProtectedPattern(pattern string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return false
	}
	_, err := compileGlob(pattern)
	return err == nil
}

// changed is one path of the diff. deleted paths are not content-scanned;
// testRemoved marks a test file that is gone (deleted, or renamed to a
// non-test path). A rename source is deleted but not testRemoved when the
// destination is still a test file.
type changed struct {
	path        string
	deleted     bool
	testRemoved bool
}

func (c *Checker) gitDiff(base, head string) (string, error) {
	out, err := git.Output(c.repoDir, "diff", "--name-status", "-z", "-M", "--end-of-options", base+".."+head)
	return string(out), err
}

func (c *Checker) gitShow(ref, path string) (string, error) {
	spec := ref + ":" + filepath.ToSlash(path)
	out, err := git.Output(c.repoDir, "show", spec)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func isMissingGitObject(err error) bool {
	msg := gitErrLower(err)
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "unknown revision") ||
		strings.Contains(msg, "invalid object name") ||
		strings.Contains(msg, "bad revision") ||
		strings.Contains(msg, "needed a single revision") ||
		strings.Contains(msg, "ambiguous argument") {
		return false
	}
	return strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "exists on disk") ||
		strings.Contains(msg, "bad object") ||
		strings.Contains(msg, "path not in")
}

func gitErrLower(err error) string {
	var ge *git.Error
	if errors.As(err, &ge) {
		return strings.ToLower(ge.Stderr)
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return strings.ToLower(string(ee.Stderr))
	}
	return ""
}

// parseDiff reads `git diff --name-status`: the NUL-separated form of -z
// (paths verbatim) or, when the input has no NUL, the tab-separated text form.
func parseDiff(diff string) []changed {
	var out []changed
	if strings.Contains(diff, "\x00") {
		f := strings.Split(strings.TrimRight(diff, "\x00"), "\x00")
		for i := 0; i < len(f); {
			status := f[i]
			n := 2
			if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
				n = 3
			}
			if i+n > len(f) {
				break
			}
			out = appendChange(out, status, f[i+1:i+n])
			i += n
		}
		return out
	}
	s := bufio.NewScanner(strings.NewReader(diff))
	for s.Scan() {
		parts := strings.Split(s.Text(), "\t")
		if len(parts) < 2 {
			continue
		}
		out = appendChange(out, parts[0], parts[1:])
	}
	return out
}

func appendChange(out []changed, status string, paths []string) []changed {
	switch {
	case strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C"):
		if len(paths) < 2 {
			return out
		}
		if strings.HasPrefix(status, "R") {
			out = append(out, changed{
				path: paths[0], deleted: true,
				testRemoved: isTestFile(paths[0]) && !equivalentTestMove(paths[0], paths[1]),
			})
		}
		return append(out, changed{path: paths[1]})
	case status == "D":
		return append(out, changed{path: paths[0], deleted: true, testRemoved: isTestFile(paths[0])})
	default:
		return append(out, changed{path: paths[0]})
	}
}

var jsFamilyExt = map[string]bool{".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".mjs": true, ".cjs": true}

// equivalentTestMove reports whether a renamed test file is still a test the
// toolchain will pick up at its destination: same language, still a test by
// the same kind of rule, and no path segment that tools skip.
func equivalentTestMove(src, dst string) bool {
	if !isTestFile(dst) {
		return false
	}
	if isTestBase(src) && !isTestBase(dst) {
		return false
	}
	se, de := strings.ToLower(filepath.Ext(src)), strings.ToLower(filepath.Ext(dst))
	if se != de && !(jsFamilyExt[se] && jsFamilyExt[de]) {
		return false
	}
	for _, seg := range strings.Split(filepath.ToSlash(dst), "/") {
		switch {
		case seg == "testdata" || seg == "vendor" || seg == "node_modules",
			strings.HasPrefix(seg, "_"), strings.HasPrefix(seg, "."):
			return false
		}
	}
	return true
}

func isTestBase(path string) bool {
	b := filepath.Base(path)
	return strings.HasSuffix(b, "_test.go") || isJSTestPath(path)
}

func isTestFile(path string) bool {
	b := filepath.Base(path)
	p := filepath.ToSlash(path)
	return strings.HasSuffix(b, "_test.go") ||
		strings.Contains("/"+p, "/tests/") ||
		strings.Contains("/"+p, "/test/") ||
		isJSTestPath(p)
}

func isJSTestPath(path string) bool {
	b := strings.ToLower(filepath.Base(path))
	return strings.Contains(b, ".test.") ||
		strings.Contains(b, ".spec.") ||
		strings.HasSuffix(b, "_test.js") ||
		strings.HasSuffix(b, "_test.ts") ||
		strings.HasSuffix(b, "_test.jsx") ||
		strings.HasSuffix(b, "_test.tsx")
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
	re, err := compileGlob(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(path)
}

func compileGlob(pattern string) (*regexp.Regexp, error) {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return nil, fmt.Errorf("empty pattern")
	}
	if strings.HasSuffix(pattern, "/") {
		return dirPrefixRegexp(strings.TrimSuffix(pattern, "/"))
	}
	if strings.HasSuffix(pattern, "/*") {
		return dirPrefixRegexp(strings.TrimSuffix(pattern, "/*"))
	}
	re, err := globToRegexp(pattern)
	if err != nil {
		return nil, err
	}
	return regexp.Compile(re)
}

func dirPrefixRegexp(prefix string) (*regexp.Regexp, error) {
	return regexp.Compile("(?i)^" + regexp.QuoteMeta(prefix) + "(/.*)?$")
}

func globToRegexp(pattern string) (string, error) {
	var b strings.Builder
	b.WriteString("(?i)^")
	for i := 0; i < len(pattern); {
		n, piece, err := nextGlobPiece(pattern, i)
		if err != nil {
			return "", err
		}
		b.WriteString(piece)
		i = n
	}
	b.WriteString("$")
	return b.String(), nil
}

func nextGlobPiece(pattern string, i int) (next int, piece string, err error) {
	if i+1 < len(pattern) && pattern[i] == '*' && pattern[i+1] == '*' {
		if i+2 < len(pattern) && pattern[i+2] == '/' {
			return i + 3, "(.*/)?", nil
		}
		return i + 2, ".*", nil
	}
	switch pattern[i] {
	case '*':
		return i + 1, "[^/]*", nil
	case '?':
		return i + 1, "[^/]", nil
	case '[':
		j := i + 1
		if j < len(pattern) && (pattern[j] == '!' || pattern[j] == '^') {
			j++
		}
		for j < len(pattern) && pattern[j] != ']' {
			j++
		}
		if j >= len(pattern) {
			return 0, "", fmt.Errorf("unterminated character class")
		}
		return j + 1, pattern[i : j+1], nil
	default:
		return i + 1, regexp.QuoteMeta(string(pattern[i])), nil
	}
}

func (c *Checker) scanContent(content, rel string) []Violation {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".go":
		return c.scanGo(content, rel)
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
		return scanJS(content, rel)
	default:
		return nil
	}
}

func (c *Checker) scanGo(content, rel string) []Violation {
	var v []Violation
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, rel, content, 0)
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
	v = append(v, scanEnv(content, rel)...)
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
					Message: "test environment detection", Severity: SeverityWarning, Context: strings.TrimSpace(line),
				})
			}
		}
	}
	return v
}

var (
	jsSkipRes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(describe|it|test)\s*\.\s*skip\b`),
		regexp.MustCompile(`(?i)\bxit\b`),
		regexp.MustCompile(`(?i)\b(describe|it|test)\s*\.\s*only\b`),
	}
	jsTrivialRes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)expect\s*\(\s*true\s*\)\s*\.\s*toBe\s*\(\s*true\s*\)`),
		regexp.MustCompile(`(?i)expect\s*\(\s*false\s*\)\s*\.\s*toBe\s*\(\s*false\s*\)`),
		regexp.MustCompile(`(?i)expect\s*\(\s*true\s*\)\s*\.\s*toBeTruthy\s*\(`),
		regexp.MustCompile(`(?i)expect\s*\(\s*false\s*\)\s*\.\s*toBeFalsy\s*\(`),
		regexp.MustCompile(`(?i)expect\s*\(\s*(\d+)\s*\)\s*\.\s*toBe\s*\(\s*(\d+)\s*\)`),
		regexp.MustCompile(`(?i)expect\s*\(\s*(['"].*?['"])\s*\)\s*\.\s*toBe\s*\(\s*(['"].*?['"])\s*\)`),
	}
)

func scanJS(content, rel string) []Violation {
	var v []Violation
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, re := range jsSkipRes {
			if re.MatchString(line) {
				v = append(v, Violation{
					Type: ViolationTestSkip, File: rel, Line: i + 1,
					Message: "weakened JS/TS test (skip/only)", Severity: SeverityWarning,
					Context: trimmed,
				})
				break
			}
		}
		if trivialJSAssert(line) {
			v = append(v, Violation{
				Type: ViolationTrivialAssert, File: rel, Line: i + 1,
				Message: "trivial JS/TS assertion", Severity: SeverityError,
				Context: trimmed,
			})
		}
	}
	return v
}

func trivialJSAssert(line string) bool {
	for _, re := range jsTrivialRes {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if len(m) >= 3 && m[1] != "" && m[2] != "" && m[1] != m[2] {
			continue
		}
		return true
	}
	return false
}
