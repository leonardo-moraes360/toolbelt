package egm

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// GateOptions are the inputs of the egm op of tb stack (R33).
type GateOptions struct {
	RepoDir    string // absolute
	Base, Head string // the stack's refs; the range is merge-base(base, head)..head
	Docs       []string
	Strict     bool
	ConfigFile string
	// Mode is local (default) or ci (docs/specs/egm-trust.md, T-R2); ci
	// implies Strict and needs ConfigFile.
	Mode string
	// AllowOffline turns ci_offline into a deterministic-only round capped
	// at PASS_PARTIAL (T-R4).
	AllowOffline bool
}

// GateDoc is one evaluated document.
type GateDoc struct {
	Doc        string   `json:"doc"`
	Verdict    string   `json:"verdict"`
	OK         bool     `json:"ok"`
	Warn       bool     `json:"warn,omitempty"`
	ExitCode   int      `json:"exit_code"`
	Downgrades []string `json:"downgrades,omitempty"`
	Reason     string   `json:"reason,omitempty"`
	// CIFindings are the fresh round's findings in ci mode (T-R3).
	CIFindings []CIFinding `json:"ci_findings,omitempty"`
}

// GateWarning is a stack-level warning (no_gated_docs, sources_changed,
// uncommitted_gated_docs).
type GateWarning struct {
	Code string `json:"code"`
	Doc  string `json:"doc,omitempty"`
	Path string `json:"path,omitempty"`
}

// GateResult is the egm step's result.
type GateResult struct {
	OK       bool          `json:"ok"`
	Warn     bool          `json:"warn"`
	Trust    string        `json:"trust"` // local or ci (T-R8)
	Head     string        `json:"head"`
	Docs     []GateDoc     `json:"docs"`
	Warnings []GateWarning `json:"warnings"`
}

// Gate is R33: it selects the gated documents added or modified in
// merge-base(base, head)..head, plus opts.Docs, and evaluates each with
// status --at head. In ci mode it checks receipts and runs a fresh round
// instead (docs/specs/egm-trust.md).
func Gate(ctx context.Context, o *GateOptions) (*GateResult, error) {
	if err := ciPreflight(o); err != nil {
		return nil, err
	}
	ci := o.Mode == GateModeCI
	if ci {
		c := *o
		c.Strict = true
		o = &c
	}
	sel, err := gateSelect(ctx, o)
	if err != nil {
		return nil, err
	}
	repo, head, docs := sel.repo, sel.head, sel.docs
	res := &GateResult{OK: true, Trust: GateModeLocal, Head: head, Docs: []GateDoc{}, Warnings: []GateWarning{}}
	if ci {
		res.Trust = GateModeCI
	}
	res.Warnings = append(res.Warnings, sourcesChanged(repo, docs, sel.changed)...)
	for _, p := range sel.removed {
		// A gated path deleted or renamed out of the gate (T-R2).
		res.Warnings = append(res.Warnings, GateWarning{Code: "gated_doc_removed", Path: p})
		res.OK = res.OK && !ci
	}
	switch {
	case len(docs) == 0:
		return res, emptySelection(ctx, repo, head, sel.set.GateDocGlobs, o.Strict, res)
	case ci:
		return res, gateCI(ctx, repo, head, sel.set, o, docs, res)
	}
	for _, d := range docs {
		gd := gateDoc(ctx, repo.Root, d, head, o)
		res.OK = res.OK && gd.OK
		res.Warn = res.Warn || gd.Warn
		res.Docs = append(res.Docs, gd)
		if code := localReceipt(ctx, repo.Root, d, head, o.ConfigFile); code != "" {
			res.Warnings = append(res.Warnings, GateWarning{Code: code, Doc: d})
		}
	}
	return res, nil
}

// gateSelection is R33's selection: the repository, head, settings at head,
// the selected documents, every changed path and the removed gated paths.
type gateSelection struct {
	repo    *Repo
	head    string
	set     *Settings
	docs    []string
	changed map[string]bool
	removed []string
}

func gateSelect(ctx context.Context, o *GateOptions) (*gateSelection, error) {
	repo, err := DiscoverRepo(filepath.Join(o.RepoDir, ".egm-gate"))
	if err != nil {
		return nil, err
	}
	head, err := repo.RevParse(ctx, o.Head)
	if err != nil {
		return nil, err
	}
	mb, err := repo.MergeBase(ctx, o.Base, head)
	if err != nil {
		return nil, err
	}
	set, err := gateSettings(ctx, repo, head, o.ConfigFile)
	if err != nil {
		return nil, err
	}
	diff, err := repo.DiffNameStatus(ctx, mb, head)
	if err != nil {
		return nil, err
	}
	sel := &gateSelection{repo: repo, head: head, set: set}
	sel.docs, sel.changed, sel.removed = selectGated(diff, set.GateDocGlobs)
	if sel.removed, err = movedInGate(ctx, repo, mb, head, sel.removed, sel.docs); err != nil {
		return nil, err
	}
	cwd, err := filepath.EvalSymlinks(o.RepoDir)
	if err != nil {
		return nil, errorf(ExitError, "usage", "%s: %v", o.RepoDir, err)
	}
	for _, d := range o.Docs {
		rel, err := gateRel(repo.Root, cwd, d)
		if err != nil {
			return nil, err
		}
		sel.docs = appendNew(sel.docs, []string{rel})
	}
	sort.Strings(sel.docs)
	return sel, nil
}

