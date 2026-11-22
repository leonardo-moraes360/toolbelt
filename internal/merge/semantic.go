package merge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Reasons a conflicted file or region stays unresolved (docs/specs/merge-strategies.md).
const (
	reasonAddAdd         = "add_add"
	reasonDeleteModify   = "delete_modify"
	reasonMode           = "mode"
	reasonSymlink        = "symlink"
	reasonGitlink        = "gitlink"
	reasonBinary         = "binary"
	reasonSensitive      = "sensitive"
	reasonTooLarge       = "too_large"
	reasonOverlap        = "overlap"
	reasonRegionTooLarge = "region_too_large"
	reasonMarkers        = "markers"
	reasonParseFailed    = "parse_failed"
	reasonMergeFile      = "merge_file_failed"
)

// Rule names recorded per resolved region.
const (
	ruleDisjoint  = "disjoint"
	ruleGoImports = "go_imports"
	ruleAgent     = "agent"
)

// Hunk is one conflict region of a file: where it sits in the ours/theirs
// stage files ([start,end], 1-based; end < start for an empty side, the lines
// are located by a forward search, so they are exact only for unique text)
// and how it was resolved or why it was not.
type Hunk struct {
	Region      int    `json:"region"`
	OursLines   [2]int `json:"ours_lines"`
	TheirsLines [2]int `json:"theirs_lines"`
	Reason      string `json:"reason,omitempty"`
	Rule        string `json:"rule,omitempty"`
	NovelLines  int    `json:"novel_lines,omitempty"`
}

// Resolution records one resolved region with the hashes of its inputs, so a
// reviewer (or the apply step of a receipt) can tell what it was built from.
type Resolution struct {
	File         string `json:"file"`
	Ticket       string `json:"ticket"`
	Commit       string `json:"commit"`
	Region       int    `json:"region"`
	Rule         string `json:"rule"`
	BaseSHA256   string `json:"base_sha256"`
	OursSHA256   string `json:"ours_sha256"`
	TheirsSHA256 string `json:"theirs_sha256"`
	ResultSHA256 string `json:"result_sha256"`
	NovelLines   int    `json:"novel_lines,omitempty"`

	// Text is the resolved lines; it only goes into receipts.
	Text string `json:"-"`
}

type stage struct {
	mode, blob string
	data       []byte
}

// region is one conflict region of `git merge-file --diff3` output.
type region struct {
	ID                 int // 1-based
	Base, Ours, Theirs []string
	OursRange          [2]int
	TheirsRange        [2]int
	// OursExact and TheirsExact say the ranges are certain: the region text
	// occurs once in the stage file, or an empty side sits between a unique
	// pair of surrounding clean lines.
	OursExact, TheirsExact bool
}

type segment struct {
	clean string
	reg   *region // nil for clean text
}

// fileState follows one conflicted file through the semantic pass, the
// optional agent pass and the final checks.
type fileState struct {
	path           string
	mode           string
	stages         [3]stage // base, ours, theirs
	segs           []segment
	regions        []*region
	markerSize     int
	texts          map[int][]string
	rules          map[int]string
	reasons        map[int]string
	novel          map[int]int
	oursGo, thrGo  *goFile
	goTried        bool
	maxRegionLines int
}

type goFile struct {
	fset *token.FileSet
	file *ast.File
}

// resolver carries the state shared by every pick of one merge.
type resolver struct {
	r              runner
	wt             string
	maxRegionLines int
	maxFileBytes   int64
	sensitive      map[string]bool
	out            []Resolution
	agent          *agentEnv // prompt proposal
	replay         *replay   // prompt apply
	// hook, when set (prompt), gets the files left unresolved by the rules.
	hook func(rv *resolver, ctx *pickCtx, files []*fileState) error
}

// pickCtx identifies the cherry-pick being resolved.
type pickCtx struct {
	commit, ticket string
}

func hexSum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func joinLines(l []string) string { return strings.Join(l, "") }

func splitLines(s string) []string {
	var out []string
	for s != "" {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			out = append(out, s)
			break
		}
		out = append(out, s[:i+1])
		s = s[i+1:]
	}
	return out
}

