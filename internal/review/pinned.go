package review

import (
	"fmt"
	"os/exec"
	"strings"
)

type PinnedApproval struct {
	ApprovedSHA      string `json:"approved_sha"`
	PatchID          string `json:"patch_id"`
	HeadPatchID      string `json:"head_patch_id"`
	Match            bool   `json:"match"`
	RangeDiffSummary string `json:"range_diff,omitempty"`
}

func CheckPinnedApproval(repo, base, approved, head string) (*PinnedApproval, error) {
	if approved == "" {
		return nil, nil
	}
	resolved, err := resolveGitRef(repo, approved)
	if err != nil {
		return nil, fmt.Errorf("approved-sha: %w", err)
	}
	approvedID, err := gitPatchIDStable(repo, base, resolved)
	if err != nil {
		return nil, fmt.Errorf("patch-id base..approved: %w", err)
	}
	headID, err := gitPatchIDStable(repo, base, head)
	if err != nil {
		return nil, fmt.Errorf("patch-id base..head: %w", err)
	}
	out := &PinnedApproval{
		ApprovedSHA: resolved,
		PatchID:     approvedID,
		HeadPatchID: headID,
		Match:       approvedID == headID,
	}
	if out.Match {
		return out, nil
	}
	summary, err := gitRangeDiff(repo, base, resolved, head)
	if err != nil {
		return nil, err
	}
	out.RangeDiffSummary = summary
	return out, nil
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

func gitRangeDiff(repo, base, approved, head string) (string, error) {
	left := base + ".." + approved
	right := base + ".." + head
	cmd := exec.Command("git", "range-diff", left, right) //nolint:gosec // refs do operador
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git range-diff %s %s: %w (%s)", left, right, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
