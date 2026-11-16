package review

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/mem"
	"gopkg.in/yaml.v3"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

type Finding struct {
	ID        string   `json:"id"`
	Severity  Severity `json:"severity"`
	Category  string   `json:"category"`
	Message   string   `json:"message"`
	File      string   `json:"file,omitempty"`
	Reviewer  string   `json:"reviewer"`
	Agreement int      `json:"agreement"`
}

type TicketScope struct {
	Title       string   `yaml:"title" json:"title"`
	BaseCommit  string   `yaml:"base_commit" json:"base_commit"`
	Scope       []string `yaml:"scope" json:"scope"`
	Forbidden   []string `yaml:"forbidden" json:"forbidden"`
	Acceptance  []string `yaml:"acceptance" json:"acceptance"`
	Description string   `yaml:"description" json:"description"`
}

type Config struct {
	RepoDir    string
	BaseRef    string
	HeadRef    string
	TicketPath string
	Reviewers  int
	Standards  []string
}

type Report struct {
	Target    string       `json:"target"`
	BaseRef   string       `json:"base_ref"`
	HeadRef   string       `json:"head_ref"`
	Findings  []Finding    `json:"findings"`
	Drift     *DriftReport `json:"drift,omitempty"`
	FreshOK   bool         `json:"fresh_context"`
	Reviewers int          `json:"reviewers"`
}

type DriftReport struct {
	OutOfScope   []string `json:"out_of_scope"`
	ForbiddenHit []string `json:"forbidden_hit"`
	BaseMismatch bool     `json:"base_mismatch"`
	DeclaredBase string   `json:"declared_base,omitempty"`
	ActualBase   string   `json:"actual_base,omitempty"`
}

func DefaultConfig(repo string) Config {
	return Config{
		RepoDir:    repo,
		BaseRef:    "HEAD~1",
		HeadRef:    "HEAD",
		TicketPath: "rd.md",
		Reviewers:  3,
		Standards:  []string{"scope", "invariants", "completeness"},
	}
}