// resolvePick resolves the unmerged files of one cherry-pick. conflicts holds
// one entry per file (same order as files); they are updated in place.
// allResolved is false when any file stays in conflict (nothing was staged
// for those, and the caller aborts the pick).
func (rv *resolver) resolvePick(ctx *pickCtx, files []string, conflicts []Conflict) (allResolved bool, err error) {
	stages, err := rv.readStages()
	if err != nil {
		return false, err
	}
	var states []*fileState
	idx := map[*fileState]int{}
	for i, f := range files {
		st, reason, err := rv.prepareFile(ctx, f, stages[f])
		if err != nil {
			return false, err
		}
		if reason != "" {
			conflicts[i].Reason = reason
			continue
		}
		states = append(states, st)
		idx[st] = i
	}
	for _, st := range states {
		st.semanticPass()
	}
	if rv.hook != nil {
		if err := rv.hook(rv, ctx, states); err != nil {
			return false, err
		}
	}
	var pending []Resolution
	for _, st := range states {
		rss, err := rv.settle(ctx, st, &conflicts[idx[st]])
		if err != nil {
			return false, err
		}
		pending = append(pending, rss...)
	}
	for i := range conflicts {
		if conflicts[i].Resolution == "" {
			return false, nil
		}
	}
	// Only a pick that resolved whole is applied, so only its regions count.
	rv.out = append(rv.out, pending...)
	return true, nil
}

// settle runs the final checks of one file and, when it passes, stages it. It
// returns the file's resolutions; the caller records them once the whole pick
// resolved.
func (rv *resolver) settle(ctx *pickCtx, st *fileState, c *Conflict) ([]Resolution, error) {
	content, reason := st.finish()
	c.Hunks = st.hunks()
	if reason != "" {
		c.Reason = reason
		return nil, nil
	}
	if err := rv.stage(st, content); err != nil {
		return nil, err
	}
	c.Resolution = string(StrategySemantic)
	rss := st.resolutions(ctx)
	for i := range rss {
		if rss[i].Rule == ruleAgent {
			c.Resolution = string(StrategyPrompt)
		}
	}
	return rss, nil
}

