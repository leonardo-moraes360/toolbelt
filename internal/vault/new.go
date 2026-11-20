package vault

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/leonardo-moraes360/toolbelt/internal/mdparse"
)

const maxSlugBytes = 80

// NewOptions are the inputs of tb vault new (R10).
type NewOptions struct {
	Title      string
	Type       string   // default "fact"
	Tags       []string // each must match the tag pattern
	Source     string   // default "manual"; MCP always passes "inferred"
	Confidence *float64 // required with source "inferred"
	Dir        string   // relative to the vault
	Body       string
	Now        func() time.Time // nil = time.Now
}

// NewResult is tb vault new's output.
type NewResult struct {
	Path     string `json:"path"`
	File     string `json:"file"`
	Redacted bool   `json:"redacted,omitempty"`
}

// newFrontmatter is marshalled by yaml (never interpolated), so a title can
// never add keys (R10).
type newFrontmatter struct {
	Title      string   `yaml:"title"`
	Type       string   `yaml:"type"`
	Tags       []string `yaml:"tags,omitempty"`
	Source     string   `yaml:"source"`
	Confidence *float64 `yaml:"confidence,omitempty"`
	Created    string   `yaml:"created"`
}

// New creates a note (R10): it never overwrites, never follows symlinks and
// writes only redacted text.
func (v *Vault) New(o *NewOptions) (*NewResult, error) {
	fm, err := v.newFields(o)
	if err != nil {
		return nil, err
	}
	title, body := Redact(o.Title), Redact(o.Body)
	redactedAny := title != o.Title || body != o.Body
	slug := truncateSlug(mdparse.Slugify(title))
	if slug == "" {
		return nil, errorf(ExitError, "bad_title", "the title has no letters or digits")
	}
	dir, rel, err := v.ensureDir(o.Dir)
	if err != nil {
		return nil, err
	}
	fm.Title = title
	content, err := noteContent(fm, title, body)
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > v.Cfg.MaxNoteBytes {
		return nil, errorf(ExitError, "note_too_large", "the note would be larger than vault.max_note_bytes (%d)", v.Cfg.MaxNoteBytes)
	}
	notePath := path.Join(rel, slug)
	file := filepath.Join(dir, slug+".md")
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL|openFlags, 0o644)
	if errors.Is(err, os.ErrExist) {
		return nil, errorf(ExitFail, "note_exists", "%s already exists", path.Join(v.Rel, notePath)+".md")
	}
	if err != nil {
		return nil, err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return &NewResult{Path: notePath, File: path.Join(v.Rel, notePath) + ".md", Redacted: redactedAny}, nil
}

func (v *Vault) newFields(o *NewOptions) (*newFrontmatter, error) {
	if strings.TrimSpace(o.Title) == "" || strings.IndexFunc(o.Title, unicode.IsControl) >= 0 {
		return nil, errorf(ExitError, "bad_title", "the title must be non-empty and have no control characters (newlines included)")
	}
	fm := &newFrontmatter{Type: o.Type, Source: o.Source, Confidence: o.Confidence}
	if fm.Type == "" {
		fm.Type = "fact"
	}
	if fm.Source == "" {
		fm.Source = SourceManual
	}
	if !contains(v.Cfg.Types, fm.Type) {
		return nil, errorf(ExitError, "usage", "--type must be one of vault.types (%s)", strings.Join(v.Cfg.Types, ", "))
	}
	if err := checkProvenance(fm.Source, fm.Confidence); err != nil {
		return nil, err
	}
	for _, t := range o.Tags {
		if !reTag.MatchString(t) {
			return nil, errorf(ExitError, "usage", "tag %q must match %s", Redact(t), reTag)
		}
	}
	fm.Tags = o.Tags
	now := time.Now
	if o.Now != nil {
		now = o.Now
	}
	fm.Created = now().UTC().Format(time.RFC3339)
	return fm, nil
}

func checkProvenance(source string, confidence *float64) error {
	switch {
	case source != SourceManual && source != SourceExtracted && source != SourceInferred:
		return errorf(ExitError, "usage", "--source must be manual, extracted or inferred")
	case source == SourceInferred && confidence == nil:
		return errorf(ExitError, "usage", "source inferred needs --confidence")
	case source != SourceInferred && confidence != nil:
		return errorf(ExitError, "usage", "--confidence only goes with source inferred")
	case confidence != nil && (*confidence < 0 || *confidence > 1):
		return errorf(ExitError, "usage", "--confidence must be in [0, 1]")
	}
	return nil
}

func noteContent(fm *newFrontmatter, title, body string) (string, error) {
	y, err := yaml.Marshal(fm)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.Write(y)
	b.WriteString("---\n# ")
	b.WriteString(title)
	b.WriteString("\n")
	if body = strings.TrimRight(body, "\n"); body != "" {
		b.WriteString("\n")
		b.WriteString(body)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// truncateSlug cuts s to maxSlugBytes at a rune boundary, without a
// trailing "-".
func truncateSlug(s string) string {
	if len(s) <= maxSlugBytes {
		return s
	}
	cut := maxSlugBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return strings.TrimRight(s[:cut], "-")
}

// ensureDir creates --dir under the vault one component at a time, refusing
// symlinks, dot components and anything resolving outside the vault (R10).
func (v *Vault) ensureDir(sub string) (abs, rel string, err error) {
	clean, err := cleanDir(sub)
	if err != nil || clean == "" {
		return v.Dir, "", err
	}
	cur := v.Dir
	for _, part := range strings.Split(clean, "/") {
		if cur, err = mkdirNoFollow(cur, part); err != nil {
			return "", "", err
		}
	}
	resolved, err := filepath.EvalSymlinks(cur)
	if err != nil {
		return "", "", err
	}
	if r, err := filepath.Rel(v.Dir, resolved); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", "", errorf(ExitError, "dir_outside_vault", "--dir resolves outside the vault")
	}
	return cur, clean, nil
}

// cleanDir validates --dir: relative, inside the vault, no dot or control
// characters in a component; "" is the vault itself.
func cleanDir(sub string) (string, error) {
	if sub == "" {
		return "", nil
	}
	clean := filepath.ToSlash(filepath.Clean(sub))
	if filepath.IsAbs(sub) || clean == ".." || strings.HasPrefix(clean, "../") || strings.ContainsAny(sub, "\\") {
		return "", errorf(ExitError, "dir_outside_vault", "--dir must be a relative path inside the vault")
	}
	if clean == "." {
		return "", nil
	}
	for _, part := range strings.Split(clean, "/") {
		if strings.HasPrefix(part, ".") || strings.IndexFunc(part, unicode.IsControl) >= 0 {
			return "", errorf(ExitError, "dir_outside_vault", "--dir components cannot start with a dot or hold control characters")
		}
	}
	return clean, nil
}

// mkdirNoFollow creates parent/part when missing and refuses a part that
// exists but is not a real directory (a symlink or a file).
func mkdirNoFollow(parent, part string) (string, error) {
	cur := filepath.Join(parent, part)
	fi, err := os.Lstat(cur)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return cur, os.Mkdir(cur, 0o755)
	case err != nil:
		return "", err
	case !fi.IsDir():
		return "", errorf(ExitError, "dir_outside_vault", "%s is not a directory (symlink or file)", Redact(part))
	}
	return cur, nil
}
