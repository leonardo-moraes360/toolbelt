package egm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/graph"
)

// Source is a resolved, included source (R7).
type Source struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Text   string `json:"-"` // redacted snapshot
}

// SkippedSource is an entry of report.json skipped_sources (R33b).
type SkippedSource struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// SourceSet is phase 1's view of the document's sources.
type SourceSet struct {
	Included []Source
	Skipped  []SkippedSource
	Warnings []Warning
	// Unstaged makes the round not clean (R7 order of the source rules).
	Unstaged bool
	// MaxExcluded is set when a source was dropped by max_* (R24).
	MaxExcluded bool
}

var (
	reSpan       = regexp.MustCompile("`([^`\n]+)`")
	reSpanExt    = regexp.MustCompile(`\.(go|py|rs|java|kt|cs|php|rb|swift|ts|tsx|js|mjs|c|h|cpp|hpp|md|mdx|html|txt|sh|json|yaml|yml|toml)$`)
	denyBaseGlob = []string{
		".env*", "*.pem", "*.key", "*.p12", "id_*", "credentials*", "secrets*", "*.tfvars", "*.tfstate*",
		"kubeconfig*", ".npmrc", ".netrc", ".pypirc", "service-account*.json", "*.keystore", "*.jks",
	}
	denyPathGlob = []string{
		".toolbelt/**", "**/testdata/**", "internal/egm/personas/**", "internal/egm/standards/**",
		"internal/egm/injection_patterns.txt", "docs/pt-BR/**",
	}
)

