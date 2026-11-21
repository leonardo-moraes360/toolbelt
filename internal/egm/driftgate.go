package egm

import (
	"context"
	"path"
	"sort"
	"strings"
)

// Drift in the gate (docs/specs/egm-drift-bootstrap.md, B1): receipt edits
// select their document, a removed receipt fails in CI, ci_source_drift
// flags drift the range introduced against the merge-base receipt, and the
// local sources_changed warning comes from receipts too.

// driftGate holds what the drift checks need from the gate selection.
type driftGate struct {
	repo    *Repo
	mb      string
	head    string
	set     *Settings
	cfg     string // the user config file (-c), as the gate's
	changed map[string]bool
	diff    []NameStatus
}

// receiptEdits returns the gated documents (at head) whose receipt was added
// or modified in the range, and those whose receipt was deleted while the
// document stays gated and had a receipt at the merge-base.
func (g *driftGate) receiptEdits(ctx context.Context) (edited, removed []string, err error) {
	byReceipt, err := g.receiptOwners(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range g.diff {
		doc, ok := byReceipt[e.Path]
		if !ok {
			continue
		}
		switch {
		case strings.HasPrefix(e.Status, "D"):
			removed = appendNew(removed, []string{doc})
		default:
			edited = appendNew(edited, []string{doc})
		}
	}
	sort.Strings(edited)
	sort.Strings(removed)
	return edited, removed, nil
}

// receiptOwners maps the receipt path of every gated document at head to
// the document.
func (g *driftGate) receiptOwners(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	touched := false
	for _, e := range g.diff {
		if path.Dir(e.Path) == receiptDir && strings.HasSuffix(e.Path, ".json") {
			touched = true
			break
		}
	}
	if !touched {
		return out, nil
	}
	docs, err := gatedAt(ctx, g.repo, g.head, g.set.GateDocGlobs)
	if err != nil {
		return nil, err
	}
	for _, d := range docs {
		out[ReceiptPath(d)] = d
	}
	return out, nil
}

// sourceDrift is ci_source_drift: for each gated document the range did not
// touch, fresh against its merge-base receipt at the merge-base, the
// comparison runs at head against that same receipt.
func (g *driftGate) sourceDrift(ctx context.Context, selected []string) ([]DriftDoc, error) {
	docs, err := gatedAt(ctx, g.repo, g.head, g.set.GateDocGlobs)
	if err != nil {
		return nil, err
	}
	// The comparison uses the merge-base settings: the pull request's own
	// config must not lower limits to hide drift.
	mbSet, err := gateSettings(ctx, g.repo, g.mb, g.cfg)
	if err != nil {
		return nil, err
	}
	var out []DriftDoc
	for _, d := range docs {
		if containsStr(selected, d) || g.changed[d] {
			continue
		}
		dd, ok, err := g.driftSinceMB(ctx, mbSet, d)
		if err != nil {
			return nil, err
		}
		if ok && Drifted(dd.Status) {
			out = append(out, dd)
		}
	}
	return out, nil
}

// driftSinceMB compares d at head with its merge-base receipt, using the
// merge-base settings throughout; ok is false when the document has no
// usable merge-base baseline, already drifted there, or nothing it cites
// (or its receipt holds) changed in the range.
func (g *driftGate) driftSinceMB(ctx context.Context, mbSet *Settings, d string) (dd DriftDoc, ok bool, err error) {
	text, sha, readable, err := docAt(ctx, g.repo, mbSet, g.mb, d)
	if err != nil || !readable {
		return dd, false, err
	}
	rc, code, err := readReceiptAt(ctx, g.repo, g.mb, d, docLines(text))
	if err != nil || code != "" || rc == nil || rc.DocSHA256 != sha || !g.touches(d, text, rc.Sources) {
		return dd, false, err
	}
	// Fresh at the merge-base: drift already there is not this range's.
	at := newDriftDoc(d, BaselineReceipt)
	if err := compareSources(ctx, g.repo, mbSet, g.mb, d, text, rc.Sources, &at); err != nil || at.Status != DriftFresh {
		return dd, false, err
	}
	// Not touched in the range, so the document bytes at head are the same.
	dd = newDriftDoc(d, BaselineReceipt)
	if err := compareSources(ctx, g.repo, mbSet, g.head, d, text, rc.Sources, &dd); err != nil {
		return dd, false, err
	}
	return dd, true, nil
}

func newDriftDoc(doc, base string) DriftDoc {
	return DriftDoc{Doc: doc, Baseline: base, Changed: []string{}, Missing: []MissingSource{}, Added: []string{}}
}

// touches: a source candidate of the document, or a path its receipt
// certified, changed in the range.
func (g *driftGate) touches(doc, text string, sources map[string]string) bool {
	for p := range sources {
		if g.changed[p] {
			return true
		}
	}
	return g.citesChanged(doc, text)
}

// citesChanged: some source candidate of the document changed in the range
// (a new source is always a changed path too).
func (g *driftGate) citesChanged(doc, text string) bool {
	rels, _ := sourceCandidates(doc, text)
	for _, r := range rels {
		if g.changed[r] {
			return true
		}
	}
	return false
}

// localSourcesChanged is the local gate's sources_changed warning: for
// unselected gated documents citing a changed path, the paths of their
// drift (receipt baseline, else a converged round) that changed in range.
func (g *driftGate) localSourcesChanged(ctx context.Context, selected []string) []GateWarning {
	docs, err := gatedAt(ctx, g.repo, g.head, g.set.GateDocGlobs)
	if err != nil {
		return []GateWarning{{Code: "drift_unavailable", Path: err.Error()}}
	}
	var out []GateWarning
	for _, d := range docs {
		if containsStr(selected, d) {
			continue
		}
		text, _, ok, err := docAt(ctx, g.repo, g.set, g.head, d)
		if err != nil {
			out = append(out, GateWarning{Code: "drift_unavailable", Doc: d, Path: err.Error()})
			continue
		}
		if !ok || !g.citesChanged(d, text) {
			continue
		}
		dd, err := driftOf(ctx, g.repo, g.set, g.head, d, true)
		if err != nil {
			out = append(out, GateWarning{Code: "drift_unavailable", Doc: d, Path: err.Error()})
			continue
		}
		var paths []string
		paths = append(paths, dd.Changed...)
		paths = append(paths, dd.Added...)
		for _, m := range dd.Missing {
			paths = append(paths, m.Path)
		}
		sort.Strings(paths)
		for _, p := range paths {
			if g.changed[p] {
				out = append(out, GateWarning{Code: "sources_changed", Doc: d, Path: p})
			}
		}
	}
	return out
}

// applyDrift adds the B1 checks to a gate result: receipt edits select
// their document (ci), a removed receipt warns or fails, and in ci mode
// ci_source_drift is raised at egm.drift.ci's severity.
func (g *driftGate) applyDrift(ctx context.Context, sel *gateSelection, ci bool, res *GateResult) error {
	edited, removed, err := g.receiptEdits(ctx)
	if err != nil {
		return err
	}
	for _, d := range removed {
		hadReceipt, err := g.receiptAt(ctx, g.mb, d)
		if err != nil {
			return err
		}
		if hadReceipt && !containsStr(sel.removed, d) {
			res.Warnings = append(res.Warnings, GateWarning{Code: "receipt_removed", Doc: d, Path: ReceiptPath(d)})
			res.OK = res.OK && !ci
			res.Warn = true
		}
	}
	if ci {
		sel.docs = appendNew(sel.docs, edited)
		sort.Strings(sel.docs)
		drifted, err := g.sourceDrift(ctx, sel.docs)
		if err != nil {
			return err
		}
		for i := range drifted {
			res.Warnings = append(res.Warnings, driftWarning(&drifted[i]))
			if g.set.Drift.CI == DriftFail {
				res.OK = false
			}
			res.Warn = true
		}
		return nil
	}
	res.Warnings = append(res.Warnings, g.localSourcesChanged(ctx, sel.docs)...)
	return nil
}

func driftWarning(dd *DriftDoc) GateWarning {
	w := GateWarning{Code: "ci_source_drift", Doc: dd.Doc}
	switch {
	case len(dd.Missing) > 0:
		w.Path = dd.Missing[0].Path
	case len(dd.Changed) > 0:
		w.Path = dd.Changed[0]
	case len(dd.Added) > 0:
		w.Path = dd.Added[0]
	}
	return w
}

// receiptAt reports whether d had a receipt blob at oid.
func (g *driftGate) receiptAt(ctx context.Context, oid, d string) (bool, error) {
	_, mode, err := readBlobFile(ctx, g.repo, oid, ReceiptPath(d), maxReceiptBytes)
	if err != nil {
		return false, err
	}
	return mode != "", nil
}
