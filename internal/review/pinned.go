package review

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/git"
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
	approvedID, err := gitPatchIDVerbatim(repo, baseSHA, resolved)
	if err != nil {
		return nil, fmt.Errorf("patch-id base..approved: %w", err)
	}
	headID, err := gitPatchIDVerbatim(repo, baseSHA, headSHA)
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
	summary, err := gitRangeDiff(repo, baseSHA, resolved, headSHA)
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
		return "", git.Wrap(err, nil, "patch-id", "--verbatim")
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
