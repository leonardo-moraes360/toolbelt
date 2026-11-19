package egm

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Checker names (R13).
const (
	checkRefs     = "refs-exist"
	checkSections = "required-sections"
	checkPlan     = "plan-files-listed"
)

// isChecker reports whether a reviewer name is a deterministic checker (R13,
// R17a).
func isChecker(name string) bool {
	switch name {
	case checkRefs, checkSections, checkPlan, injectionChecker:
		return true
	}
	return false
}

// Exemption is a refs-exist path skipped by a marker or a plan Create (R13).
type Exemption struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Reason string `json:"reason"` // example_marker | plan_create | refs_ignore
}

// CheckResult is the output of the checkers for one snapshot.
type CheckResult struct {
	Findings   []Finding
	Exemptions []Exemption
	// inputs are "path\tkind" lines (kind: file, dir, missing); their sha256
	// is checks_sha (glossary: content).
	inputs []string
}

func (r *CheckResult) checksSHA() string {
	in := append([]string(nil), r.inputs...)
	sort.Strings(in)
	return sha256Hex([]byte(strings.Join(in, "\n")))
}

// indexView answers existence questions against the index (interactive) or a
// commit tree (gate mode, R7a).
type indexView interface {
	// exists reports "file", "dir" (directory with versioned files) or "".
	exists(ctx context.Context, paths []string) (map[string]string, error)
	// under lists versioned files under dir (at most limit).
	under(ctx context.Context, dir string, limit int) ([]string, error)
	// symlinked reports whether some component of rel is a symlink.
	symlinked(ctx context.Context, rel string) (bool, error)
}

type worktreeIndex struct{ repo *Repo }

func (w worktreeIndex) symlinked(_ context.Context, rel string) (bool, error) {
	_, why, _ := lstatChain(w.repo.Root, rel)
	return why == "symlink", nil
}

func (w worktreeIndex) exists(ctx context.Context, paths []string) (map[string]string, error) {
	out := map[string]string{}
	if len(paths) == 0 {
		return out, nil
	}
	entries, err := w.repo.LsFilesStage(ctx, paths...)
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		dir := strings.TrimSuffix(p, "/") + "/"
		for _, e := range entries {
			if e.Path == p {
				out[p] = "file"
				break
			}
			if strings.HasPrefix(e.Path, dir) {
				out[p] = "dir"
			}
		}
	}
	return out, nil
}

func (w worktreeIndex) under(ctx context.Context, dir string, limit int) ([]string, error) {
	entries, err := w.repo.LsFilesStage(ctx, dir)
	if err != nil {
		return nil, err
	}
	if len(entries) > limit {
		return nil, errorf(ExitError, "too_many_entries", "more than %d versioned files under %s", limit, dir)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Path)
	}
	return out, nil
}

// requiredHeadings per structural standard (R13); either language passes.
var requiredHeadings = map[string][][2]string{
	"feature-spec": {
		{"## Overview", "## Visão geral"}, {"## Problem", "## Problema"}, {"## Goals", "## Objetivos"},
		{"## Non-Goals", "## Não-objetivos"}, {"## Technical Plan", "## Plano técnico"},
	},
	"adr": {
		{"## Status", "## Status"}, {"## Context", "## Contexto"}, {"## Decision", "## Decisão"},
		{"## Consequences", "## Consequências"}, {"## Alternatives Considered", "## Alternativas Consideradas"},
	},
}

func structuralStandard(docType string) string {
	if docType == "adr" {
		return "adr"
	}
	return "feature-spec"
}

func firstHeading(d *Doc) string {
	if hs := d.Headings([2]int{1, len(d.Lines)}); len(hs) > 0 {
		return hs[0]
	}
	return ""
}

func checkRequiredSections(d *Doc, docType, docRel string) []Finding {
	var out []Finding
	title := firstHeading(d)
	line := max(1, d.HeadingLine(title))
	for _, pair := range requiredHeadings[structuralStandard(docType)] {
		if d.HasHeading(pair[0]) || d.HasHeading(pair[1]) {
			continue
		}
		want := "`" + pair[0] + "`"
		if pair[1] != pair[0] {
			want += " (or `" + pair[1] + "`)"
		}
		out = append(out, Finding{
			Reviewer: checkSections, Severity: SevMajor,
			Scope:    Scope{Heading: title, Lines: []int{line, line}},
			Message:  fmt.Sprintf("missing required section %s of the %s standard", want, structuralStandard(docType)),
			Evidence: []Evidence{{Path: docRel, Line: line}},
			Key:      "sections:" + pair[0],
		})
	}
	return out
}
