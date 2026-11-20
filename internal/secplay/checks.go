package secplay

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// SecChecksFile is the project-local store of registered blue checks, written
// to .toolbelt/secchecks.yaml by `tb blue apply` and read by `tb blue check`.
const SecChecksFile = "secchecks.yaml"

// Severity of a check hit.
type Severity string

const (
	// SeverityError fails `tb blue check` (non-zero exit).
	SeverityError Severity = "error"
	// SeverityWarning is reported but does not fail.
	SeverityWarning Severity = "warning"
)

func (s Severity) norm() Severity {
	if s == SeverityError {
		return SeverityError
	}
	return SeverityWarning
}

// Check is a static, grep/regex detector declared by a blue playbook. It never
// runs a command; it only matches a regular expression against source lines.
type Check struct {
	ID           string   `yaml:"id" json:"id"`
	Class        string   `yaml:"class,omitempty" json:"class,omitempty"`
	Description  string   `yaml:"description" json:"description"`
	Pattern      string   `yaml:"pattern" json:"pattern"`
	IncludeGlobs []string `yaml:"include_globs,omitempty" json:"include_globs,omitempty"`
	Severity     Severity `yaml:"severity,omitempty" json:"severity,omitempty"`
}

func (c Check) compile() (*regexp.Regexp, error) {
	for _, g := range c.IncludeGlobs {
		if strings.Contains(strings.TrimPrefix(g, "**/"), "/") {
			return nil, fmt.Errorf("check %q: include_glob %q matches by base name only; a %q path separator never matches", c.ID, g, "/")
		}
	}
	re, err := regexp.Compile(c.Pattern)
	if err != nil {
		return nil, fmt.Errorf("check %q: invalid pattern: %w", c.ID, err)
	}
	return re, nil
}

// SecChecks is the on-disk secchecks.yaml document.
type SecChecks struct {
	Version int     `yaml:"version" json:"version"`
	Checks  []Check `yaml:"checks" json:"checks"`
}

// SecChecksPath returns the path to secchecks.yaml inside a toolbelt dir.
func SecChecksPath(toolbeltDir string) string {
	return filepath.Join(toolbeltDir, SecChecksFile)
}

// LoadSecChecks reads secchecks.yaml; a missing file yields an empty, version-1
// document (not an error).
func LoadSecChecks(path string) (*SecChecks, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &SecChecks{Version: 1}, nil
	}
	if err != nil {
		return nil, err
	}
	var sc SecChecks
	if err := yaml.Unmarshal(raw, &sc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if sc.Version == 0 {
		sc.Version = 1
	}
	return &sc, nil
}

