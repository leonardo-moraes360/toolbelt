package egm

import (
	"context"
	"os"
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
	Head     string        `json:"head"`
	Docs     []GateDoc     `json:"docs"`
	Warnings []GateWarning `json:"warnings"`
}

// Gate is R33: it selects the gated documents added or modified in
// merge-base(base, head)..head, plus opts.Docs, and evaluates each with
// status --at head.
func Gate(ctx context.Context, o *GateOptions) (*GateResult, error) {
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
	docs, changed := selectGated(diff, set.GateDocGlobs)
	for _, d := range o.Docs {
		rel, err := gateRel(repo.Root, o.RepoDir, d)
		if err != nil {
			return nil, err
		}
		docs = appendNew(docs, []string{rel})
	}
	sort.Strings(docs)
	res := &GateResult{OK: true, Head: head, Docs: []GateDoc{}, Warnings: []GateWarning{}}
	res.Warnings = append(res.Warnings, sourcesChanged(repo, docs, changed)...)
	if len(docs) == 0 {
		return res, emptySelection(ctx, repo, head, set.GateDocGlobs, o.Strict, res)
	}
	for _, d := range docs {
		gd := gateDoc(ctx, repo.Root, d, head, o)
		res.OK = res.OK && gd.OK
		res.Warn = res.Warn || gd.Warn
		res.Docs = append(res.Docs, gd)
	}
	return res, nil
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

// selectGated returns the added or modified gated documents and every
// changed path of the range.
func selectGated(diff []NameStatus, globs []string) (docs []string, changed map[string]bool) {
	changed = map[string]bool{}
	for _, e := range diff {
		changed[e.Path] = true
		if (strings.HasPrefix(e.Status, "A") || strings.HasPrefix(e.Status, "M") || strings.HasPrefix(e.Status, "T")) && MatchAny(globs, e.Path) {
			docs = append(docs, e.Path)
		}
	}
	return docs, changed
}

func gateRel(root, cwd, doc string) (string, error) {
	p, err := gateDocPath(cwd, doc)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errorf(ExitError, "usage", "--egm-doc %s is outside the repository", doc)
	}
	return filepath.ToSlash(rel), nil
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

// globPrefix is the directory part of a glob before its first "*".
func globPrefix(g string) string {
	i := strings.IndexAny(g, "*?[")
	if i < 0 {
		return g
	}
	d := g[:i]
	if j := strings.LastIndex(d, "/"); j >= 0 {
		return d[:j]
	}
	return ""
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
		if MatchAny(globs, p) {
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
		if p := globPrefix(g); p != "" && !containsStr(dirs, p) {
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
	var present []string
	for _, e := range staged {
		if !MatchAny(globs, e.Path) || containsStr(present, e.Path) {
			continue
		}
		if fi, why, missing := lstatChain(repo.Root, e.Path); !missing && why == "" && fi.Mode().IsRegular() {
			present = append(present, e.Path)
		}
	}
	if len(present) == 0 {
		return nil, nil
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
	var out []string
	for i, p := range present {
		if at[p] != hashes[i] {
			out = append(out, p)
		}
	}
	return out, nil
}
