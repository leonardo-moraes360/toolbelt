package egm

import (
	"context"
	"fmt"
	"strings"
)

const exampleMarker = "<!-- tb:egm-example -->"

type refCandidate struct {
	rel  string
	line int
}

// refCandidates returns the R7 (b) spans with a "/" that survive steps 1–5 of
// resolveSource. Those that leave the root or cross a symlink are dropped
// without saying whether they exist (R13).
func refCandidates(ctx context.Context, d *Doc, set *Settings, idx indexView, exempt map[string]bool, res *CheckResult) ([]refCandidate, error) {
	var out []refCandidate
	seen := map[string]bool{}
	for i, l := range d.Lines {
		n := i + 1
		for _, s := range SpanCandidates(l) {
			if !strings.Contains(s, "/") {
				continue
			}
			rel, why := cleanRel("", s)
			if why != "" || seen[rel] {
				continue
			}
			if reason := refExemption(l, rel, set, exempt); reason != "" {
				if reason != "internal" {
					res.Exemptions = append(res.Exemptions, Exemption{Path: rel, Line: n, Reason: reason})
				}
				continue
			}
			link, err := idx.symlinked(ctx, rel)
			if err != nil {
				return nil, err
			}
			if link {
				continue
			}
			seen[rel] = true
			out = append(out, refCandidate{rel: rel, line: n})
		}
	}
	return out, nil
}

func refExemption(line, rel string, set *Settings, exempt map[string]bool) string {
	switch {
	case strings.HasPrefix(rel, ".toolbelt/") || rel == ".toolbelt" || strings.HasPrefix(rel, ".git/") || rel == ".git":
		return "internal"
	case strings.Contains(line, exampleMarker):
		return "example_marker"
	case exempt[strings.TrimSuffix(rel, "/")]:
		return "plan_create"
	case MatchAny(set.RefsIgnore, rel):
		return "refs_ignore"
	}
	return ""
}

func checkRefsExist(ctx context.Context, d *Doc, docRel string, set *Settings, idx indexView, exempt map[string]bool, res *CheckResult) error {
	cands, err := refCandidates(ctx, d, set, idx, exempt, res)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(cands))
	for _, c := range cands {
		paths = append(paths, strings.TrimSuffix(c.rel, "/"))
	}
	found, err := idx.exists(ctx, paths)
	if err != nil {
		return err
	}
	for _, c := range cands {
		p := strings.TrimSuffix(c.rel, "/")
		kind := found[p]
		res.inputs = append(res.inputs, p+"\t"+kindOrMissing(kind))
		if kind != "" {
			continue
		}
		res.Findings = append(res.Findings, Finding{
			Reviewer: checkRefs, Severity: SevMajor,
			Scope:    Scope{Heading: d.HeadingAt(c.line), Lines: []int{c.line, c.line}},
			Message:  fmt.Sprintf("%#q is not a versioned file or directory", c.rel),
			Evidence: []Evidence{{Path: docRel, Line: c.line}},
			Key:      "refs:" + p,
		})
	}
	return nil
}

// RunCheckers runs the deterministic checkers and the injection scan over the
// snapshot (R13, R17a). They run in both engines.
func RunCheckers(ctx context.Context, d *Doc, docRel, docType string, set *Settings, idx indexView, sources []Source) (*CheckResult, error) {
	res := &CheckResult{}
	res.Findings = append(res.Findings, checkRequiredSections(d, docType, docRel)...)
	exempt := createExempt(ParsePlan(d))
	if err := checkRefsExist(ctx, d, docRel, set, idx, exempt, res); err != nil {
		return nil, err
	}
	if d.HasDetailed() {
		if err := checkPlanFiles(ctx, d, docRel, idx, res); err != nil {
			return nil, err
		}
	}
	res.Findings = append(res.Findings, ScanInjection(d, docRel)...)
	for _, s := range sources {
		if f := ScanInjectionSource(s.Path, s.Text); f != nil {
			res.Findings = append(res.Findings, *f)
		}
	}
	res.Findings = dedupeByKey(res.Findings)
	return res, nil
}

// dedupeByKey keeps the first finding per (reviewer, key): checker lineages
// are matched by key, so a repeated key (the same path twice in the plan)
// must not open a new lineage every round.
func dedupeByKey(fs []Finding) []Finding {
	seen := map[string]bool{}
	out := fs[:0]
	for i := range fs {
		k := fs[i].Reviewer + "\x00" + fs[i].Key
		if fs[i].Key != "" && seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, fs[i])
	}
	return out
}
