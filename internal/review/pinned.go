package review

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/git"
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
	baseSHA, err := resolveGitRef(repo, base)
	if err != nil {
		return nil, fmt.Errorf("base: %w", err)
	}
	resolved, err := resolveGitRef(repo, approved)
	if err != nil {
		return nil, fmt.Errorf("approved-sha: %w", err)
	}
	headSHA, err := resolveGitRef(repo, head)
	if err != nil {
		return nil, fmt.Errorf("head: %w", err)
	}
	approvedID, err := gitPatchIDStable(repo, baseSHA, resolved)
	if err != nil {
		return nil, fmt.Errorf("patch-id base..approved: %w", err)
	}
	headID, err := gitPatchIDStable(repo, baseSHA, headSHA)
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
	summary, err := gitRangeDiff(repo, baseSHA, resolved, headSHA)
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
		return "", git.Wrap(err, nil, "patch-id", "--stable")
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
	if base == approved {
		return "", fmt.Errorf("approved-sha coincide com a base (%s); intervalo base..approved vazio — não há commits para range-diff; o patch do head diverge do SHA aprovado", shortRef(base))
	}
	out, err := git.CombinedOutput(repo, "range-diff", left, right)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func shortRef(ref string) string {
	if len(ref) > 12 {
		return ref[:12]
	}
	return ref
}
