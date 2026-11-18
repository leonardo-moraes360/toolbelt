package review

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/git"
)

type PinnedApproval struct {
	Checked          bool   `json:"checked"`
	ApprovedSHA      string `json:"approved_sha,omitempty"`
	ApprovedBase     string `json:"approved_base,omitempty"`
	HeadBase         string `json:"head_base,omitempty"`
	PatchID          string `json:"patch_id,omitempty"`
	HeadPatchID      string `json:"head_patch_id,omitempty"`
	Match            *bool  `json:"match,omitempty"`
	RangeDiffSummary string `json:"range_diff,omitempty"`
}

func boolPtr(v bool) *bool { return &v }

func (p *PinnedApproval) Diverged() bool {
	return p != nil && p.Checked && p.Match != nil && !*p.Match
}

var rangeDiffStatusRe = regexp.MustCompile(`(?m)^\s*(?:\d+|-):\s+\S+\s+([=!<>])\s+(?:\d+|-):\s+\S+`)

func CheckPinnedApproval(repo, base, approvedBase, approved, head string) (*PinnedApproval, error) {
	if approved == "" {
		return &PinnedApproval{Checked: false}, nil
	}
	resolved, err := resolveGitRef(repo, approved)
	if err != nil {
		return nil, fmt.Errorf("approved-sha: %w", err)
	}
	headResolved, err := resolveGitRef(repo, head)
	if err != nil {
		return nil, fmt.Errorf("head: %w", err)
	}
	baseResolved, err := resolveGitRef(repo, base)
	if err != nil {
		return nil, fmt.Errorf("base: %w", err)
	}

	approvedSideBase := strings.TrimSpace(approvedBase)
	if approvedSideBase == "" {
		approvedSideBase, err = gitMergeBase(repo, resolved, baseResolved)
		if err != nil {
			return nil, fmt.Errorf("merge-base approved base: %w", err)
		}
	} else {
		approvedSideBase, err = resolveGitRef(repo, approvedSideBase)
		if err != nil {
			return nil, fmt.Errorf("approved-base: %w", err)
		}
	}
	headSideBase, err := gitMergeBase(repo, headResolved, baseResolved)
	if err != nil {
		return nil, fmt.Errorf("merge-base head base: %w", err)
	}

	approvedID, err := gitPatchIDVerbatim(repo, approvedSideBase, resolved)
	if err != nil {
		return nil, fmt.Errorf("patch-id approved_base..approved: %w", err)
	}
	headID, err := gitPatchIDVerbatim(repo, headSideBase, headResolved)
	if err != nil {
		return nil, fmt.Errorf("patch-id head_base..head: %w", err)
	}
	match := approvedID == headID
	out := &PinnedApproval{
		Checked:      true,
		ApprovedSHA:  resolved,
		ApprovedBase: approvedSideBase,
		HeadBase:     headSideBase,
		PatchID:      approvedID,
		HeadPatchID:  headID,
		Match:        boolPtr(match),
	}
	if match {
		return out, nil
	}
	summary, err := gitRangeDiff(repo, approvedSideBase, resolved, headSideBase, headResolved)
	if err != nil {
		return nil, err
	}
	out.RangeDiffSummary = summary
	if rangeDiffAllEqual(summary) {
		out.Match = boolPtr(true)
		out.RangeDiffSummary = ""
	}
	return out, nil
}

func gitPatchIDVerbatim(repo, base, head string) (string, error) {
	diff, err := gitDiff(repo, base, head)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("git", "patch-id", "--verbatim")
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader(diff)
	out, err := cmd.Output()
	if err != nil {
		return "", git.Wrap(err, nil, "patch-id", "--verbatim")
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

func gitRangeDiff(repo, leftBase, leftHead, rightBase, rightHead string) (string, error) {
	left := leftBase + ".." + leftHead
	right := rightBase + ".." + rightHead
	if leftBase == leftHead {
		return "", fmt.Errorf("approved-sha matches the base (%s); base..approved range is empty — no commits for range-diff; the head patch diverges from the approved SHA", shortRef(leftBase))
	}
	out, err := git.CombinedOutput(repo, "range-diff", left, right)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func rangeDiffAllEqual(summary string) bool {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return true
	}
	matches := rangeDiffStatusRe.FindAllStringSubmatch(summary, -1)
	if len(matches) == 0 {
		return false
	}
	for _, m := range matches {
		if m[1] != "=" {
			return false
		}
	}
	return true
}

func shortRef(ref string) string {
	if len(ref) > 12 {
		return ref[:12]
	}
	return ref
}