// saveSecChecks writes sc to path, creating the directory. Checks are sorted by
// ID for a stable file.
func saveSecChecks(path string, sc *SecChecks) error {
	if sc.Version == 0 {
		sc.Version = 1
	}
	sort.SliceStable(sc.Checks, func(i, j int) bool { return sc.Checks[i].ID < sc.Checks[j].ID })
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(sc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ApplyResult reports what Apply did.
type ApplyResult struct {
	Class   string  `json:"class"`
	Added   []Check `json:"added"`
	Updated []Check `json:"updated"`
	DryRun  bool    `json:"dry_run"`
	Path    string  `json:"path"`
}

// Apply registers the blue playbook's checks for class into secchecks.yaml under
// toolbeltDir (load-mutate-save, deduped by check ID). With dryRun it computes
// the diff without writing.
func (c *Corpus) Apply(toolbeltDir, class string, dryRun bool) (*ApplyResult, error) {
	var pb *Playbook
	for _, name := range sortedNames(c.Blue) {
		b := c.Blue[name]
		if b.Class != class {
			continue
		}
		if pb != nil {
			return nil, fmt.Errorf("ambiguous class %q: matched by %q and %q", class, pb.Name, b.Name)
		}
		pb = b
	}
	if pb == nil {
		return nil, fmt.Errorf("no blue playbook for class %q", class)
	}
	if len(pb.Checks) == 0 {
		return nil, fmt.Errorf("blue playbook %q declares no checks", pb.Name)
	}
	for _, ch := range pb.Checks {
		if _, err := ch.compile(); err != nil {
			return nil, err
		}
	}

	path := SecChecksPath(toolbeltDir)
	sc, err := LoadSecChecks(path)
	if err != nil {
		return nil, err
	}
	idx := make(map[string]int, len(sc.Checks))
	for i, ch := range sc.Checks {
		idx[ch.ID] = i
	}
	res := &ApplyResult{Class: class, DryRun: dryRun, Path: path}
	for _, ch := range pb.Checks {
		ch.Severity = ch.Severity.norm()
		if ch.Class == "" {
			ch.Class = class
		}
		if i, ok := idx[ch.ID]; ok {
			if !checkEqual(sc.Checks[i], ch) {
				sc.Checks[i] = ch
				res.Updated = append(res.Updated, ch)
			}
		} else {
			sc.Checks = append(sc.Checks, ch)
			idx[ch.ID] = len(sc.Checks) - 1
			res.Added = append(res.Added, ch)
		}
	}
	if dryRun {
		return res, nil
	}
	if len(res.Added) == 0 && len(res.Updated) == 0 {
		return res, nil
	}
	if err := saveSecChecks(path, sc); err != nil {
		return nil, err
	}
	return res, nil
}

func checkEqual(a, b Check) bool {
	if a.ID != b.ID || a.Class != b.Class || a.Description != b.Description ||
		a.Pattern != b.Pattern || a.Severity.norm() != b.Severity.norm() ||
		len(a.IncludeGlobs) != len(b.IncludeGlobs) {
		return false
	}
	for i := range a.IncludeGlobs {
		if a.IncludeGlobs[i] != b.IncludeGlobs[i] {
			return false
		}
	}
	return true
}

// Violation is one check hit in the working tree.
type Violation struct {
	CheckID  string   `json:"check_id"`
	Class    string   `json:"class,omitempty"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Severity Severity `json:"severity"`
	Match    string   `json:"match"`
}

const maxScanBytes = 512 * 1024

var skipDirs = map[string]bool{
	".git": true, ".toolbelt": true, "node_modules": true, "vendor": true,
	".venv": true, "dist": true, "build": true,
}

// Run scans the working tree under repoDir against the registered checks and
// returns the violations. It only reads files and matches regexes — it never
// executes anything. Files are matched against each check's include_globs (by
// base name); a check with no globs applies to every text file.
func Run(repoDir, toolbeltDir string) ([]Violation, error) {
	sc, err := LoadSecChecks(SecChecksPath(toolbeltDir))
	if err != nil {
		return nil, err
	}
	if len(sc.Checks) == 0 {
		return nil, nil
	}
	type compiled struct {
		check Check
		re    *regexp.Regexp
	}
	cs := make([]compiled, 0, len(sc.Checks))
	for _, ch := range sc.Checks {
		re, err := ch.compile()
		if err != nil {
			return nil, err
		}
		cs = append(cs, compiled{ch, re})
	}

	var out []Violation
	err = filepath.WalkDir(repoDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// A single unreadable entry must not abort the whole scan; only a
			// failure to read the root itself is fatal.
			if path == repoDir {
				return err
			}
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			// Skip vendored/build dirs, but never treat the scan root as one
			// just because its own basename matches (e.g. running from ./build).
			if path != repoDir && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		// Only scan regular files. Symlinks, FIFOs, devices and sockets are
		// skipped before any open, so a planted link cannot make the scanner
		// read outside the tree or block on a device.
		if !d.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(repoDir, path)
		if relErr != nil {
			rel = path
		}
		base := d.Name()
		var applicable []compiled
		for _, cc := range cs {
			if globsMatch(cc.check.IncludeGlobs, base) {
				applicable = append(applicable, cc)
			}
		}
		if len(applicable) == 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > maxScanBytes {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if isBinary(raw) {
			return nil
		}
		lines := strings.Split(string(raw), "\n")
		for _, cc := range applicable {
			for i, ln := range lines {
				if cc.re.MatchString(ln) {
					out = append(out, Violation{
						CheckID:  cc.check.ID,
						Class:    cc.check.Class,
						File:     rel,
						Line:     i + 1,
						Severity: cc.check.Severity.norm(),
						Match:    strings.TrimSpace(ln),
					})
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].CheckID < out[j].CheckID
	})
	return out, nil
}

// globsMatch reports whether base matches any glob (by base name). An empty
// glob list matches every file.
func globsMatch(globs []string, base string) bool {
	if len(globs) == 0 {
		return true
	}
	for _, g := range globs {
		g = strings.TrimPrefix(g, "**/")
		if ok, _ := filepath.Match(g, base); ok {
			return true
		}
	}
	return false
}

func isBinary(b []byte) bool {
	n := len(b)
	if n > 8000 {
		n = 8000
	}
	for i := 0; i < n; i++ {
		if b[i] == 0 {
			return true
		}
	}
	return false
}

// HasError reports whether any violation is error severity.
func HasError(vs []Violation) bool {
	for _, v := range vs {
		if v.Severity.norm() == SeverityError {
			return true
		}
	}
	return false
}