// readStages maps each unmerged path to its index stages (1=base, 2=ours, 3=theirs).
func (rv *resolver) readStages() (map[string]*[3]stage, error) {
	out, err := rv.r.git(rv.wt, "ls-files", "-u", "-z")
	if err != nil {
		return nil, err
	}
	res := map[string]*[3]stage{}
	for _, ent := range strings.Split(out, "\x00") {
		meta, path, ok := strings.Cut(ent, "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta)
		if len(f) != 3 {
			continue
		}
		n, err := strconv.Atoi(f[2])
		if err != nil || n < 1 || n > 3 {
			continue
		}
		if res[path] == nil {
			res[path] = &[3]stage{}
		}
		res[path][n-1] = stage{mode: f[0], blob: f[1]}
	}
	return res, nil
}

// prepareFile classifies f and, for a text file, runs git merge-file and
// parses its regions. A non-empty reason is a file-level refusal.
func (rv *resolver) prepareFile(ctx *pickCtx, path string, stages *[3]stage) (*fileState, string, error) {
	if stages == nil {
		return nil, reasonDeleteModify, nil
	}
	if rv.sensitive[path] {
		return nil, reasonSensitive, nil
	}
	if reason := classifyStages(stages); reason != "" {
		return nil, reason, nil
	}
	if refused, err := rv.attrRefuses(ctx, path); err != nil || refused {
		return nil, reasonBinary, err
	}
	for i := range stages {
		size, err := rv.r.out(rv.wt, "cat-file", "-s", stages[i].blob)
		if err != nil {
			return nil, "", err
		}
		if n, _ := strconv.ParseInt(size, 10, 64); n > rv.maxFileBytes {
			return nil, reasonTooLarge, nil
		}
	}
	for i := range stages {
		data, err := rv.r.git(rv.wt, "cat-file", "blob", stages[i].blob)
		if err != nil {
			return nil, "", err
		}
		stages[i].data = []byte(data)
		if bytes.IndexByte(stages[i].data[:min(len(data), 8000)], 0) >= 0 {
			return nil, reasonBinary, nil
		}
	}
	st := &fileState{path: path, mode: stages[1].mode, stages: *stages, maxRegionLines: rv.maxRegionLines,
		texts: map[int][]string{}, rules: map[int]string{}, reasons: map[int]string{}, novel: map[int]int{}}
	st.markerSize = markerSizeFor(stages)
	merged, err := rv.mergeFile(st)
	if err != nil {
		return nil, reasonMergeFile, nil //nolint:nilerr // a merge-file failure leaves the file in conflict
	}
	if err := st.parse(merged); err != nil {
		return nil, reasonMergeFile, nil //nolint:nilerr // unparsable output leaves the file in conflict
	}
	return st, "", nil
}

// attrRefuses reports whether .gitattributes at the picked commit marks path
// as not text-mergeable: `-merge`, `merge=binary` or `binary`.
func (rv *resolver) attrRefuses(ctx *pickCtx, path string) (bool, error) {
	out, err := rv.r.git(rv.wt, "check-attr", "-z", "--source="+ctx.commit, "merge", "binary", "--", path)
	if err != nil {
		return false, err
	}
	f := strings.Split(out, "\x00")
	for i := 0; i+2 < len(f); i += 3 {
		switch name, val := f[i+1], f[i+2]; {
		case name == "merge" && (val == "unset" || val == "binary"):
			return true, nil
		case name == "binary" && val == "set":
			return true, nil
		}
	}
	return false, nil
}

// classifyStages returns the file-level refusal for the three index stages.
func classifyStages(s *[3]stage) string {
	switch {
	case s[0].blob == "":
		return reasonAddAdd
	case s[1].blob == "" || s[2].blob == "":
		return reasonDeleteModify
	case s[0].mode != s[1].mode || s[1].mode != s[2].mode:
		return reasonMode
	case s[1].mode == "120000":
		return reasonSymlink
	case s[1].mode == "160000":
		return reasonGitlink
	}
	return ""
}

// markerSizeFor is one more than the longest run of <, =, > or | at the start
// of any line of the stages (at least 7), so file content never reads as a
// marker to tb's parser.
func markerSizeFor(s *[3]stage) int {
	longest := 6
	for i := range s {
		for _, line := range bytes.Split(s[i].data, []byte("\n")) {
			if len(line) == 0 || !strings.ContainsRune("<=>|", rune(line[0])) {
				continue
			}
			n := 0
			for n < len(line) && strings.ContainsRune("<=>|", rune(line[n])) {
				n++
			}
			longest = max(longest, n)
		}
	}
	return longest + 1
}

// mergeFile runs `git merge-file -p --diff3` over the three stages in a
// private temp directory. It reads no attributes and runs no merge driver.
func (rv *resolver) mergeFile(st *fileState) (string, error) {
	dir, err := os.MkdirTemp("", "tb-merge-file-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	names := []string{"base", "ours", "theirs"}
	for i, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), st.stages[i].data, 0o600); err != nil {
			return "", err
		}
	}
	cmd, err := rv.r.command(dir, "merge-file", "-p", "--diff3", "--marker-size="+strconv.Itoa(st.markerSize),
		"-L", "ours", "-L", "base", "-L", "theirs", "ours", "base", "theirs")
	if err != nil {
		return "", err
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		// Exit codes 1..127 are the number of conflicts; anything else failed.
		var ee *exec.ExitError
		if !errors.As(err, &ee) || ee.ExitCode() < 1 || ee.ExitCode() > 127 {
			return "", fmt.Errorf("git merge-file: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
	}
	return stdout.String(), nil
}

func isMarker(line string, c byte, n int) bool {
	l := strings.TrimRight(line, "\r\n")
	if len(l) < n {
		return false
	}
	for i := 0; i < n; i++ {
		if l[i] != c {
			return false
		}
	}
	return len(l) == n || l[n] == ' '
}

// parse splits merge-file output into clean text and regions.
func (st *fileState) parse(out string) error {
	const (
		clean = iota
		ours
		base
		theirs
	)
	n := st.markerSize
	state := clean
	var cleanBuf strings.Builder
	var cur *region
	flush := func() {
		if cleanBuf.Len() > 0 {
			st.segs = append(st.segs, segment{clean: cleanBuf.String()})
			cleanBuf.Reset()
		}
	}
	for _, line := range splitLines(out) {
		switch {
		case state == clean && isMarker(line, '<', n):
			flush()
			cur = &region{ID: len(st.regions) + 1}
			state = ours
		case state == clean:
			cleanBuf.WriteString(line)
		case state == ours && isMarker(line, '|', n):
			state = base
		case state == ours:
			cur.Ours = append(cur.Ours, line)
		case state == base && isMarker(line, '=', n):
			state = theirs
		case state == base:
			cur.Base = append(cur.Base, line)
		case state == theirs && isMarker(line, '>', n):
			st.segs = append(st.segs, segment{reg: cur})
			st.regions = append(st.regions, cur)
			cur, state = nil, clean
		default:
			cur.Theirs = append(cur.Theirs, line)
		}
	}
	if state != clean {
		return errors.New("unterminated conflict region")
	}
	flush()
	st.locate()
	return nil
}

// locate finds each region's ours/theirs lines in the stage files by a
// forward search (regions come in file order) and marks whether the answer is
// exact: a non-empty region text must occur once in the file; an empty side is
// placed between the clean lines around it, which must occur once as a pair.
func (st *fileState) locate() {
	oursLines := splitLines(string(st.stages[1].data))
	theirsLines := splitLines(string(st.stages[2].data))
	oc, tc := 0, 0
	for i, sg := range st.segs {
		rg := sg.reg
		if rg == nil {
			continue
		}
		prev, next, hasPrev, hasNext := st.neighbors(i)
		rg.OursRange, oc, rg.OursExact = locateSide(oursLines, rg.Ours, oc, prev, next, hasPrev, hasNext)
		rg.TheirsRange, tc, rg.TheirsExact = locateSide(theirsLines, rg.Theirs, tc, prev, next, hasPrev, hasNext)
	}
}

// neighbors returns the clean line right before and right after segment i.
func (st *fileState) neighbors(i int) (prev, next string, hasPrev, hasNext bool) {
	if i > 0 && st.segs[i-1].reg == nil {
		if l := splitLines(st.segs[i-1].clean); len(l) > 0 {
			prev, hasPrev = l[len(l)-1], true
		}
	}
	if i+1 < len(st.segs) && st.segs[i+1].reg == nil {
		if l := splitLines(st.segs[i+1].clean); len(l) > 0 {
			next, hasNext = l[0], true
		}
	}
	return prev, next, hasPrev, hasNext
}

func locateSide(lines, block []string, from int, prev, next string, hasPrev, hasNext bool) (rng [2]int, cursor int, exact bool) {
	rng, cursor = findBlock(lines, block, from)
	if len(block) > 0 {
		return rng, cursor, countBlock(lines, block) == 1
	}
	if !hasPrev && !hasNext {
		return rng, cursor, false
	}
	// The gap sits after line index at (0-based, -1 = file start).
	var gaps []int
	for at := -1; at < len(lines); at++ {
		if hasPrev && (at < 0 || lines[at] != prev) {
			continue
		}
		if !hasPrev && at != -1 {
			continue
		}
		if hasNext && (at+1 >= len(lines) || lines[at+1] != next) {
			continue
		}
		gaps = append(gaps, at)
	}
	if len(gaps) != 1 {
		return rng, cursor, false
	}
	return [2]int{gaps[0] + 2, gaps[0] + 1}, gaps[0] + 1, true
}

// countBlock is the number of places block occurs in lines.
func countBlock(lines, block []string) int {
	n := 0
	for i := 0; i+len(block) <= len(lines); i++ {
		match := true
		for j := range block {
			if lines[i+j] != block[j] {
				match = false
				break
			}
		}
		if match {
			n++
		}
	}
	return n
}

// findBlock returns the 1-based [start,end] of block in lines at or after
// from, and the new cursor. A missing or empty block gives end = start-1.
func findBlock(lines, block []string, from int) (rng [2]int, next int) {
	if len(block) == 0 {
		return [2]int{from + 1, from}, from
	}
	for i := from; i+len(block) <= len(lines); i++ {
		match := true
		for j := range block {
			if lines[i+j] != block[j] {
				match = false
				break
			}
		}
		if match {
			return [2]int{i + 1, i + len(block)}, i + len(block)
		}
	}
	return [2]int{from + 1, from + len(block)}, from
}

// semanticPass tries the deterministic rules on every region.
func (st *fileState) semanticPass() {
	for _, rg := range st.regions {
		if text, ok, reason := st.disjoint(rg); ok {
			st.texts[rg.ID], st.rules[rg.ID] = text, ruleDisjoint
			continue
		} else if reason != "" {
			st.reasons[rg.ID] = reason
		}
		if text, ok := st.goImports(rg); ok {
			st.texts[rg.ID], st.rules[rg.ID] = text, ruleGoImports
			delete(st.reasons, rg.ID)
		}
	}
}

// unresolved lists the region IDs without a resolution.
func (st *fileState) unresolved() []*region {
	var out []*region
	for _, rg := range st.regions {
		if _, ok := st.texts[rg.ID]; !ok {
			out = append(out, rg)
		}
	}
	return out
}

type edit struct {
	bs, be int
	repl   []string
}

// commonAffixes is the length of the shared prefix and, after it, suffix.
func commonAffixes(a, b []string) (prefix, suffix int) {
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	for suffix < len(a)-prefix && suffix < len(b)-prefix && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	return prefix, suffix
}

// lcsTable is the suffix LCS length table of a and b (row stride len(b)+1).
func lcsTable(a, b []string) []uint16 {
	w := len(b) + 1
	dp := make([]uint16, (len(a)+1)*w)
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i*w+j] = dp[(i+1)*w+j+1] + 1
			} else {
				dp[i*w+j] = max(dp[(i+1)*w+j], dp[i*w+j+1])
			}
		}
	}
	return dp
}

