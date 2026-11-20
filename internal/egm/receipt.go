package egm

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
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
	if len(s.doc.Lines) == 0 {
		return nil, errorf(ExitError, "usage", "%s is empty", s.rel)
	}
	// The lock keeps a concurrent resolve or loop next from changing the
	// state between the verdict and the claims.
	unlock, err := lockState(s.stateDir)
	if err != nil {
		return nil, err
	}
	defer unlock()
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
	if err := s.committedAtHead(ctx); err != nil {
		return nil, err
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

// committedAtHead refuses when the document or an included source differs
// from HEAD's blob: CI recomputes the hashes from the commit (R7a), so a
// receipt over uncommitted bytes (or bytes an eol/LFS filter rewrites) would
// always be receipt_stale there.
func (s *snapshot) committedAtHead(ctx context.Context) error {
	head, err := s.repo.RevParse(ctx, "HEAD")
	if err != nil {
		return err
	}
	b, _, err := readBlobFile(ctx, s.repo, head, s.rel, s.set.MaxPacketBytes)
	if err != nil {
		return err
	}
	if b == nil || sha256Hex(b) != s.docSHA {
		return errorf(ExitError, "receipt_uncommitted", "%s differs from HEAD: commit it first, then write the receipt", s.rel)
	}
	at, err := CollectSourcesAt(ctx, s.repo, &s.set, head, s.rel, s.text)
	if err != nil {
		return err
	}
	want := map[string]string{}
	for _, src := range at.Included {
		want[src.Path] = src.SHA256
	}
	for _, src := range s.sources.Included {
		if want[src.Path] != src.SHA256 {
			return errorf(ExitError, "receipt_uncommitted", "source %s differs from HEAD: commit it first, then write the receipt", src.Path)
		}
		delete(want, src.Path)
	}
	for p := range want {
		return errorf(ExitError, "receipt_uncommitted", "source %s is included at HEAD but not in the worktree", p)
	}
	return nil
}

// buildReceipt computes sources from the current snapshot: PASS and
// PASS_PARTIAL pin the content hash to the converged round (R25), so they
// equal the last round's sources (T-R1).
func (s *snapshot) buildReceipt(ctx context.Context, st *State) (*Receipt, error) {
	rc := &Receipt{Version: receiptVersion, Doc: s.rel, DocSHA256: s.docSHA, Sources: map[string]string{}, Claims: []Claim{}}
	for _, src := range s.sources.Included {
		rc.Sources[src.Path] = src.SHA256
	}
	maps := map[int]*LineMap{}
	for _, l := range st.Lineages {
		kind := claimKind(st, l)
		if kind == "" {
			continue
		}
		lines, err := s.claimLines(ctx, l, maps)
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
		case t.By == "arbiter" && strings.HasPrefix(t.Reason, reasonWithdrawn):
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
func (s *snapshot) claimLines(ctx context.Context, l *Lineage, maps map[int]*LineMap) ([]int, error) {
	if len(l.Scope.Lines) != 2 {
		return s.sectionLines(l.Scope.Heading), nil
	}
	lm, err := s.roundMap(ctx, l.ScopeRound, maps)
	if err != nil {
		return nil, err
	}
	if m := lm.MapScope(l.Scope).Lines; len(m) == 2 && m[0] >= 1 && m[1] <= len(s.doc.Lines) {
		return m, nil
	}
	return s.sectionLines(l.Scope.Heading), nil
}

// roundMap maps round n's snapshot to the current document, cached per round.
func (s *snapshot) roundMap(ctx context.Context, n int, maps map[int]*LineMap) (*LineMap, error) {
	if lm, ok := maps[n]; ok {
		return lm, nil
	}
	old, err := s.readRound(n, s.rel)
	if err != nil {
		return nil, err
	}
	if old == "" {
		return nil, errorf(ExitError, "state", "round %d snapshot of %s is missing", n, s.rel)
	}
	lm, err := DiffSnapshots(ctx, s.stateDir, old, s.text)
	if err != nil {
		return nil, err
	}
	maps[n] = lm
	return lm, nil
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

// writeReceiptFile writes rel under root through os.Root, so no symlink
// can take the write outside the repository; a path component that is a
// symlink or not a directory is refused outright.
func writeReceiptFile(root, rel string, data []byte) error {
	r, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer r.Close()
	dir := path.Dir(rel)
	parts := strings.Split(dir, "/")
	for i := range parts {
		p := strings.Join(parts[:i+1], "/")
		fi, err := r.Lstat(p)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if err := r.Mkdir(p, 0o755); err != nil {
				return err
			}
		case err != nil:
			return err
		case !fi.IsDir():
			return errorf(ExitError, "receipt_path_untrusted", "%s is not a directory (symlink or file)", p)
		}
	}
	if fi, err := r.Lstat(rel); err == nil && !fi.Mode().IsRegular() {
		return errorf(ExitError, "receipt_path_untrusted", "%s is not a regular file", rel)
	}
	tmp := path.Join(dir, "."+path.Base(rel)+".tmp")
	f, err := r.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|oNoFollow, 0o644)
	if err != nil {
		return err
	}
	defer r.Remove(tmp) //nolint:errcheck // no-op after the rename
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return r.Rename(tmp, rel)
}
