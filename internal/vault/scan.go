package vault

import (
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// scan is one pass over the vault (R3).
type scan struct {
	notes       []*Note
	unread      []*Note  // .md entries not parsed (too large, not regular): linkable, not listed
	attachments []string // relative to the vault, "/"
	issues      []Issue  // not_regular, note_too_large, unreadable
}

// scan walks the vault without following symlinks. Directories starting
// with "." are skipped; regular ".md" files are notes, other regular files
// attachments, anything else not_regular.
func (v *Vault) scan() (*scan, error) {
	sc := &scan{}
	err := filepath.WalkDir(v.Dir, func(p string, d fs.DirEntry, walkErr error) error {
		rel, err := filepath.Rel(v.Dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if walkErr != nil {
			if rel == "." {
				return walkErr
			}
			// An unreadable directory or entry is reported; the scan goes on.
			sc.issues = append(sc.issues, Issue{Code: "unreadable", Level: LevelWarning, File: path.Join(v.Rel, rel)})
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if rel != "." && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		file := path.Join(v.Rel, rel)
		isMD := strings.EqualFold(path.Ext(rel), ".md")
		if !d.Type().IsRegular() {
			sc.issues = append(sc.issues, Issue{Code: "not_regular", Level: LevelWarning, File: file})
			if isMD {
				sc.unread = append(sc.unread, placeholder(file, rel))
			}
			return nil
		}
		if !isMD {
			sc.attachments = append(sc.attachments, rel)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			sc.issues = append(sc.issues, Issue{Code: "unreadable", Level: LevelWarning, File: file})
			return nil
		}
		sc.read(v, p, file, rel, info)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(sc.notes, func(i, j int) bool { return sc.notes[i].Path < sc.notes[j].Path })
	sort.Slice(sc.unread, func(i, j int) bool { return sc.unread[i].Path < sc.unread[j].Path })
	sort.Strings(sc.attachments)
	return sc, nil
}

func (sc *scan) read(v *Vault, abs, file, rel string, info fs.FileInfo) {
	b, err := readNote(abs, info, v.Cfg.MaxNoteBytes)
	switch {
	case errors.Is(err, errTooLarge):
		sc.issues = append(sc.issues, Issue{Code: "note_too_large", Level: LevelError, File: file})
		sc.unread = append(sc.unread, placeholder(file, rel))
	case errors.Is(err, errNotRegular) || isLoop(err):
		sc.issues = append(sc.issues, Issue{Code: "not_regular", Level: LevelWarning, File: file})
		sc.unread = append(sc.unread, placeholder(file, rel))
	case err != nil:
		sc.issues = append(sc.issues, Issue{Code: "unreadable", Level: LevelWarning, File: file})
		sc.unread = append(sc.unread, placeholder(file, rel))
	default:
		sc.notes = append(sc.notes, parseNote(file, notePath(rel), string(b), v.Cfg.Types))
	}
}

func notePath(rel string) string { return rel[:len(rel)-len(path.Ext(rel))] }

// placeholder is a note that exists but was not parsed: links to it
// resolve, and its fragments are not checked.
func placeholder(file, rel string) *Note {
	p := notePath(rel)
	return &Note{Path: p, File: file, Name: p[strings.LastIndex(p, "/")+1:], unread: true}
}