// diffEdits returns the edits that turn base into side (a line LCS after
// trimming the common prefix and suffix).
func diffEdits(base, side []string) []edit {
	p, s := commonAffixes(base, side)
	a, b := base[p:len(base)-s], side[p:len(side)-s]
	w := len(b) + 1
	dp := lcsTable(a, b)
	var edits []edit
	var cur *edit
	flush := func() {
		if cur != nil {
			edits = append(edits, *cur)
			cur = nil
		}
	}
	open := func(at int) {
		if cur == nil {
			cur = &edit{bs: p + at, be: p + at}
		}
	}
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			flush()
			i++
			j++
		case i < len(a) && (j == len(b) || dp[(i+1)*w+j] >= dp[i*w+j+1]):
			open(i)
			cur.be = p + i + 1
			i++
		default:
			open(i)
			cur.repl = append(cur.repl, b[j])
			j++
		}
	}
	flush()
	return edits
}

func editsOverlap(a, b edit) bool {
	aIns, bIns := a.bs == a.be, b.bs == b.be
	switch {
	case aIns && bIns:
		return a.bs == b.bs
	case aIns:
		return b.bs < a.bs && a.bs < b.be
	case bIns:
		return a.bs < b.bs && b.bs < a.be
	}
	return a.bs < b.be && b.bs < a.be
}

