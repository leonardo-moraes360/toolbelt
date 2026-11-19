package egm

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
)

const arbiterTermGap = 80

func parseEvidence(s string) (Evidence, bool) {
	i := strings.LastIndex(s, ":")
	if i <= 0 {
		return Evidence{}, false
	}
	n, err := strconv.Atoi(s[i+1:])
	if err != nil || n < 1 {
		return Evidence{}, false
	}
	return Evidence{Path: s[:i], Line: n}, true
}

func refuseContest(format string, a ...any) error {
	return errorf(ExitError, "contest_refused", format, a...)
}

// validateContest is R28's --contest: open/reopened, an agent configured,
// no previous contest, evidence in cited that passes resolveSource again, a
// persona other than comprehension, and the R17a checks on the excerpt.
func (rv *resolver) validateContest(ctx context.Context, l *Lineage, raw string) (func(), error) {
	switch {
	case l.Status != StOpen && l.Status != StReopened:
		return nil, invalid(l, "--contest", "only open or reopened lineages can be contested")
	case l.Contests != 0:
		return nil, refuseContest("%s was already contested once", l.ID)
	case l.Persona == "comprehension":
		return nil, refuseContest("%s is a comprehension finding: not contestable", l.ID)
	}
	if _, err := ResolveProfile(&rv.s.set); err != nil {
		return nil, refuseContest("--contest needs the llm engine with an agent (egm.agent_cmd): %v", err)
	}
	e, ok := parseEvidence(raw)
	if !ok {
		return nil, errorf(ExitError, "usage", "--evidence must be path:line")
	}
	text, err := rv.evidenceText(ctx, l, e)
	if err != nil {
		return nil, err
	}
	if err := rv.injectionGuard(text, e); err != nil {
		return nil, err
	}
	return func() {
		l.Contests = 1
		l.ContestEvidence = &e
		rv.move(l, "contest", StContestPending, "")
	}, nil
}

// evidenceText resolves the evidence file again and returns its redacted
// text: the document from the current snapshot, or a versioned source whose
// worktree, HEAD and opened_round bytes are the same.
func (rv *resolver) evidenceText(ctx context.Context, l *Lineage, e Evidence) (string, error) {
	if !containsStr(l.Cited, e.Path) {
		return "", refuseContest("%s is not among the sources %s cites", e.Path, l.ID)
	}
	if e.Path == rv.s.rel {
		return rv.s.text, nil
	}
	rel, why := cleanRel("", e.Path)
	if why != "" || denied(rel, rv.s.rel, rv.s.set.DocGlobs) {
		return "", refuseContest("evidence %s is not an allowed source", e.Path)
	}
	fi, why, missing := lstatChain(rv.s.repo.Root, rel)
	if missing || why != "" || !fi.Mode().IsRegular() {
		return "", refuseContest("evidence %s is not a regular file inside the repository", e.Path)
	}
	b, err := readRegular(filepath.Join(rv.s.repo.Root, filepath.FromSlash(rel)), rv.s.set.MaxSourceBytes)
	if err != nil {
		return "", refuseContest("evidence %s: %v", e.Path, err)
	}
	blob, err := rv.s.repo.CatFileBlob(ctx, "HEAD", rel, rv.s.set.MaxSourceBytes)
	if err != nil {
		return "", refuseContest("evidence %s is not versioned at HEAD", e.Path)
	}
	opened := rv.st.roundN(l.OpenedRound)
	if opened == nil || sha256Hex(b) != sha256Hex(blob) || opened.Sources[rel] != sha256Hex(b) {
		return "", refuseContest("evidence %s changed since round %d or differs from HEAD", e.Path, l.OpenedRound)
	}
	red, secret := Redact(string(b))
	if secret {
		return "", refuseContest("evidence %s looks like a secret", e.Path)
	}
	return red, nil
}

// injectionGuard is R17a for a contest: the evidence excerpt (±20 lines) may
// contain only the already-flagged matches of injection-scan lineages with
// the same match_sha, and no term aimed at the arbiter near any match.
func (rv *resolver) injectionGuard(text string, e Evidence) error {
	d := ParseDoc(text, nil)
	lo, hi := max(1, e.Line-arbiterWindow), min(len(d.Lines), e.Line+arbiterWindow)
	exempt := rv.exemptLines(e.Path)
	for _, m := range scanLines(d) {
		if m.line < lo || m.line > hi {
			continue
		}
		if nearArbiterTerm(d.Lines[m.line-1], m.text) {
			return refuseContest("the evidence has a term aimed at the arbiter near an injection pattern (line %d)", m.line)
		}
		if !exempt[m.line] {
			return refuseContest("the evidence has an injection pattern at line %d", m.line)
		}
	}
	return nil
}

func nearArbiterTerm(line, match string) bool {
	i := strings.Index(strings.ToLower(line), strings.ToLower(match))
	if i < 0 {
		return reArbiterTerm.MatchString(line)
	}
	from, to := max(0, i-arbiterTermGap), min(len(line), i+len(match)+arbiterTermGap)
	return reArbiterTerm.MatchString(line[from:to])
}

// exemptLines are the scope lines of injection-scan lineages in open,
// reopened, contest_pending or settled whose match_sha still equals the
// current scan's.
func (rv *resolver) exemptLines(path string) map[int]bool {
	out := map[int]bool{}
	if path != rv.s.rel {
		return out
	}
	current := map[string]string{}
	for i := range rv.s.checks.Findings {
		if f := &rv.s.checks.Findings[i]; f.Reviewer == injectionChecker {
			current[f.Key] = f.MatchSHA
		}
	}
	for _, l := range rv.st.Lineages {
		if l.Owner != injectionChecker || len(l.Scope.Lines) != 2 || current[l.Key] != l.MatchSHA {
			continue
		}
		switch l.Status {
		case StOpen, StReopened, StContestPending, StSettled:
			for n := l.Scope.Lines[0]; n <= l.Scope.Lines[1]; n++ {
				out[n] = true
			}
		}
	}
	return out
}
