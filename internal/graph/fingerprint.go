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
	fw := &fpWalker{root: abs, opts: opts, skip: map[string]bool{}}
	for _, d := range opts.SkipDirs {
		fw.skip[d] = true
	}
	fw.vault = opts.vault
	if fw.vault == nil {
		fw.vault = resolveVault(abs, opts)
	}
	if err := filepath.Walk(abs, fw.visit); err != nil {
		return "", err
	}
	if vs := fw.vault; vs != nil && vs.index {
		if err := fw.addVault(vs); err != nil {
			return "", err
		}
	}
	lines := fw.lines
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

// fpWalker collects one fingerprint line per indexed file.
type fpWalker struct {
	root  string
	vault *vaultState
	opts  *BuildOptions
	skip  map[string]bool
	lines []string
}

func (fw *fpWalker) visit(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.IsDir() {
		if path != fw.root && (fw.skip[info.Name()] || fw.vault.skipDir(fw.root, path, info)) {
			return filepath.SkipDir
		}
		return nil
	}
	if info.Size() > fw.opts.MaxFileBytes || len(indexersFor(path, fw.opts)) == 0 {
		return nil
	}
	rel, _ := filepath.Rel(fw.root, path)
	fw.lines = append(fw.lines, fmt.Sprintf("%s|%d|%d",
		filepath.ToSlash(rel), info.ModTime().UnixNano(), info.Size()))
	return nil
}

// addVault fingerprints every vault entry the scan sees (a note, an
// attachment or a symlink changes link states, whatever its size), skipping
// dot entries like .obsidian/, plus the vault config, which changes what is
// parsed.
func (fw *fpWalker) addVault(vs *vaultState) error {
	fw.lines = append(fw.lines, "vault-config|"+vs.cfg)
	return filepath.Walk(vs.v.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path != vs.v.Dir && strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(vs.v.Dir, path)
		fw.lines = append(fw.lines, fmt.Sprintf("vault:%s|%d|%d|%s",
			filepath.ToSlash(rel), info.ModTime().UnixNano(), info.Size(), info.Mode().Type()))
		return nil
	})
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