// disjoint applies both sides' edits when they touch disjoint base lines.
func (st *fileState) disjoint(rg *region) (text []string, ok bool, reason string) {
	if max(len(rg.Base), len(rg.Ours), len(rg.Theirs)) > st.maxRegionLines {
		return nil, false, reasonRegionTooLarge
	}
	oe, te := diffEdits(rg.Base, rg.Ours), diffEdits(rg.Base, rg.Theirs)
	for _, a := range oe {
		for _, b := range te {
			if editsOverlap(a, b) {
				return nil, false, reasonOverlap
			}
		}
	}
	all := append(append([]edit{}, oe...), te...)
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].bs != all[j].bs {
			return all[i].bs < all[j].bs
		}
		return all[i].be-all[i].bs < all[j].be-all[j].bs
	})
	pos := 0
	for _, e := range all {
		text = append(text, rg.Base[pos:e.bs]...)
		text = append(text, e.repl...)
		pos = e.be
	}
	text = append(text, rg.Base[pos:]...)
	return text, true, ""
}

var reImportSpec = regexp.MustCompile(`^\s*(?:[A-Za-z_][A-Za-z0-9_]*|\.)?\s*"[^"\n]+"\s*$`)

func parseGo(data []byte, mode parser.Mode) *goFile {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", data, mode)
	if err != nil {
		return nil
	}
	return &goFile{fset: fset, file: f}
}

