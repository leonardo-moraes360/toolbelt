package egm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (s *snapshot) readRound(n int, rel string) (string, error) {
	p := filepath.Join(s.stateDir, fmt.Sprintf("round-%d", n), "target.md")
	if rel != s.rel {
		p = filepath.Join(s.stateDir, fmt.Sprintf("round-%d", n), "sources", filepath.FromSlash(rel))
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

// touchedSince is the change test of R28 (--fix) and R20 (still_holds:
// false): since round ref, a non-whitespace change touched the mapped scope
// or the evidence ±5 lines of a cited source.
func (s *snapshot) touchedSince(ctx context.Context, l *Lineage, refRound int) (bool, error) {
	ref, err := s.readRound(refRound, s.rel)
	if err != nil {
		return false, err
	}
	lm, err := DiffSnapshots(ctx, s.stateDir, ref, s.text)
	if err != nil {
		return false, err
	}
	changed := lm.Changed()
	// Scope and evidence lines are numbered in ScopeRound's snapshot.
	sm := lm
	if l.ScopeRound != refRound {
		old, err := s.readRound(l.ScopeRound, s.rel)
		if err != nil {
			return false, err
		}
		if sm, err = DiffSnapshots(ctx, s.stateDir, old, s.text); err != nil {
			return false, err
		}
	}
	if Touches(sm.MapScope(l.Scope).Lines, changed, 0) {
		return true, nil
	}
	return s.evidenceTouched(ctx, l, refRound, sm, changed)
}

func (s *snapshot) evidenceTouched(ctx context.Context, l *Lineage, refRound int, docMap *LineMap, changed map[int]bool) (bool, error) {
	for _, e := range l.Evidence {
		if !containsStr(l.Cited, e.Path) {
			continue
		}
		if e.Path == s.rel {
			if Touches([]int{docMap.Map(e.Line), docMap.Map(e.Line)}, changed, 5) {
				return true, nil
			}
			continue
		}
		ok, err := s.sourceTouched(ctx, refRound, e)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

func (s *snapshot) sourceTouched(ctx context.Context, ref int, e Evidence) (bool, error) {
	old, err := s.readRound(ref, e.Path)
	if err != nil {
		return false, err
	}
	cur := ""
	for _, src := range s.sources.Included {
		if src.Path == e.Path {
			cur = src.Text
		}
	}
	sm, err := DiffSnapshots(ctx, s.stateDir, old, cur)
	if err != nil {
		return false, err
	}
	return Touches([]int{sm.Map(e.Line), sm.Map(e.Line)}, sm.Changed(), 5), nil
}