// SpanCandidates returns the single-backtick spans that may be paths (R7 b):
// no space, <, >, $, ~, (, *, {, … nor :, ending in a known extension or /.
func SpanCandidates(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range reSpan.FindAllStringSubmatch(text, -1) {
		s := m[1]
		if strings.ContainsAny(s, " <>$~(*{…:") || seen[s] {
			continue
		}
		if !reSpanExt.MatchString(s) && !strings.HasSuffix(s, "/") {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// cleanRel runs steps 1–4 of resolveSource: strip fragment/query, refuse an
// absolute path, join with base and Clean, refuse leaving the root. reason is
// "" when rel is usable.
func cleanRel(base, raw string) (rel, reason string) {
	if strings.Contains(raw, "://") || strings.HasPrefix(raw, "mailto:") {
		return "", "url"
	}
	if i := strings.IndexAny(raw, "#?"); i >= 0 {
		raw = raw[:i]
	}
	if raw == "" {
		return "", "empty"
	}
	if strings.HasPrefix(raw, "/") || filepath.IsAbs(raw) {
		return "", "absolute"
	}
	rel = path.Clean(path.Join(base, raw))
	if rel == ".." || strings.HasPrefix(rel, "../") || rel == "." {
		return "", "outside_root"
	}
	return rel, ""
}

// lstatChain applies Lstat to each component from the root (step 5). It
// returns the last component's info, "symlink" when any component is one,
// and missing=true when a component does not exist.
func lstatChain(root, rel string) (fi os.FileInfo, reason string, missing bool) {
	cur := root
	for _, part := range strings.Split(rel, "/") {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", true
		}
		if err != nil {
			return nil, "unreadable", false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, "symlink", false
		}
		fi = info
	}
	return fi, "", false
}

func denied(rel, doc string, gated []string) bool {
	for _, part := range strings.Split(rel, "/") {
		if part == ".git" {
			return true
		}
	}
	if MatchAny(denyPathGlob, rel) || MatchAny(denyBaseGlob, path.Base(rel)) {
		return true
	}
	return rel != doc && MatchAny(gated, rel)
}

// sourceCandidates gathers R7 (a) links relative to the document's directory
// and (b) spans relative to the root, cleaned and deduplicated.
func sourceCandidates(docRel, text string) (rels []string, skipped []SkippedSource) {
	seen := map[string]bool{docRel: true}
	add := func(base, raw string) {
		rel, why := cleanRel(base, raw)
		if why == "empty" {
			return
		}
		if why != "" {
			if !seen["!"+raw] {
				seen["!"+raw] = true
				skipped = append(skipped, SkippedSource{Path: raw, Reason: why})
			}
			return
		}
		if !seen[rel] {
			seen[rel] = true
			rels = append(rels, rel)
		}
	}
	for _, l := range graph.CollectMDLinks(text) {
		add(path.Dir(docRel), l)
	}
	for _, s := range SpanCandidates(text) {
		if !strings.HasSuffix(s, "/") {
			add("", s)
		}
	}
	return rels, skipped
}

type sourceCtx struct {
	repo *Repo
	set  *Settings
	doc  string
	out  *SourceSet
	used int64
}

func (c *sourceCtx) skip(p, reason string) {
	c.out.Skipped = append(c.out.Skipped, SkippedSource{Path: p, Reason: reason})
}

// CollectSources resolves the document's sources from the worktree (R7).
// docBytes counts toward max_packet_bytes.
func CollectSources(ctx context.Context, repo *Repo, set *Settings, docRel, docText string) (*SourceSet, error) {
	c := &sourceCtx{repo: repo, set: set, doc: docRel, out: &SourceSet{}, used: int64(len(docText))}
	rels, skipped := sourceCandidates(docRel, docText)
	c.out.Skipped = append(c.out.Skipped, skipped...)
	files := c.filterFS(rels)
	if len(files) == 0 {
		return c.finish(), nil
	}
	ignored, err := repo.CheckIgnore(ctx, files...)
	if err != nil {
		return nil, err
	}
	if err := c.checkIndex(ctx, files, ignored); err != nil {
		return nil, err
	}
	return c.finish(), nil
}

func (c *sourceCtx) filterFS(rels []string) []string {
	var files []string
	for _, rel := range rels {
		if denied(rel, c.doc, c.set.DocGlobs) {
			c.skip(rel, "excluded")
			continue
		}
		fi, why, missing := lstatChain(c.repo.Root, rel)
		switch {
		case missing:
			c.skip(rel, "missing")
		case why != "":
			c.skip(rel, why)
		case !fi.Mode().IsRegular():
			c.skip(rel, "not_regular")
		default:
			files = append(files, rel)
		}
	}
	return files
}

// checkIndex keeps the versioned sources. Ignore rules only drop unversioned
// files: a tracked file that matches .gitignore is in the commit, and gate
// mode (R7a) must see the same content.
func (c *sourceCtx) checkIndex(ctx context.Context, files []string, ignored map[string]bool) error {
	if len(files) == 0 {
		return nil
	}
	entries, err := c.repo.LsFilesStage(ctx, files...)
	if err != nil {
		return err
	}
	idx := map[string]IndexEntry{}
	for _, e := range entries {
		idx[e.Path] = e
	}
	var versioned []string
	for _, f := range files {
		e, ok := idx[f]
		switch {
		case !ok && ignored[f]:
			c.skip(f, "ignored")
		case !ok:
			c.unstaged(f, "unversioned")
		case e.Mode != "100644" && e.Mode != "100755":
			c.skip(f, "mode_"+e.Mode)
		default:
			versioned = append(versioned, f)
		}
	}
	if len(versioned) == 0 {
		return nil
	}
	hashes, err := c.repo.HashObject(ctx, versioned...)
	if err != nil {
		return err
	}
	for i, f := range versioned {
		if i < len(hashes) && hashes[i] != idx[f].Object {
			c.out.Unstaged = true
			c.out.Warnings = append(c.out.Warnings, Warning{Code: "unstaged_source", Path: f, Message: msgUnstaged})
		}
		c.include(f)
	}
	return nil
}

const msgUnstaged = "faça git add das fontes citadas antes de convergir (git add the cited sources before converging)"

func (c *sourceCtx) unstaged(f, reason string) {
	c.skip(f, reason)
	c.out.Unstaged = true
	c.out.Warnings = append(c.out.Warnings, Warning{Code: "unstaged_source", Path: f, Message: msgUnstaged})
}

func (c *sourceCtx) include(f string) {
	b, err := readRegular(filepath.Join(c.repo.Root, filepath.FromSlash(f)), c.set.MaxSourceBytes)
	if errors.Is(err, errTooBig) {
		c.skip(f, "max_source_bytes")
		c.out.MaxExcluded = true
		return
	}
	if err != nil {
		c.skip(f, "unreadable")
		return
	}
	c.addBytes(f, b)
}

// addBytes applies the redaction and packet rules to a source's bytes.
func (c *sourceCtx) addBytes(f string, b []byte) {
	red, secret := Redact(string(b))
	if secret {
		c.skip(f, "secret_like")
		c.out.Warnings = append(c.out.Warnings, Warning{Code: "secret_like", Path: f})
		return
	}
	if c.used+int64(len(b)) > c.set.MaxPacketBytes {
		c.skip(f, "max_packet_bytes")
		c.out.MaxExcluded = true
		return
	}
	c.used += int64(len(b))
	c.out.Included = append(c.out.Included, Source{Path: f, SHA256: sha256Hex(b), Text: red})
}

func (c *sourceCtx) finish() *SourceSet {
	sort.Slice(c.out.Included, func(i, j int) bool { return c.out.Included[i].Path < c.out.Included[j].Path })
	sort.SliceStable(c.out.Skipped, func(i, j int) bool { return c.out.Skipped[i].Path < c.out.Skipped[j].Path })
	return c.out
}

var errTooBig = errors.New("too big")

// readRegular reads a file that must still be regular when opened, up to max
// bytes (errTooBig above).
func readRegular(p string, maxBytes int64) ([]byte, error) {
	// Check the type before opening: open(2) on a FIFO blocks for good and
	// ignores the context (R7: the document must be a regular file).
	li, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !li.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	f, err := os.Open(p) //nolint:gosec // callers validated the path
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	if fi.Size() > maxBytes {
		return nil, errTooBig
	}
	b, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxBytes {
		return nil, errTooBig
	}
	return b, nil
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
