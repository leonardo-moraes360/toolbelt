package egm

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var reSlugBad = regexp.MustCompile(`[^a-z0-9-]`)

// Slug identifies a document by its repo-relative slash path (R14). A renamed
// document is a new document.
func Slug(rel string) string {
	s := strings.ToLower(rel)
	s = strings.ReplaceAll(s, "/", "--")
	s = reSlugBad.ReplaceAllString(s, "-")
	h := sha256.Sum256([]byte(rel))
	return s + "-" + hex.EncodeToString(h[:4])
}
