package egm

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strings"
)

// Source drift (docs/specs/egm-drift-bootstrap.md, B1): a gated document is
// compared with the sources its baseline certified, by the hashes of the
// baseline's paths, so the review packet budget never decides drift.

const (
	DriftVersion = "egm-source-drift/0.1"

	DriftFresh         = "fresh"
	DriftNoBaseline    = "no_baseline"
	DriftSourcesDiffer = "sources_differ"
	DriftDrifted       = "drifted"
	DriftSourceMissing = "source_missing"
	DriftUnreadable    = "unreadable"

	BaselineReceipt = "receipt"
	BaselineState   = "state"
	BaselineNone    = "none"

	// egm.drift.ci values.
	DriftWarn = "warn"
	DriftFail = "fail"

	// maxDriftBlobBytes bounds one source read for hashing: drift has no
	// R7 size limit, but a blob must still fit in memory.
	maxDriftBlobBytes = 256 << 20
)

// driftRank orders statuses by severity.
var driftRank = map[string]int{
	DriftNoBaseline: 0, DriftFresh: 1, DriftSourcesDiffer: 2, DriftDrifted: 3, DriftSourceMissing: 4, DriftUnreadable: 5,
}

// MissingSource is a baseline path that no longer resolves.
type MissingSource struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// DriftDoc is one document of the report.
type DriftDoc struct {
	Doc      string          `json:"doc"`
	Status   string          `json:"status"`
	Baseline string          `json:"baseline"`
	Changed  []string        `json:"changed"`
	Missing  []MissingSource `json:"missing"`
	Added    []string        `json:"added"`
	Reason   string          `json:"reason,omitempty"`
}

// DriftReport is tb egm drift-report's result.
type DriftReport struct {
	V         string     `json:"v"`
	At        string     `json:"at"`
	Documents []DriftDoc `json:"documents"`
}

// ExitCode is the highest exit among the documents.
func (r *DriftReport) ExitCode() int {
	code := ExitOK
	for i := range r.Documents {
		code = max(code, driftExit(r.Documents[i].Status))
	}
	return code
}

func driftExit(status string) int {
	switch status {
	case DriftFresh, DriftNoBaseline:
		return ExitOK
	case DriftUnreadable:
		return ExitError
	}
	return ExitFail
}

// DriftOptions are drift-report's inputs.
type DriftOptions struct {
	Cwd        string // absolute
	Docs       []string
	At         string // default HEAD
	ConfigFile string
}

// Drift runs tb egm drift-report.
func Drift(ctx context.Context, o *DriftOptions) (*DriftReport, error) {
	repo, err := DiscoverRepo(filepath.Join(o.Cwd, ".egm-drift"))
	if err != nil {
		return nil, err
	}
	at := o.At
	if at == "" {
		at = "HEAD"
	}
	oid, err := repo.RevParse(ctx, at)
	if err != nil {
		return nil, err
	}
	set, err := gateSettings(ctx, repo, oid, o.ConfigFile)
	if err != nil {
		return nil, err
	}
	docs, err := driftDocs(ctx, repo, oid, o, set)
	if err != nil {
		return nil, err
	}
	rep := &DriftReport{V: DriftVersion, At: oid, Documents: []DriftDoc{}}
	for _, d := range docs {
		dd, err := driftOf(ctx, repo, set, oid, d, true)
		if err != nil {
			return nil, err
		}
		rep.Documents = append(rep.Documents, dd)
	}
	sort.SliceStable(rep.Documents, func(i, j int) bool {
		a, b := &rep.Documents[i], &rep.Documents[j]
		if driftRank[a.Status] != driftRank[b.Status] {
			return driftRank[a.Status] > driftRank[b.Status]
		}
		return a.Doc < b.Doc
	})
	return rep, nil
}

// driftDocs: the explicit documents (each must be gated), or every gated
// document at oid.
func driftDocs(ctx context.Context, repo *Repo, oid string, o *DriftOptions, set *Settings) ([]string, error) {
	if len(o.Docs) > 0 {
		cwd, err := filepath.EvalSymlinks(o.Cwd)
		if err != nil {
			return nil, errorf(ExitError, "usage", "%s: %v", o.Cwd, err)
		}
		var out []string
		for _, d := range o.Docs {
			rel, err := gateRel(repo.Root, cwd, d)
			if err != nil {
				return nil, err
			}
			if !IsGated(set.GateDocGlobs, rel) {
				return nil, errorf(ExitError, "usage", "%s is not a gated document (egm.doc_globs)", rel)
			}
			out = appendNew(out, []string{rel})
		}
		sort.Strings(out)
		return out, nil
	}
	return gatedAt(ctx, repo, oid, set.GateDocGlobs)
}