// Run executa revisão fresh-context (Elephant-Goldfish) sem herdar estado do elephant.
func Run(cfg *Config) (*Report, error) {
	if cfg.Reviewers <= 0 {
		cfg.Reviewers = 3
	}
	ticket, err := LoadTicket(cfg.TicketPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if ticket != nil && ticket.BaseCommit != "" && cfg.BaseRef == "HEAD~1" {
		cfg.BaseRef = ticket.BaseCommit
	}

	changed, err := listChangedFiles(cfg.RepoDir, cfg.BaseRef, cfg.HeadRef)
	if err != nil {
		return nil, err
	}
	diff, err := gitDiff(cfg.RepoDir, cfg.BaseRef, cfg.HeadRef)
	if err != nil {
		return nil, err
	}

	invariants, _ := loadInvariants(cfg.RepoDir)

	report := &Report{
		Target:    cfg.TicketPath,
		BaseRef:   cfg.BaseRef,
		HeadRef:   cfg.HeadRef,
		FreshOK:   true,
		Reviewers: cfg.Reviewers,
	}

	drift := DetectDrift(ticket, changed, cfg.BaseRef)
	report.Drift = drift

	// Cada goldfish recebe apenas ticket + diff + invariantes (contexto fresco isolado).
	var all []Finding
	for i := 1; i <= cfg.Reviewers; i++ {
		name := fmt.Sprintf("goldfish-%d", i)
		ctx := FreshContext{
			Reviewer:   name,
			Ticket:     ticket,
			Diff:       diff,
			Changed:    changed,
			Invariants: invariants,
			Standards:  cfg.Standards,
		}
		all = append(all, Fresh(&ctx)...)
	}
	report.Findings = aggregateFindings(all)
	return report, nil
}

type FreshContext struct {
	Reviewer   string
	Ticket     *TicketScope
	Diff       string
	Changed    []string
	Invariants []mem.Invariant
	Standards  []string
}

func Fresh(ctx *FreshContext) []Finding {
	var out []Finding
	standards := map[string]bool{}
	for _, s := range ctx.Standards {
		standards[s] = true
	}
	if standards["scope"] || len(standards) == 0 {
		out = append(out, reviewScope(ctx)...)
	}
	if standards["invariants"] || len(standards) == 0 {
		out = append(out, reviewInvariants(ctx)...)
	}
	if standards["completeness"] || len(standards) == 0 {
		out = append(out, reviewCompleteness(ctx)...)
	}
	return out
}

func reviewScope(ctx *FreshContext) []Finding {
	if ctx.Ticket == nil || len(ctx.Ticket.Scope) == 0 {
		return []Finding{{
			ID: "scope-missing", Severity: SeverityWarning, Category: "scope",
			Message: "ticket/rd.md sem scope declarado", Reviewer: ctx.Reviewer, Agreement: 1,
		}}
	}
	var out []Finding
	for _, f := range ctx.Changed {
		if !inScope(f, ctx.Ticket.Scope) {
			out = append(out, Finding{
				ID: "drift-" + f, Severity: SeverityError, Category: "scope",
				Message: "mudança fora do scope declarado", File: f,
				Reviewer: ctx.Reviewer, Agreement: 1,
			})
		}
		if hitForbidden(f, ctx.Ticket.Forbidden) {
			out = append(out, Finding{
				ID: "forbidden-" + f, Severity: SeverityCritical, Category: "scope",
				Message: "arquivo proibido pelo mandate foi alterado", File: f,
				Reviewer: ctx.Reviewer, Agreement: 1,
			})
		}
	}
	return out
}

func reviewInvariants(ctx *FreshContext) []Finding {
	var out []Finding
	lowerDiff := strings.ToLower(ctx.Diff)
	for _, inv := range ctx.Invariants {
		rule := strings.ToLower(inv.Rule)
		if strings.Contains(rule, "nunca") || strings.Contains(rule, "never") {
			if suggestsViolation(rule, lowerDiff, ctx.Changed) {
				out = append(out, Finding{
					ID: "inv-" + inv.ID, Severity: SeverityCritical, Category: "invariants",
					Message:  "possível violação de invariante: " + inv.Rule,
					Reviewer: ctx.Reviewer, Agreement: 1,
				})
			}
		}
	}
	return out
}

func reviewCompleteness(ctx *FreshContext) []Finding {
	if ctx.Ticket == nil || len(ctx.Ticket.Acceptance) == 0 {
		return nil
	}
	lowerDiff := strings.ToLower(ctx.Diff + " " + strings.Join(ctx.Changed, " "))
	var out []Finding
	for i, crit := range ctx.Ticket.Acceptance {
		key := strings.ToLower(crit)
		tokens := strings.Fields(key)
		hit := 0
		for _, tok := range tokens {
			if len(tok) < 4 {
				continue
			}
			if strings.Contains(lowerDiff, tok) {
				hit++
			}
		}
		meaningful := 0
		for _, tok := range tokens {
			if len(tok) >= 4 {
				meaningful++
			}
		}
		if meaningful > 0 && hit*2 < meaningful {
			out = append(out, Finding{
				ID: fmt.Sprintf("accept-%d", i), Severity: SeverityWarning, Category: "completeness",
				Message:  "critério de aceite pouco evidenciado no diff: " + crit,
				Reviewer: ctx.Reviewer, Agreement: 1,
			})
		}
	}
	return out
}

func DetectDrift(ticket *TicketScope, changed []string, actualBase string) *DriftReport {
	rep := &DriftReport{ActualBase: actualBase}
	if ticket == nil {
		return rep
	}
	rep.DeclaredBase = ticket.BaseCommit
	if ticket.BaseCommit != "" && actualBase != "" && ticket.BaseCommit != actualBase {
		rep.BaseMismatch = true
	}
	for _, f := range changed {
		if len(ticket.Scope) > 0 && !inScope(f, ticket.Scope) {
			rep.OutOfScope = append(rep.OutOfScope, f)
		}
		if hitForbidden(f, ticket.Forbidden) {
			rep.ForbiddenHit = append(rep.ForbiddenHit, f)
		}
	}
	return rep
}

func LoadTicket(path string) (*TicketScope, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	ticket, ok, err := parseTicketFrontmatter(text)
	if err != nil {
		return nil, err
	}
	if ok {
		return ticket, nil
	}
	return parseTicketMarkdown(text), nil
}

func parseTicketFrontmatter(text string) (*TicketScope, bool, error) {
	if !strings.HasPrefix(strings.TrimSpace(text), "---") {
		return nil, false, nil
	}
	parts := strings.SplitN(text, "---", 3)
	if len(parts) < 3 {
		return nil, false, nil
	}
	var t TicketScope
	if err := yaml.Unmarshal([]byte(parts[1]), &t); err != nil {
		return nil, false, err
	}
	if t.Description == "" {
		t.Description = strings.TrimSpace(parts[2])
	}
	return &t, true, nil
}

func parseTicketMarkdown(text string) *TicketScope {
	t := &TicketScope{Description: text}
	reBase := regexp.MustCompile(`(?mi)^(?:base_commit|base)\s*:\s*(\S+)`)
	t.Scope = parseMarkdownScope(text)
	t.Forbidden = parseMarkdownForbidden(text)
	if m := reBase.FindStringSubmatch(text); len(m) > 1 {
		t.BaseCommit = strings.TrimSpace(m[1])
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- [ ]") || strings.HasPrefix(line, "- [x]") {
			t.Acceptance = append(t.Acceptance, strings.TrimSpace(line[5:]))
		}
	}
	return t
}

