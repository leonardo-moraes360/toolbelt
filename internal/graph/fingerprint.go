package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func SourceFingerprint(root string, opts *BuildOptions) (string, error) {
	if opts == nil {
		def := DefaultBuildOptions()
		opts = &def
	}
	normalizeBuildOptions(opts)
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	abs, err = resolveIndexRoot(abs)
	if err != nil {
		return "", err
	}
	skip := map[string]bool{}
	for _, d := range opts.SkipDirs {
		skip[d] = true
	}
	var lines []string
	err = filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != abs && skip[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Size() > opts.MaxFileBytes {
			return nil
		}
		if len(indexersFor(path, opts)) == 0 {
			return nil
		}
		rel, _ := filepath.Rel(abs, path)
		lines = append(lines, fmt.Sprintf("%s|%d|%d",
			filepath.ToSlash(rel), info.ModTime().UnixNano(), info.Size()))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

func (s *Store) CheckStale(root string, opts *BuildOptions) (bool, error) {
	s.mu.RLock()
	fp := ""
	if s.index != nil {
		fp = s.index.Fingerprint
	}
	s.mu.RUnlock()
	if fp == "" {
		return true, nil
	}
	cur, err := SourceFingerprint(root, opts)
	if err != nil {
		return false, err
	}
	return cur != fp, nil
}

func (s *Store) Stale() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stale
}

func (s *Store) SetStale(v bool) {
	s.mu.Lock()
	s.stale = v
	s.mu.Unlock()
}

func (s *Store) EnsureFresh(root string, opts *BuildOptions, indexOnChange bool) (refreshed bool, err error) {
	stale, err := s.CheckStale(root, opts)
	if err != nil {
		return false, err
	}
	if !stale {
		s.SetStale(false)
		return false, nil
	}
	if indexOnChange {
		if _, err := s.RebuildOpts(root, opts); err != nil {
			return false, err
		}
		s.SetStale(false)
		return true, nil
	}
	s.SetStale(true)
	return false, nil
}

func stampIndexFingerprint(idx *Index, root string, opts *BuildOptions) {
	fp, err := SourceFingerprint(root, opts)
	if err != nil {
		return
	}
	idx.Fingerprint = fp
	idx.BuiltAt = time.Now().UTC().Format(time.RFC3339)
}
