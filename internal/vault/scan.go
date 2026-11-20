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
	attachments []string // relative to the vault, "/"
	issues      []Issue  // not_regular, note_too_large
}

// scan walks the vault without following symlinks. Directories starting
// with "." are skipped; regular ".md" files are notes, other regular files
// attachments, anything else not_regular.
func (v *Vault) scan() (*scan, error) {
	sc := &scan{}
	err := filepath.WalkDir(v.Dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(v.Dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		file := path.Join(v.Rel, rel)
		if !d.Type().IsRegular() {
			sc.issues = append(sc.issues, Issue{Code: "not_regular", Level: LevelWarning, File: file})
			return nil
		}
		if !strings.EqualFold(path.Ext(rel), ".md") {
			sc.attachments = append(sc.attachments, rel)
			return nil
		}
		return sc.read(v, p, file, rel)
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(sc.notes, func(i, j int) bool { return sc.notes[i].Path < sc.notes[j].Path })
	sort.Strings(sc.attachments)
	return sc, nil
}

func (sc *scan) read(v *Vault, abs, file, rel string) error {
	b, err := readNote(abs, v.Cfg.MaxNoteBytes)
	switch {
	case errors.Is(err, errTooLarge):
		sc.issues = append(sc.issues, Issue{Code: "note_too_large", Level: LevelError, File: file})
		return nil
	case errors.Is(err, errNotRegular) || isLoop(err):
		sc.issues = append(sc.issues, Issue{Code: "not_regular", Level: LevelWarning, File: file})
		return nil
	case err != nil:
		return err
	}
	sc.notes = append(sc.notes, parseNote(file, rel[:len(rel)-len(path.Ext(rel))], string(b), v.Cfg.Types))
	return nil
}