func parseMarkdownScope(text string) []string {
	if items := collectHeadingList(text, "scope", "escopo"); len(items) > 0 {
		return items
	}
	return collectLabelList(text, "scope", "escopo")
}

func parseMarkdownForbidden(text string) []string {
	if items := collectHeadingList(text, "forbidden", "proibido"); len(items) > 0 {
		return items
	}
	return collectLabelList(text, "forbidden", "proibido")
}

func collectHeadingList(text string, names ...string) []string {
	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[strings.ToLower(n)] = true
	}
	headingRe := regexp.MustCompile(`(?i)^#{1,6}\s+(.+?)\s*$`)
	lines := strings.Split(text, "\n")
	inSection := false
	var items []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := headingRe.FindStringSubmatch(trimmed); len(m) > 1 {
			title := strings.ToLower(strings.TrimSpace(m[1]))
			if inSection {
				break
			}
			if nameSet[title] {
				inSection = true
			}
			continue
		}
		if !inSection {
			continue
		}
		if trimmed == "" {
			if len(items) > 0 {
				break
			}
			continue
		}
		if item, ok := parseListItem(trimmed); ok {
			if item != "" {
				items = append(items, item)
			}
			continue
		}
		if len(items) > 0 {
			break
		}
	}
	return items
}

func collectLabelList(text string, names ...string) []string {
	labelRe := regexp.MustCompile(`(?i)^(?:` + strings.Join(names, "|") + `)\s*:\s*(.*)$`)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		m := labelRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		var items []string
		rest := strings.TrimSpace(m[1])
		if rest != "" {
			if item, ok := parseListItem(rest); ok {
				if item != "" {
					items = append(items, item)
				}
			} else {
				items = append(items, splitList(rest)...)
			}
		}
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" {
				if len(items) > 0 {
					break
				}
				continue
			}
			if item, ok := parseListItem(trimmed); ok {
				if item != "" {
					items = append(items, item)
				}
				continue
			}
			break
		}
		return items
	}
	return nil
}

var listItemRe = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s+(.+)$`)

func parseListItem(line string) (string, bool) {
	m := listItemRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return "", false
	}
	return cleanScopePath(m[1]), true
}

func cleanScopePath(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '`' {
		if end := strings.IndexByte(s[1:], '`'); end >= 0 {
			s = s[1 : 1+end]
		}
	}
	return strings.Trim(strings.TrimSpace(s), "\"'")
}