// gateCI evaluates docs in ci mode (T-R2, T-R6).
func gateCI(ctx context.Context, repo *Repo, head string, set *Settings, o *GateOptions, docs []string, res *GateResult) error {
	if len(docs) > set.CI.MaxDocs {
		res.OK = false
		res.Warnings = append(res.Warnings, GateWarning{Code: "ci_budget", Path: fmt.Sprintf("%d documents > egm.ci.max_docs %d", len(docs), set.CI.MaxDocs)})
		return nil
	}
	offline, err := ciEngine(set)
	if err != nil {
		return err
	}
	c := &ciRun{o: o, root: repo.Root, head: head, offline: offline, runsLeft: set.MaxAgentRuns}
	for _, d := range docs {
		gd := c.doc(ctx, d)
		res.OK = res.OK && gd.OK
		res.Warn = res.Warn || gd.Warn
		res.Docs = append(res.Docs, gd)
	}
	return nil
}

// localReceipt is T-R8: in local mode a missing, invalid or stale receipt
// is a warning, never a failure.
func localReceipt(ctx context.Context, root, rel, head, cfg string) string {
	_, _, code, err := receiptCheck(ctx, root, rel, head, cfg)
	switch {
	case err != nil:
		return ""
	case code == "no_receipt":
		return "receipt_missing"
	case code != "":
		return code
	}
	return ""
}

func gateSettings(ctx context.Context, repo *Repo, head, cfgFile string) (*Settings, error) {
	user, err := LoadUserConfig(cfgFile)
	if err != nil {
		return nil, errorf(ExitError, "config", "%v", err)
	}
	raw, _, err := readRepoConfigAt(ctx, repo, head)
	if err != nil {
		return nil, err
	}
	set := ApplyRepoConfig(&user, raw)
	return &set, nil
}

// selectGated returns the added or modified gated documents, every changed
// path of the range, and the gated paths deleted in it (--no-renames: a
// rename out of the gate is a deletion).
func selectGated(diff []NameStatus, globs []string) (docs []string, changed map[string]bool, removed []string) {
	changed = map[string]bool{}
	for _, e := range diff {
		changed[e.Path] = true
		switch {
		case !IsGated(globs, e.Path):
		case strings.HasPrefix(e.Status, "A") || strings.HasPrefix(e.Status, "M") || strings.HasPrefix(e.Status, "T"):
			docs = append(docs, e.Path)
		case strings.HasPrefix(e.Status, "D"):
			removed = append(removed, e.Path)
		}
	}
	return docs, changed, removed
}

func gateDoc(ctx context.Context, root, doc, head string, o *GateOptions) GateDoc {
	out, err := Status(ctx, &Options{Doc: doc, Cwd: root, ConfigFile: o.ConfigFile, At: head, Strict: o.Strict})
	if err != nil {
		return GateDoc{Doc: doc, Verdict: "ERROR", ExitCode: ExitError, Reason: err.Error()}
	}
	r := out.Report
	gd := GateDoc{Doc: doc, Verdict: r.Verdict, ExitCode: out.ExitCode, Downgrades: r.Downgrades}
	switch {
	case r.Verdict == VerdictPass:
		gd.OK = true
	case r.Verdict == VerdictPassPartial && out.ExitCode == ExitOK:
		gd.OK, gd.Warn = true, true
	}
	for i := range r.Findings {
		if r.Findings[i].Downgrade == "forced_blocker" {
			gd.OK, gd.Reason = false, "forced_blocker: "+r.Findings[i].ID
		}
	}
	return gd
}

// sourcesChanged warns about documents with state, not selected, whose
// recorded sources changed in the range (drift detection is Phase B).
func sourcesChanged(repo *Repo, selected []string, changed map[string]bool) []GateWarning {
	root, err := StateRoot()
	if err != nil {
		return nil
	}
	dirs, err := os.ReadDir(filepath.Join(root, "egm", repo.ID))
	if err != nil {
		return nil
	}
	var out []GateWarning
	for _, d := range dirs {
		st, err := loadState(filepath.Join(root, "egm", repo.ID, d.Name()))
		if err != nil || st == nil || containsStr(selected, st.Doc) {
			continue
		}
		last := st.lastRound()
		if last == nil {
			continue
		}
		paths := make([]string, 0, len(last.Sources))
		for p := range last.Sources {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			if changed[p] {
				out = append(out, GateWarning{Code: "sources_changed", Doc: st.Doc, Path: p})
			}
		}
	}
	return out
}

