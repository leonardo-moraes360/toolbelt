package egm

import (
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/secrets"
)

// MatchGlob matches a repo-relative slash path against a glob where `**`
// spans directories (also mid-path), `*` and `?` stay inside one segment.
// Used for doc_globs, doc_types and refs_ignore.
func MatchGlob(pattern, path string) bool { return secrets.MatchGlob(pattern, path) }

// MatchAny reports whether path matches any of the globs.
func MatchAny(globs []string, path string) bool { return secrets.MatchAny(globs, path) }

// GlobPrefixDir is the directory part of a glob before its first wildcard,
// without the trailing slash ("docs/specs/**" → "docs/specs").
func GlobPrefixDir(g string) string {
	i := strings.IndexAny(g, "*?[{")
	if i < 0 {
		return strings.TrimSuffix(g, "/")
	}
	j := strings.LastIndex(g[:i], "/")
	if j < 0 {
		return ""
	}
	return g[:j]
}

// IsGated reports whether rel is a gated document: it matches one of globs
// and is not a directory index (README.md), which lists documents instead of
// being one (R33, dogfood #38). An index can still be reviewed explicitly.
func IsGated(globs []string, rel string) bool {
	return !strings.EqualFold(rel[strings.LastIndex(rel, "/")+1:], "README.md") && MatchAny(globs, rel)
}