func splitList(s string) []string {
	parts := regexp.MustCompile(`[, ]+`).Split(strings.TrimSpace(s), -1)
	var out []string
	for _, p := range parts {
		p = cleanScopePath(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func inScope(file string, scope []string) bool {
	file = filepath.ToSlash(file)
	for _, s := range scope {
		s = filepath.ToSlash(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if matchPathGlob(file, s) {
			return true
		}
	}
	return false
}

func matchPathGlob(file, pattern string) bool {
	if strings.HasSuffix(pattern, "/**") && strings.Count(pattern, "**") == 1 {
		prefix := strings.TrimSuffix(pattern, "/**")
		return file == prefix || strings.HasPrefix(file, prefix+"/")
	}
	if strings.Contains(pattern, "**") {
		re, err := doublestarRegexp(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(file)
	}
	if strings.ContainsAny(pattern, "*?") {
		ok, _ := filepath.Match(pattern, file)
		return ok
	}
	return file == pattern || strings.HasPrefix(file, strings.TrimSuffix(pattern, "/")+"/")
}

func doublestarRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteByte('^')
	for i := 0; i < len(pattern); {
		if i+1 < len(pattern) && pattern[i] == '*' && pattern[i+1] == '*' {
			i = appendDoublestar(&b, pattern, i+2)
			continue
		}
		switch pattern[i] {
		case '*':
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
		i++
	}
	b.WriteByte('$')
	return regexp.Compile(b.String())
}

func appendDoublestar(b *strings.Builder, pattern string, i int) int {
	if i < len(pattern) && pattern[i] == '/' {
		i++
	}
	s := b.String()
	switch {
	case strings.HasSuffix(s, "/"):
		b.Reset()
		b.WriteString(strings.TrimSuffix(s, "/"))
		b.WriteString("(?:/.*)?")
		if i < len(pattern) && pattern[i] != '/' {
			b.WriteByte('/')
		}
	case s == "^":
		b.WriteString("(?:.*/)?")
	default:
		b.WriteString(".*")
	}
	return i
}

func hitForbidden(file string, forbidden []string) bool {
	return len(forbidden) > 0 && inScope(file, forbidden)
}

func suggestsViolation(rule, lowerDiff string, changed []string) bool {
	pathMarkers := []string{"tests/hidden", "thresholds.yaml"}
	for _, marker := range pathMarkers {
		if !strings.Contains(rule, marker) {
			continue
		}
		patterns := []string{marker, marker + "/**", marker + "/*"}
		for _, f := range changed {
			if pathMarkerHit(f, marker) {
				return true
			}
			file := filepath.ToSlash(f)
			for _, p := range patterns {
				if matchPathGlob(file, p) {
					return true
				}
			}
		}
	}
	contentMarkers := []string{"t.skip", "__eq__"}
	for _, c := range contentMarkers {
		if strings.Contains(rule, c) && strings.Contains(lowerDiff, c) {
			return true
		}
	}
	return false
}

func pathMarkerHit(file, marker string) bool {
	file = filepath.ToSlash(strings.ToLower(file))
	marker = filepath.ToSlash(strings.ToLower(strings.Trim(marker, "/")))
	if marker == "" {
		return false
	}
	if !strings.Contains(marker, "/") {
		return filepath.Base(file) == marker
	}
	if file == marker || strings.HasPrefix(file, marker+"/") {
		return true
	}
	if strings.Contains(file, "/"+marker+"/") || strings.HasSuffix(file, "/"+marker) {
		return true
	}
	return false
}

func loadInvariants(repo string) ([]mem.Invariant, error) {
	dir := mem.LocateRepoToolbeltDir(repo)
	return mem.NewStore(dir).List("")
}

func listChangedFiles(repo, base, head string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", base+".."+head) //nolint:gosec // refs do operador
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func gitDiff(repo, base, head string) (string, error) {
	cmd := exec.Command("git", "diff", base+".."+head) //nolint:gosec // refs do operador
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

func aggregateFindings(all []Finding) []Finding {
	type key struct{ id, file, msg string }
	counts := map[key]Finding{}
	for _, f := range all {
		k := key{f.ID, f.File, f.Message}
		cur, ok := counts[k]
		if !ok {
			counts[k] = f
			continue
		}
		cur.Agreement++
		counts[k] = cur
	}
	out := make([]Finding, 0, len(counts))
	for _, f := range counts {
		out = append(out, f)
	}
	return out
}
