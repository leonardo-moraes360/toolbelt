package review

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type PinnedApproval struct {
	ApprovedSHA      string `json:"approved_sha"`
	ApprovedBase     string `json:"approved_base"`
	HeadBase         string `json:"head_base"`
	PatchID          string `json:"patch_id"`
	HeadPatchID      string `json:"head_patch_id"`
	Match            bool   `json:"match"`
	RangeDiffSummary string `json:"range_diff,omitempty"`
}

var rangeDiffStatusRe = regexp.MustCompile(`(?m)^\s*(?:\d+|-):\s+\S+\s+([=!<>])\s+(?:\d+|-):\s+\S+`)

func CheckPinnedApproval(repo, base, approvedBase, approved, head string) (*PinnedApproval, error) {
	if approved == "" {
		return nil, nil
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

	approvedID, err := gitPatchIDStable(repo, approvedSideBase, resolved)
	if err != nil {
		return nil, fmt.Errorf("patch-id approved_base..approved: %w", err)
	}
	headID, err := gitPatchIDStable(repo, headSideBase, headResolved)
	if err != nil {
		return nil, fmt.Errorf("patch-id head_base..head: %w", err)
	}
	out := &PinnedApproval{
		ApprovedSHA:  resolved,
		ApprovedBase: approvedSideBase,
		HeadBase:     headSideBase,
		PatchID:      approvedID,
		HeadPatchID:  headID,
		Match:        approvedID == headID,
	}
	if out.Match {
		return out, nil
	}
	summary, err := gitRangeDiff(repo, approvedSideBase, resolved, headSideBase, headResolved)
	if err != nil {
		return nil, err
	}
	out.RangeDiffSummary = summary
	if rangeDiffAllEqual(summary) {
		out.Match = true
		out.RangeDiffSummary = ""
	}
	return out, nil
}

func gitMergeBase(repo, a, b string) (string, error) {
	cmd := exec.Command("git", "merge-base", a, b) //nolint:gosec // refs do operador
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git merge-base %s %s: %w", a, b, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func gitPatchIDStable(repo, base, head string) (string, error) {
	diff, err := gitDiff(repo, base, head)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("git", "patch-id", "--stable")
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader(diff)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git patch-id: %w", err)
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
	cmd := exec.Command("git", "-c", "core.pager=", "range-diff", "--no-patch", left, right) //nolint:gosec // refs do operador
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "PAGER=cat")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git range-diff %s %s: %w (%s)", left, right, err, strings.TrimSpace(string(out)))
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
