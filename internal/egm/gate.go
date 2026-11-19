package egm

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

// Gate mode (R7a): with --at <rev>, the document, the sources, the
// repository config and the checkers' inputs come from the commit's tree,
// never from the worktree or the recorded paths.

const (
	modeTree    = "040000"
	modeSymlink = "120000"
)

func regularMode(m string) bool { return m == "100644" || m == "100755" }

// treeIndex is indexView over a commit tree.
type treeIndex struct {
	repo *Repo
	oid  string
}

func (t treeIndex) exists(ctx context.Context, paths []string) (map[string]string, error) {
	out := map[string]string{}
	if len(paths) == 0 {
		return out, nil
	}
	entries, err := t.repo.LsTree(ctx, t.oid, paths...)
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		for _, e := range entries {
			if e.Path != p {
				continue
			}
			// A tree entry only exists with versioned files below it, the
			// same "dir" as the index view.
			out[p] = "file"
			if e.Mode == modeTree {
				out[p] = "dir"
			}
		}
	}
	return out, nil
}

func (t treeIndex) under(ctx context.Context, dir string, limit int) ([]string, error) {
	return t.repo.LsTreeNames(ctx, t.oid, dir, limit)
}

// symlinked lists every prefix of rel in one ls-tree call: a 120000 entry on
// the way means the path crosses a versioned symlink.
func (t treeIndex) symlinked(ctx context.Context, rel string) (bool, error) {
	parts := strings.Split(rel, "/")
	prefixes := make([]string, 0, len(parts))
	for i := range parts {
		prefixes = append(prefixes, strings.Join(parts[:i+1], "/"))
	}
	entries, err := t.repo.LsTree(ctx, t.oid, prefixes...)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.Mode == modeSymlink {
			return true, nil
		}
	}
	return false, nil
}

// gateDocPath resolves the document path for gate mode: the document may be
// absent from the worktree, so only its directory goes through EvalSymlinks.
func gateDocPath(cwd, doc string) (string, error) {
	p := doc
	if !filepath.IsAbs(p) {
		if !filepath.IsAbs(cwd) {
			return "", errorf(ExitError, "usage", "cwd must be absolute")
		}
		p = filepath.Join(cwd, p)
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		return "", errorf(ExitError, "usage", "document %s: %v", doc, err)
	}
	return filepath.Join(dir, filepath.Base(p)), nil
}

// readBlobFile reads a regular file of the tree at oid, up to limit bytes.
// ok=false means absent, not regular or behind a symlink.
func readBlobFile(ctx context.Context, repo *Repo, oid, rel string, limit int64) (b []byte, mode string, err error) {
	entries, err := repo.LsTree(ctx, oid, rel)
	if err != nil {
		return nil, "", err
	}
	for _, e := range entries {
		if e.Path != rel {
			continue
		}
		if !regularMode(e.Mode) {
			return nil, e.Mode, nil
		}
		b, err = repo.CatFileBlob(ctx, oid, rel, limit)
		return b, e.Mode, err
	}
	return nil, "", nil
}

// readRepoConfigAt is readRepoConfig over the commit's blob (R34, gate mode).
func readRepoConfigAt(ctx context.Context, repo *Repo, oid string) ([]byte, *Warning, error) {
	const rel = ".toolbelt/config.yaml"
	b, mode, err := readBlobFile(ctx, repo, oid, rel, maxRepoConfigBytes)
	var e *Error
	if errors.As(err, &e) && e.Reason == "blob_too_large" {
		return nil, &Warning{Code: "config_untrusted", Message: "larger than 64 KiB: ignored", Path: rel}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if mode != "" && !regularMode(mode) {
		return nil, &Warning{Code: "config_untrusted", Message: "not a regular file: ignored", Path: rel}, nil
	}
	return b, nil, nil
}

// readDocAt reads the document from the commit (R7a).
func (s *snapshot) readDocAt(ctx context.Context, docPath string) error {
	if err := s.setRel(docPath); err != nil {
		return err
	}
	b, mode, err := readBlobFile(ctx, s.repo, s.at, s.rel, s.set.MaxPacketBytes)
	var e *Error
	switch {
	case errors.As(err, &e) && e.Reason == "blob_too_large":
		return errorf(ExitError, "max_packet_bytes", "document %s at %s is larger than egm.max_packet_bytes (%d)", s.rel, s.target, s.set.MaxPacketBytes)
	case err != nil:
		return err
	case mode == "":
		return errorf(ExitError, "usage", "document %s does not exist at %s", s.rel, s.target)
	case b == nil:
		return errorf(ExitError, "usage", "document %s at %s is not a regular file (mode %s)", s.rel, s.target, mode)
	}
	s.setDoc(b)
	return nil
}

// CollectSourcesAt is CollectSources over the commit's tree (R7a): the same
// candidates and exclusions; the mode comes from ls-tree and the bytes from
// cat-file blob. ignore rules do not apply to versioned files.
func CollectSourcesAt(ctx context.Context, repo *Repo, set *Settings, oid, docRel, docText string) (*SourceSet, error) {
	c := &sourceCtx{repo: repo, set: set, doc: docRel, out: &SourceSet{}, used: int64(len(docText))}
	rels, skipped := sourceCandidates(docRel, docText)
	c.out.Skipped = append(c.out.Skipped, skipped...)
	var files []string
	for _, rel := range rels {
		if denied(rel, docRel, set.DocGlobs) {
			c.skip(rel, "excluded")
			continue
		}
		files = append(files, rel)
	}
	if len(files) == 0 {
		return c.finish(), nil
	}
	entries, err := repo.LsTree(ctx, oid, files...)
	if err != nil {
		return nil, err
	}
	modes := map[string]string{}
	for _, e := range entries {
		modes[e.Path] = e.Mode
	}
	for _, f := range files {
		switch m := modes[f]; {
		case m == "":
			// Absent, or behind a non-tree component such as a symlink.
			c.skip(f, "missing")
		case !regularMode(m):
			c.skip(f, "mode_"+m)
		default:
			if err := c.includeBlob(ctx, oid, f); err != nil {
				return nil, err
			}
		}
	}
	return c.finish(), nil
}

func (c *sourceCtx) includeBlob(ctx context.Context, oid, f string) error {
	b, err := c.repo.CatFileBlob(ctx, oid, f, c.set.MaxSourceBytes)
	var e *Error
	if errors.As(err, &e) && e.Reason == "blob_too_large" {
		c.skip(f, "max_source_bytes")
		c.out.MaxExcluded = true
		return nil
	}
	if err != nil {
		return err
	}
	c.addBytes(f, b)
	return nil
}
