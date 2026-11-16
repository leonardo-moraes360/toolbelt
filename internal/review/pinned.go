package review

import (
	"fmt"
	"os/exec"
	"strings"
)

type PinnedApproval struct {
	Checked          bool   `json:"checked"`
	ApprovedSHA      string `json:"approved_sha,omitempty"`
	PatchID          string `json:"patch_id,omitempty"`
	HeadPatchID      string `json:"head_patch_id,omitempty"`
	Match            *bool  `json:"match,omitempty"`
	RangeDiffSummary string `json:"range_diff,omitempty"`
}

func boolPtr(v bool) *bool { return &v }

func (p *PinnedApproval) Diverged() bool {
	return p != nil && p.Checked && p.Match != nil && !*p.Match
}

func CheckPinnedApproval(repo, base, approved, head string) (*PinnedApproval, error) {
	if approved == "" {
		return &PinnedApproval{Checked: false}, nil
	}
	resolved, err := resolveGitRef(repo, approved)
	if err != nil {
		return nil, fmt.Errorf("approved-sha: %w", err)
	}
	approvedID, err := gitPatchIDVerbatim(repo, base, resolved)
	if err != nil {
		return nil, fmt.Errorf("patch-id base..approved: %w", err)
	}
	headID, err := gitPatchIDVerbatim(repo, base, head)
	if err != nil {
		return nil, fmt.Errorf("patch-id base..head: %w", err)
	}
	match := approvedID == headID
	out := &PinnedApproval{
		Checked:     true,
		ApprovedSHA: resolved,
		PatchID:     approvedID,
		HeadPatchID: headID,
		Match:       boolPtr(match),
	}
	if match {
		return out, nil
	}
	summary, err := gitRangeDiff(repo, base, resolved, head)
	if err != nil {
		return nil, err
	}
	out.RangeDiffSummary = summary
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
