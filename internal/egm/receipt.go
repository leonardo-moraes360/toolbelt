package egm

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Receipt (T-R1, docs/specs/egm-trust.md): the author's claims about a
// document's review, committed next to it and checked by the CI mode. A
// receipt is a claim, not a proof.

const (
	receiptVersion = 1
	receiptDir     = ".toolbelt/egm/receipts"

	ClaimArbiterWithdrawn = "arbiter_withdrawn"
	ClaimDeclined         = "declined"
	ClaimForced           = "forced"
)

// Receipt is the committed file. Fields are declared in key order, so the
// encoding is deterministic.
type Receipt struct {
	Claims    []Claim           `json:"claims"`
	Doc       string            `json:"doc"`
	DocSHA256 string            `json:"doc_sha256"`
	Sources   map[string]string `json:"sources"`
	Version   int               `json:"version"`
}

// Claim is a lineage at major or above that CI must not count as new: one
// the arbiter withdrew, one declined, or one force-settled.
type Claim struct {
	Heading  string `json:"heading"`
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Lines    []int  `json:"lines"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// ReceiptPath is the repository-relative path of rel's receipt.
func ReceiptPath(rel string) string { return path.Join(receiptDir, Slug(rel)+".json") }

// Encode is the receipt's committed bytes: sorted keys, two-space indent and
// a trailing newline.
func (r *Receipt) Encode() []byte {
	b, _ := json.MarshalIndent(r, "", "  ")
	return append(b, '\n')
}

// WriteReceipt is `tb egm receipt`: it refuses unless the worktree verdict is
// PASS or PASS_PARTIAL, then writes the receipt under the repository root.
func WriteReceipt(ctx context.Context, o *Options) (*Outcome, error) {
	if o.At != "" {
		return nil, errorf(ExitError, "usage", "receipt evaluates the worktree; --at is not supported")
	}
	s, err := prepare(ctx, o)
	if err != nil {
		return nil, err
	}
	st, err := loadState(s.stateDir)
	if err != nil {
		return nil, err
	}
	var last *Round
	engine := ""
	if st != nil {
		if last = st.lastRound(); last != nil {
			engine = last.Engine
		}
	}
	rep := s.report(st, last, engine, "")
	if rep.Verdict != VerdictPass && rep.Verdict != VerdictPassPartial {
		return &Outcome{Report: rep, ExitCode: statusExit(rep.Verdict, false),
			Message: "no receipt written: the verdict is " + rep.Verdict + " (needs PASS or PASS_PARTIAL)"}, nil
	}
	rc, err := s.buildReceipt(ctx, st)
	if err != nil {
		return nil, err
	}
	rel := ReceiptPath(s.rel)
	if err := writeReceiptFile(s.repo.Root, rel, rc.Encode()); err != nil {
		return nil, err
	}
	return &Outcome{Report: rep, ExitCode: ExitOK, Receipt: rel, Message: "wrote " + rel}, nil
}

func (s *snapshot) buildReceipt(ctx context.Context, st *State) (*Receipt, error) {
	rc := &Receipt{Version: receiptVersion, Doc: s.rel, DocSHA256: s.docSHA, Sources: map[string]string{}, Claims: []Claim{}}
	for _, src := range s.sources.Included {
		rc.Sources[src.Path] = src.SHA256
	}
	for _, l := range st.Lineages {
		kind := claimKind(st, l)
		if kind == "" {
			continue
		}
		lines, err := s.claimLines(ctx, l)
		if err != nil {
			return nil, err
		}
		msg, _ := Redact(l.Message)
		rc.Claims = append(rc.Claims, Claim{ID: l.ID, Severity: l.Severity, Kind: kind, Heading: l.Scope.Heading, Lines: lines, Message: msg})
	}
	sort.Slice(rc.Claims, func(i, j int) bool { return lineageLess(rc.Claims[i].ID, rc.Claims[j].ID) })
	return rc, nil
}

// claimKind classifies a lineage at major or above; "" means not a claim.
func claimKind(st *State, l *Lineage) string {
	if sevRank(l.Severity) < sevRank(SevMajor) {
		return ""
	}
	if (l.Status == StFixed || l.Status == StSettled) && arbiterWithdrew(st, l.ID) {
		return ClaimArbiterWithdrawn
	}
	switch {
	case l.Status == StDeclined:
		return ClaimDeclined
	case l.Status == StSettled && (l.Downgrade == "forced" || l.Downgrade == "forced_blocker"):
		return ClaimForced
	}
	return ""
}

// arbiterWithdrew reports whether the arbiter withdrew lineage id after its
// last opening or reopening: the owner's later confirmation (R29) keeps the
// claim, a reopening drops it.
func arbiterWithdrew(st *State, id string) bool {
	withdrew := false
	for _, t := range st.Transitions {
		switch {
		case t.Lineage != id:
		case t.By == "arbiter" && strings.HasPrefix(t.Reason, "withdrawn"):
			withdrew = true
		case t.To == StOpen || t.To == StReopened:
			withdrew = false
		}
	}
	return withdrew
}

// claimLines maps the lineage's scope to the current document. A scope
// without lines (or one that no longer fits) falls back to its heading's
// section, and an unknown heading to the whole document: a claim always
// carries lines (T-R1).
func (s *snapshot) claimLines(ctx context.Context, l *Lineage) ([]int, error) {
	n := len(s.doc.Lines)
	if len(l.Scope.Lines) == 2 {
		old, err := s.readRound(l.ScopeRound, s.rel)
		if err != nil {
			return nil, err
		}
		lm, err := DiffSnapshots(ctx, s.stateDir, old, s.text)
		if err != nil {
			return nil, err
		}
		if m := lm.MapScope(l.Scope).Lines; len(m) == 2 && m[0] >= 1 && m[1] <= n {
			return m, nil
		}
	}
	return s.sectionLines(l.Scope.Heading), nil
}

// sectionLines is the range from heading h to the line before the next
// heading at the same or a higher level; the whole document when h is absent.
func (s *snapshot) sectionLines(h string) []int {
	n := len(s.doc.Lines)
	start := s.doc.HeadingLine(h)
	if start == 0 {
		return []int{1, max(1, n)}
	}
	level := headingLevel(s.doc.Lines[start-1])
	for i := start + 1; i <= n; i++ {
		if !s.doc.InFence(i) && reATX.MatchString(s.doc.Lines[i-1]) && headingLevel(s.doc.Lines[i-1]) <= level {
			return []int{start, i - 1}
		}
	}
	return []int{start, n}
}

func headingLevel(line string) int {
	t := strings.TrimSpace(line)
	return len(t) - len(strings.TrimLeft(t, "#"))
}

// lineageLess orders F-2 before F-10.
func lineageLess(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

// writeReceiptFile writes rel under root, refusing a path component that is
// a symlink or not a directory: the receipt must land inside the repository.
func writeReceiptFile(root, rel string, data []byte) error {
	dir := path.Dir(rel)
	parts := strings.Split(dir, "/")
	for i := range parts {
		p := filepath.Join(root, filepath.FromSlash(strings.Join(parts[:i+1], "/")))
		fi, err := os.Lstat(p)
		switch {
		case os.IsNotExist(err):
			if err := os.Mkdir(p, 0o755); err != nil { //nolint:gosec // a committed directory
				return err
			}
		case err != nil:
			return err
		case !fi.IsDir():
			return errorf(ExitError, "receipt_path_untrusted", "%s is not a directory (symlink or file)", strings.Join(parts[:i+1], "/"))
		}
	}
	full := filepath.Join(root, filepath.FromSlash(dir))
	name := path.Base(rel)
	if fi, err := os.Lstat(filepath.Join(full, name)); err == nil && !fi.Mode().IsRegular() {
		return errorf(ExitError, "receipt_path_untrusted", "%s is not a regular file", rel)
	}
	if err := writeAtomic(full, name, data); err != nil {
		return err
	}
	return os.Chmod(filepath.Join(full, name), 0o644) //nolint:gosec // a committed file
}