// gatedAt lists every gated document in the tree of oid.
func gatedAt(ctx context.Context, repo *Repo, oid string, globs []string) ([]string, error) {
	var out []string
	for _, dir := range globDirs(globs) {
		names, err := repo.LsTreeNames(ctx, oid, dir, 10000)
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			if IsGated(globs, n) {
				out = appendNew(out, []string{n})
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// baseline is what a document's sources are compared with.
type baseline struct {
	kind    string
	sources map[string]string
}

// driftOf compares rel at oid with its baseline: the receipt at oid, or
// (withState) a converged local round for the same document bytes.
func driftOf(ctx context.Context, repo *Repo, set *Settings, oid, rel string, withState bool) (DriftDoc, error) {
	dd := DriftDoc{Doc: rel, Baseline: BaselineNone, Changed: []string{}, Missing: []MissingSource{}, Added: []string{}}
	text, docSHA, ok, err := docAt(ctx, repo, set, oid, rel)
	if err != nil || !ok {
		dd.Status, dd.Reason = DriftUnreadable, "document unreadable"
		return dd, err
	}
	base, code, err := baselineOf(ctx, repo, oid, rel, text, docSHA, withState)
	switch {
	case err != nil:
		return dd, err
	case code == "receipt_invalid":
		dd.Status, dd.Reason = DriftUnreadable, "receipt_invalid"
		return dd, nil
	case base == nil:
		dd.Status = DriftNoBaseline
		return dd, nil
	}
	dd.Baseline = base.kind
	if err := compareSources(ctx, repo, set, oid, rel, text, base.sources, &dd); err != nil {
		return dd, err
	}
	return dd, nil
}

// docAt reads a gated document's blob at oid.
func docAt(ctx context.Context, repo *Repo, set *Settings, oid, rel string) (text, sha string, ok bool, err error) {
	b, mode, err := readBlobFile(ctx, repo, oid, rel, set.MaxPacketBytes)
	var e *Error
	switch {
	case errors.As(err, &e) && e.Reason == "blob_too_large":
		return "", "", false, nil
	case err != nil:
		return "", "", false, err
	case mode == "" || b == nil:
		return "", "", false, nil
	}
	return string(b), sha256Hex(b), true, nil
}

// baselineOf: the committed receipt at oid when it certifies these
// document bytes, else a converged state round for them.
func baselineOf(ctx context.Context, repo *Repo, oid, rel, text, docSHA string, withState bool) (*baseline, string, error) {
	rc, code, err := readReceiptAt(ctx, repo, oid, rel, strings.Count(text, "\n")+1)
	if err != nil {
		return nil, "", err
	}
	if code == "receipt_invalid" {
		return nil, code, nil
	}
	if rc != nil && rc.DocSHA256 == docSHA {
		return &baseline{kind: BaselineReceipt, sources: rc.Sources}, "", nil
	}
	if !withState {
		return nil, "", nil
	}
	if src := convergedRound(repo, rel, docSHA); src != nil {
		return &baseline{kind: BaselineState, sources: src}, "", nil
	}
	return nil, "", nil
}

// convergedRound returns the sources of the latest round over docSHA whose
// content converged (the rounds a receipt could have been written from).
func convergedRound(repo *Repo, rel, docSHA string) map[string]string {
	dir, err := StateDir(repo.ID, Slug(rel))
	if err != nil {
		return nil
	}
	st, err := loadState(dir)
	if err != nil || st == nil {
		return nil
	}
	converged := map[string]bool{}
	for i := range st.Convergences {
		converged[st.Convergences[i].Content] = true
	}
	for i := len(st.Rounds) - 1; i >= 0; i-- {
		r := &st.Rounds[i]
		if r.DocSHA256 == docSHA && converged[r.Content] {
			return r.Sources
		}
	}
	return nil
}

// compareSources hashes each baseline path at oid (no size limit) and
// collects the R7 sources that are new.
func compareSources(ctx context.Context, repo *Repo, set *Settings, oid, rel, text string, base map[string]string, dd *DriftDoc) error {
	paths := make([]string, 0, len(base))
	for p := range base {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	if err := hashBaseline(ctx, repo, set, oid, rel, paths, base, dd); err != nil {
		return err
	}
	cur, err := CollectSourcesAt(ctx, repo, set, oid, rel, text)
	if err != nil {
		return err
	}
	for _, src := range cur.Included {
		if _, ok := base[src.Path]; !ok {
			dd.Added = append(dd.Added, src.Path)
		}
	}
	sort.Strings(dd.Added)
	dd.Status = driftStatus(dd)
	return nil
}

func hashBaseline(ctx context.Context, repo *Repo, set *Settings, oid, rel string, paths []string, base map[string]string, dd *DriftDoc) error {
	var lookup []string
	for _, p := range paths {
		if validRelPath(p) != nil || denied(p, rel, set.DocGlobs) {
			dd.Missing = append(dd.Missing, MissingSource{Path: p, Reason: "denied"})
			continue
		}
		lookup = append(lookup, p)
	}
	if len(lookup) == 0 {
		return nil
	}
	entries, err := repo.lsTreeOID(ctx, oid, lookup...)
	if err != nil {
		return err
	}
	modes := map[string]string{}
	for _, e := range entries {
		modes[e.Path] = e.Mode
	}
	for _, p := range lookup {
		switch m := modes[p]; {
		case m == "":
			dd.Missing = append(dd.Missing, MissingSource{Path: p, Reason: "deleted"})
		case !regularMode(m):
			dd.Missing = append(dd.Missing, MissingSource{Path: p, Reason: "mode_" + m})
		default:
			b, err := repo.CatFileBlob(ctx, oid, p, maxDriftBlobBytes)
			if err != nil {
				return err
			}
			if sha256Hex(b) != base[p] {
				dd.Changed = append(dd.Changed, p)
			}
		}
	}
	return nil
}

func driftStatus(dd *DriftDoc) string {
	switch {
	case len(dd.Missing) > 0:
		return DriftSourceMissing
	case len(dd.Changed) > 0:
		return DriftDrifted
	case len(dd.Added) > 0:
		return DriftSourcesDiffer
	}
	return DriftFresh
}

// Drifted reports whether a status means the sources moved.
func Drifted(status string) bool {
	return status == DriftDrifted || status == DriftSourceMissing || status == DriftSourcesDiffer
}
