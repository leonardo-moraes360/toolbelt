package egm

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

const (
	maxBraceExpand = 64
	maxGlobEntries = 1000
	mirrorMarker   = "(+ docs/pt-BR/)"
)

// PlanRow is one row of the detailed plan's file table (R13).
type PlanRow struct {
	Line   int
	Action string
	Paths  []string // after {a,b} and mirror expansion
	Raw    []string // spans as written
	err    string   // expansion problem (major finding)
}

var reTableSep = regexp.MustCompile(`^\|?\s*:?-{3,}`)

func splitRow(l string) []string {
	l = strings.TrimSpace(l)
	l = strings.TrimPrefix(l, "|")
	l = strings.TrimSuffix(l, "|")
	cells := strings.Split(l, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

func colIndex(header []string, names ...string) int {
	for i, h := range header {
		for _, n := range names {
			if strings.EqualFold(h, n) {
				return i
			}
		}
	}
	return -1
}

// ParsePlan returns the rows of the first DetailedRange table with a
// File/Arquivo and an Action/Ação column (R13 steps 1–3).
func ParsePlan(d *Doc) []PlanRow {
	if !d.HasDetailed() {
		return nil
	}
	for n := d.Detailed[0]; n < d.Detailed[1]; n++ {
		if d.InFence(n) || !strings.HasPrefix(strings.TrimSpace(d.Lines[n-1]), "|") {
			continue
		}
		header := splitRow(d.Lines[n-1])
		fc, ac := colIndex(header, "File", "Arquivo"), colIndex(header, "Action", "Ação")
		if fc < 0 || ac < 0 || !reTableSep.MatchString(strings.TrimSpace(d.Lines[n])) {
			continue
		}
		return planRows(d, n+2, fc, ac)
	}
	return nil
}

func planRows(d *Doc, start, fc, ac int) []PlanRow {
	var rows []PlanRow
	for n := start; n <= d.Detailed[1]; n++ {
		l := strings.TrimSpace(d.Lines[n-1])
		if !strings.HasPrefix(l, "|") {
			break
		}
		cells := splitRow(l)
		if fc >= len(cells) || ac >= len(cells) {
			continue
		}
		rows = append(rows, planRow(n, cells[fc], cells[ac]))
	}
	return rows
}

func planRow(line int, cell, action string) PlanRow {
	r := PlanRow{Line: line, Action: action}
	outside := reSpan.ReplaceAllString(cell, "")
	mirror := strings.Contains(outside, mirrorMarker)
	for _, m := range reSpan.FindAllStringSubmatch(cell, -1) {
		r.Raw = append(r.Raw, m[1])
		exp, err := expandBraces(m[1])
		if err != "" {
			r.err = err
			continue
		}
		r.Paths = append(r.Paths, exp...)
	}
	if mirror {
		for _, p := range append([]string(nil), r.Paths...) {
			if strings.HasPrefix(p, "docs/") && !strings.HasPrefix(p, "docs/pt-BR/") {
				r.Paths = append(r.Paths, "docs/pt-BR/"+strings.TrimPrefix(p, "docs/"))
			}
		}
	}
	return r
}

// expandBraces expands one level-at-a-time {a,b} groups, up to 64 paths.
func expandBraces(s string) (paths []string, problem string) {
	out := []string{s}
	for {
		var next []string
		grew := false
		for _, p := range out {
			i := strings.Index(p, "{")
			if i < 0 {
				next = append(next, p)
				continue
			}
			j := strings.Index(p[i:], "}")
			if j < 0 {
				return nil, "unbalanced braces in `" + s + "`"
			}
			inner := p[i+1 : i+j]
			if strings.Contains(inner, "{") {
				return nil, "nested braces in `" + s + "`"
			}
			grew = true
			for _, alt := range strings.Split(inner, ",") {
				next = append(next, p[:i]+alt+p[i+j+1:])
			}
		}
		if len(next) > maxBraceExpand {
			return nil, fmt.Sprintf("%#q expands to more than %d paths", s, maxBraceExpand)
		}
		out = next
		if !grew {
			return out, ""
		}
	}
}

// createExempt returns the paths and directories a refs-exist reference may
// point at without existing yet: the plan's Create paths and their parents
// (for a glob, the prefix before the first *).
func createExempt(rows []PlanRow) map[string]bool {
	ex := map[string]bool{}
	for _, r := range rows {
		if !strings.EqualFold(r.Action, "Create") {
			continue
		}
		for _, p := range r.Paths {
			if i := strings.Index(p, "*"); i >= 0 {
				p = p[:i]
			}
			p = strings.TrimSuffix(p, "/")
			for p != "." && p != "" {
				ex[p] = true
				p = path.Dir(p)
			}
		}
	}
	return ex
}

type planCheck struct {
	d      *Doc
	docRel string
	root   string
	idx    indexView
	res    *CheckResult
}

func (c *planCheck) finding(r *PlanRow, sev, key, msg string) {
	c.res.Findings = append(c.res.Findings, Finding{
		Reviewer: checkPlan, Severity: sev,
		Scope:   Scope{Heading: c.d.HeadingAt(r.Line), Lines: []int{r.Line, r.Line}},
		Message: msg, Evidence: []Evidence{{Path: c.docRel, Line: r.Line}},
		Key: "plan:" + key,
	})
}

func checkPlanFiles(ctx context.Context, d *Doc, docRel, root string, idx indexView, res *CheckResult) error {
	rows := ParsePlan(d)
	c := &planCheck{d: d, docRel: docRel, root: root, idx: idx, res: res}
	var plain []string
	for i := range rows {
		plain = append(plain, c.validateRow(&rows[i])...)
	}
	found, err := idx.exists(ctx, plain)
	if err != nil {
		return err
	}
	for i := range rows {
		if err := c.checkExistence(ctx, &rows[i], found); err != nil {
			return err
		}
	}
	return nil
}

// validateRow runs the static checks and returns the plain (glob-free) paths
// that need an index query.
func (c *planCheck) validateRow(r *PlanRow) []string {
	if r.err != "" {
		c.finding(r, SevMajor, "expand:"+strings.Join(r.Raw, ","), r.err)
	}
	act := strings.ToLower(r.Action)
	if act != "create" && act != "modify" && act != "delete" {
		c.finding(r, SevMajor, "action:"+r.Action+":"+strings.Join(r.Raw, ","), fmt.Sprintf("plan action %q must be Create, Modify or Delete", r.Action))
	}
	var plain []string
	for _, p := range r.Paths {
		if why := c.badPath(p); why != "" {
			c.finding(r, SevMajor, "path:"+p, fmt.Sprintf("plan path %#q %s", p, why))
			continue
		}
		if !strings.Contains(p, "*") {
			plain = append(plain, strings.TrimSuffix(p, "/"))
		}
	}
	return plain
}

func (c *planCheck) badPath(p string) string {
	if strings.Contains(p, "**") {
		return "uses `**`, which the plan does not accept"
	}
	rel, why := cleanRel("", p)
	if why != "" {
		return "is " + strings.ReplaceAll(why, "_", " ")
	}
	if i := strings.Index(rel, "*"); i >= 0 {
		rel = path.Dir(rel[:i+1])
		if rel == "." {
			return "is a glob at the repository root, which would sweep the whole index"
		}
	}
	if _, why, _ := lstatChain(c.root, strings.TrimSuffix(rel, "/")); why == "symlink" {
		return "crosses a symlink"
	}
	return ""
}

func (c *planCheck) checkExistence(ctx context.Context, r *PlanRow, found map[string]string) error {
	act := strings.ToLower(r.Action)
	for _, p := range r.Paths {
		if c.badPath(p) != "" {
			continue
		}
		kind, err := c.kindOf(ctx, p, found)
		var e *Error
		if errors.As(err, &e) && e.Reason == "too_many_entries" {
			c.finding(r, SevMajor, "glob:"+p, fmt.Sprintf("plan glob %#q matches under a directory with more than %d versioned files; list the files", p, maxGlobEntries))
			continue
		}
		if err != nil {
			return err
		}
		c.res.inputs = append(c.res.inputs, p+"\t"+kindOrMissing(kind))
		switch {
		case act == "create" && kind != "" && !strings.Contains(p, "*"):
			c.finding(r, SevNit, "exists:"+p, fmt.Sprintf("plan marks %#q as Create, but it already exists", p))
		case (act == "modify" || act == "delete") && kind == "":
			c.finding(r, SevMajor, "missing:"+p, fmt.Sprintf("plan marks %#q as %s, but it does not exist", p, r.Action))
		}
	}
	return nil
}

func (c *planCheck) kindOf(ctx context.Context, p string, found map[string]string) (string, error) {
	i := strings.Index(p, "*")
	if i < 0 {
		return found[strings.TrimSuffix(p, "/")], nil
	}
	dir := path.Dir(p[:i+1])
	files, err := c.idx.under(ctx, dir, maxGlobEntries)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if MatchGlob(p, f) {
			return "file", nil
		}
	}
	return "", nil
}

func kindOrMissing(k string) string {
	if k == "" {
		return "missing"
	}
	return k
}