// importDecl returns the ordinal of the parenthesized import declaration that
// holds the 1-based line range [start,end] (an empty range is a position).
func (g *goFile) importDecl(rng [2]int) int {
	ord := 0
	for _, d := range g.file.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		if gd.Lparen.IsValid() {
			lp, rp := g.fset.Position(gd.Lparen).Line, g.fset.Position(gd.Rparen).Line
			inside := lp < rng[0] && rng[1] < rp
			if rng[1] < rng[0] { // empty side: the insertion point must be inside
				inside = lp < rng[0] && rng[0] <= rp
			}
			if inside {
				return ord
			}
		}
		ord++
	}
	return -1
}

func importKey(line string) string { return strings.TrimSpace(line) }

func importPath(spec string) string {
	i, j := strings.IndexByte(spec, '"'), strings.LastIndexByte(spec, '"')
	if i < 0 || j <= i {
		return spec
	}
	return spec[i+1 : j]
}

// goImports merges two sides' import lines of one parenthesized block.
func (st *fileState) goImports(rg *region) ([]string, bool) {
	if !strings.HasSuffix(st.path, ".go") {
		return nil, false
	}
	if !st.goTried {
		st.goTried = true
		o, t := parseGo(st.stages[1].data, parser.ImportsOnly), parseGo(st.stages[2].data, parser.ImportsOnly)
		if o != nil && t != nil {
			st.oursGo, st.thrGo = o, t
		}
	}
	if st.oursGo == nil {
		return nil, false
	}
	for _, side := range [][]string{rg.Base, rg.Ours, rg.Theirs} {
		for _, l := range side {
			if !reImportSpec.MatchString(strings.TrimSuffix(l, "\n")) {
				return nil, false
			}
		}
	}
	if !rg.OursExact || !rg.TheirsExact {
		return nil, false
	}
	od, td := st.oursGo.importDecl(rg.OursRange), st.thrGo.importDecl(rg.TheirsRange)
	if od < 0 || od != td {
		return nil, false
	}
	indent := importIndent(rg)
	specs := mergeImportSpecs(rg)
	out := make([]string, len(specs))
	for i, sp := range specs {
		out[i] = indent + sp + "\n"
	}
	return out, true
}

// mergeImportSpecs is ours ∪ theirs minus the specs either side removed from
// base, sorted by import path.
func mergeImportSpecs(rg *region) []string {
	keep := map[string]bool{}
	for _, side := range [][]string{rg.Ours, rg.Theirs} {
		for _, l := range side {
			keep[importKey(l)] = true
		}
	}
	for _, side := range [][]string{rg.Ours, rg.Theirs} {
		have := map[string]bool{}
		for _, l := range side {
			have[importKey(l)] = true
		}
		for _, b := range rg.Base {
			if !have[importKey(b)] {
				delete(keep, importKey(b))
			}
		}
	}
	specs := make([]string, 0, len(keep))
	for k := range keep {
		specs = append(specs, k)
	}
	sort.Slice(specs, func(i, j int) bool {
		if pi, pj := importPath(specs[i]), importPath(specs[j]); pi != pj {
			return pi < pj
		}
		return specs[i] < specs[j]
	})
	return specs
}

// importIndent is the leading whitespace of the first line of ours (else
// theirs, else base); a tab when the region is empty.
func importIndent(rg *region) string {
	for _, side := range [][]string{rg.Ours, rg.Theirs, rg.Base} {
		if len(side) > 0 {
			l := side[0]
			return l[:len(l)-len(strings.TrimLeft(l, " \t"))]
		}
	}
	return "\t"
}

// assemble joins clean text and region resolutions.
func (st *fileState) assemble() []byte {
	var b bytes.Buffer
	for _, sg := range st.segs {
		if sg.reg == nil {
			b.WriteString(sg.clean)
			continue
		}
		b.WriteString(joinLines(st.texts[sg.reg.ID]))
	}
	out := b.Bytes()
	// git adds a newline after a conflict region at the end of a file that
	// had none; do not keep one that neither side had.
	if n := len(st.segs); n > 0 && st.segs[n-1].reg != nil &&
		!bytes.HasSuffix(st.stages[1].data, []byte("\n")) && !bytes.HasSuffix(st.stages[2].data, []byte("\n")) {
		out = bytes.TrimSuffix(out, []byte("\n"))
	}
	return out
}