// emptySelection is R33's empty case: warn (or fail with strict), and fail
// when the worktree has gated documents that differ from head.
func emptySelection(ctx context.Context, repo *Repo, head string, globs []string, strict bool, res *GateResult) error {
	res.Warn = true
	res.Warnings = append(res.Warnings, GateWarning{Code: "no_gated_docs"})
	if strict {
		res.OK = false
	}
	dirty, err := uncommittedGated(ctx, repo, head, globs)
	if err != nil {
		return err
	}
	for _, p := range dirty {
		res.OK = false
		res.Warnings = append(res.Warnings, GateWarning{Code: "uncommitted_gated_docs", Path: p})
	}
	return nil
}

// globPrefix is the directory part of a glob before its first wildcard;
// "." for a root-level glob. A literal glob is its own prefix.
func globPrefix(g string) string {
	i := strings.IndexAny(g, "*?[")
	if i < 0 {
		return path.Clean(g)
	}
	d := g[:i]
	j := strings.LastIndex(d, "/")
	if j <= 0 {
		return "."
	}
	return path.Clean(d[:j])
}

// uncommittedGated lists gated documents of the worktree that are
// unversioned or whose bytes differ from head's blob, looking only in the
// prefix directories of the globs (R33).
func uncommittedGated(ctx context.Context, repo *Repo, head string, globs []string) ([]string, error) {
	dirs := globDirs(globs)
	if len(dirs) == 0 {
		return nil, nil
	}
	others, err := repo.LsFilesOthers(ctx, dirs...)
	if err != nil {
		return nil, err
	}
	var dirty []string
	for _, p := range others {
		if IsGated(globs, p) {
			dirty = append(dirty, p)
		}
	}
	changed, err := changedFromHead(ctx, repo, head, globs, dirs)
	if err != nil {
		return nil, err
	}
	dirty = append(dirty, changed...)
	sort.Strings(dirty)
	return dirty, nil
}

func globDirs(globs []string) []string {
	var dirs []string
	for _, g := range globs {
		// A prefix git would refuse cannot come from a validated glob; skip
		// it rather than fail the whole step.
		if p := globPrefix(g); validRelPath(p) == nil && !containsStr(dirs, p) {
			dirs = append(dirs, p)
		}
	}
	return dirs
}

// changedFromHead lists the indexed gated documents present in the worktree
// whose bytes (hash-object --no-filters) differ from head's blob.
func changedFromHead(ctx context.Context, repo *Repo, head string, globs, dirs []string) ([]string, error) {
	staged, err := repo.LsFilesStage(ctx, dirs...)
	if err != nil {
		return nil, err
	}
	var present, out []string
	for _, e := range staged {
		if !IsGated(globs, e.Path) || containsStr(present, e.Path) || containsStr(out, e.Path) {
			continue
		}
		// Deleted, or replaced by a symlink or a non-regular file: a
		// worktree change against head all the same.
		if fi, why, missing := lstatChain(repo.Root, e.Path); missing || why != "" || !fi.Mode().IsRegular() {
			out = append(out, e.Path)
			continue
		}
		present = append(present, e.Path)
	}
	if len(present) == 0 {
		return out, nil
	}
	hashes, err := repo.HashObject(ctx, present...)
	if err != nil {
		return nil, err
	}
	tree, err := repo.LsTree(ctx, head, present...)
	if err != nil {
		return nil, err
	}
	at := map[string]string{}
	for _, e := range tree {
		at[e.Path] = e.Object
	}
	for i, p := range present {
		if at[p] != hashes[i] {
			out = append(out, p)
		}
	}
	return out, nil
}

// movedInGate drops the removed gated paths whose blob at the merge-base is
// the blob of a selected document at head: a rename inside the gate (the
// diff has --no-renames). A rename with an edit still counts as removed.
func movedInGate(ctx context.Context, repo *Repo, mb, head string, removed, docs []string) ([]string, error) {
	if len(removed) == 0 || len(docs) == 0 {
		return removed, nil
	}
	old, err := repo.LsTree(ctx, mb, removed...)
	if err != nil {
		return nil, err
	}
	cur, err := repo.LsTree(ctx, head, docs...)
	if err != nil {
		return nil, err
	}
	added := map[string]bool{}
	for _, e := range cur {
		added[e.Object] = true
	}
	moved := map[string]bool{}
	for _, e := range old {
		moved[e.Path] = added[e.Object]
	}
	var out []string
	for _, p := range removed {
		if !moved[p] {
			out = append(out, p)
		}
	}
	return out, nil
}
