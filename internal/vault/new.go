package vault

import (
	"errors"
	"math"
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
// writes only redacted text. Directory creation and the write go through
// os.Root, so no symlink swapped in meanwhile can take them outside the
// vault.
func (v *Vault) New(o *NewOptions) (*NewResult, error) {
	if int64(len(o.Title)+len(o.Body)) > v.Cfg.MaxNoteBytes {
		return nil, errorf(ExitError, "note_too_large", "title and body exceed vault.max_note_bytes (%d)", v.Cfg.MaxNoteBytes)
	}
	fm, err := v.newFields(o)
	if err != nil {
		return nil, err
	}
	rel, err := cleanDir(o.Dir)
	if err != nil {
		return nil, err
	}
	title, body := Redact(o.Title), Redact(o.Body)
	slug := safeSlug(truncateSlug(mdparse.Slugify(title)))
	if slug == "" {
		return nil, errorf(ExitError, "bad_title", "the title has no letters or digits")
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
	if err := writeNew(v.Dir, rel, slug+".md", content); err != nil {
		var e *Error
		if errors.As(err, &e) && e.Reason == "note_exists" {
			e.Msg = path.Join(v.Rel, notePath) + ".md already exists"
		}
		return nil, err
	}
	return &NewResult{Path: notePath, File: path.Join(v.Rel, notePath) + ".md", Redacted: title != o.Title || body != o.Body}, nil
}

// writeNew creates rel's directories (refusing any existing component that
// is not a real directory) and the file name with O_EXCL, all inside an
// os.Root at dir; a failed write removes the partial file.
func writeNew(dir, rel, name, content string) error {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := mkdirsNoFollow(r, rel); err != nil {
		return err
	}
	file := path.Join(rel, name)
	if rel == "" {
		file = name
	}
	f, err := r.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL|openFlags, 0o644)
	if errors.Is(err, os.ErrExist) {
		return errorf(ExitFail, "note_exists", "%s already exists", file)
	}
	if err != nil {
		return err
	}
	_, werr := f.WriteString(content)
	cerr := f.Close()
	if werr != nil || cerr != nil {
		_ = r.Remove(file)
		return errors.Join(werr, cerr)
	}
	return nil
}

// mkdirsNoFollow creates rel under r one component at a time and refuses a
// component that exists but is not a real directory (symlink or file).
func mkdirsNoFollow(r *os.Root, rel string) error {
	if rel == "" {
		return nil
	}
	cur := ""
	for _, part := range strings.Split(rel, "/") {
		cur = path.Join(cur, part)
		fi, err := r.Lstat(cur)
		switch {
		case errors.Is(err, os.ErrNotExist):
			if err := r.Mkdir(cur, 0o755); err != nil {
				return err
			}
		case err != nil:
			return err
		case !fi.IsDir():
			return errorf(ExitError, "dir_outside_vault", "%s is not a directory (symlink or file)", Redact(part))
		}
	}
	return nil
}

// EnsureDir creates rel (a validated vault.path) under root for tb init,
// one component at a time, refusing symlinked or non-directory components,
// so a cloned repository cannot make tb init create directories elsewhere.
func EnsureDir(root, rel string) error {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
	if !filepath.IsLocal(filepath.FromSlash(clean)) {
		return errorf(ExitError, "vault_path_outside_repo", "vault.path %s is not inside the repository", rel)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := mkdirsNoFollow(r, clean); err != nil {
		var e *Error
		if errors.As(err, &e) {
			e.Reason = "vault_path_outside_repo"
		}
		return err
	}
	return nil
}

// reservedNames are Windows device names: a note named con.md could map to
// a device there, so such slugs get a suffix.
var reservedNames = map[string]bool{"con": true, "prn": true, "aux": true, "nul": true}

func safeSlug(s string) string {
	if s == "" {
		return ""
	}
	if reservedNames[s] || (len(s) == 4 && (strings.HasPrefix(s, "com") || strings.HasPrefix(s, "lpt")) && s[3] >= '0' && s[3] <= '9') {
		return s + "-note"
	}
	return s
}

// badTitleRune is a control character or an invisible one that could spoof
// a title: format characters (bidi overrides, zero-width) and line or
// paragraph separators.
func badTitleRune(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp)
}

func (v *Vault) newFields(o *NewOptions) (*newFrontmatter, error) {
	if strings.TrimSpace(o.Title) == "" || strings.IndexFunc(o.Title, badTitleRune) >= 0 {
		return nil, errorf(ExitError, "bad_title", "the title must be non-empty and have no control or invisible characters (newlines included)")
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
	case confidence != nil && (math.IsNaN(*confidence) || math.IsInf(*confidence, 0) || *confidence < 0 || *confidence > 1):
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

// cleanDir validates --dir (R10): relative and local, no component that
// starts with a dot, holds a control character or a ":" (a Windows drive or
// stream), or ends with a dot or a space; "" is the vault itself.
func cleanDir(sub string) (string, error) {
	if sub == "" {
		return "", nil
	}
	clean := filepath.ToSlash(filepath.Clean(sub))
	if clean == "." {
		return "", nil
	}
	if strings.ContainsAny(sub, "\\:") || !filepath.IsLocal(filepath.FromSlash(clean)) {
		return "", errorf(ExitError, "dir_outside_vault", "--dir must be a relative path inside the vault")
	}
	for _, part := range strings.Split(clean, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") ||
			strings.IndexFunc(part, unicode.IsControl) >= 0 || safeSlug(strings.ToLower(part)) != strings.ToLower(part) {
			return "", errorf(ExitError, "dir_outside_vault", "--dir has a component that is hidden, reserved or ends with a dot or space")
		}
	}
	return clean, nil
}