// markerLine reports whether l is a conflict marker of size 7 or n.
func markerLine(l string, n int) bool {
	for _, c := range []byte("<|=>") {
		if isMarker(l, c, 7) || isMarker(l, c, n) {
			return true
		}
	}
	return false
}

// finish checks every region's resolution and the whole file (M6). It returns
// the content to stage or the file-level reason.
func (st *fileState) finish() (content []byte, reason string) {
	if len(st.unresolved()) > 0 {
		return nil, firstReason(st)
	}
	if reason := st.checkMarkers(); reason != "" {
		return nil, reason
	}
	content = st.assemble()
	if strings.HasSuffix(st.path, ".go") &&
		parseGo(st.stages[1].data, 0) != nil && parseGo(st.stages[2].data, 0) != nil &&
		parseGo(content, 0) == nil {
		return nil, reasonParseFailed
	}
	return content, ""
}

// checkMarkers refuses a resolved region that holds a conflict-marker line
// its own inputs did not have; the region goes back to unresolved.
func (st *fileState) checkMarkers() string {
	for _, rg := range st.regions {
		seen := map[string]bool{}
		for _, side := range [][]string{rg.Base, rg.Ours, rg.Theirs} {
			for _, l := range side {
				seen[l] = true
			}
		}
		for _, l := range st.texts[rg.ID] {
			if markerLine(l, st.markerSize) && !seen[l] {
				st.reasons[rg.ID] = markersReason(st.rules[rg.ID])
				delete(st.texts, rg.ID)
				return st.reasons[rg.ID]
			}
		}
	}
	return ""
}

func markersReason(rule string) string {
	if rule == ruleAgent {
		return "agent_markers"
	}
	return reasonMarkers
}

// firstReason is the file-level reason of an unresolved file: the reason of
// its first unresolved region.
func firstReason(st *fileState) string {
	for _, rg := range st.regions {
		if _, ok := st.texts[rg.ID]; !ok {
			if r := st.reasons[rg.ID]; r != "" {
				return r
			}
		}
	}
	return reasonOverlap
}

func (st *fileState) hunks() []Hunk {
	out := make([]Hunk, 0, len(st.regions))
	for _, rg := range st.regions {
		h := Hunk{Region: rg.ID, OursLines: rg.OursRange, TheirsLines: rg.TheirsRange, NovelLines: st.novel[rg.ID]}
		if _, ok := st.texts[rg.ID]; ok {
			h.Rule = st.rules[rg.ID]
		} else {
			h.Reason = st.reasons[rg.ID]
		}
		out = append(out, h)
	}
	return out
}

func (st *fileState) resolutions(ctx *pickCtx) []Resolution {
	out := make([]Resolution, 0, len(st.regions))
	for _, rg := range st.regions {
		if _, ok := st.texts[rg.ID]; !ok {
			continue
		}
		text := joinLines(st.texts[rg.ID])
		out = append(out, Resolution{
			File: st.path, Ticket: ctx.ticket, Commit: ctx.commit, Region: rg.ID, Rule: st.rules[rg.ID],
			BaseSHA256: hexSum(joinLines(rg.Base)), OursSHA256: hexSum(joinLines(rg.Ours)),
			TheirsSHA256: hexSum(joinLines(rg.Theirs)), ResultSHA256: hexSum(text),
			NovelLines: st.novel[rg.ID], Text: text,
		})
	}
	return out
}

// stage writes content as the file's blob and checks it out through git, so
// path handling (no symlink traversal) stays git's.
func (rv *resolver) stage(st *fileState, content []byte) error {
	tmp, err := os.CreateTemp("", "tb-merge-res-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	sha, err := rv.r.out(rv.wt, "hash-object", "-w", "--no-filters", "--", tmp.Name())
	if err != nil {
		return err
	}
	if _, err := rv.r.git(rv.wt, "update-index", "--add", "--cacheinfo", st.mode+","+sha+","+st.path); err != nil {
		return err
	}
	_, err = rv.r.git(rv.wt, "checkout-index", "-f", "--", st.path)
	return err
}
