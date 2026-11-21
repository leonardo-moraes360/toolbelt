package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Error struct {
	Args        []string
	Stderr      string
	ExitCode    int
	MissingRefs []string
	Err         error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	var b strings.Builder
	op := "git"
	if len(e.Args) > 0 {
		op = "git " + strings.Join(e.Args, " ")
	}
	if e.Err != nil {
		fmt.Fprintf(&b, "%s: %v", op, e.Err)
	} else {
		fmt.Fprintf(&b, "%s failed", op)
	}
	stderr := sanitizeStderr(e.Stderr)
	if stderr != "" {
		fmt.Fprintf(&b, "\nstderr: %s", stderr)
	}
	for _, ref := range e.MissingRefs {
		ref = strings.TrimSuffix(ref, "^{commit}")
		fmt.Fprintf(&b, "\nref missing: %s", ref)
		fmt.Fprintf(&b, "\nhint: %s", missingRefHint(ref))
	}
	return b.String()
}

// missingRefHint avoids suggesting `git fetch origin HEAD~1`: a relative ref
// (ancestor/parent syntax) is missing because history is short — a single-commit
// repository or a shallow clone — not because a branch was never fetched.
func missingRefHint(ref string) string {
	if strings.ContainsAny(ref, "~^") {
		return "history too short for " + ref + " (single-commit repo or shallow clone): pass --base <sha> or run git fetch --deepen=1"
	}
	return "git fetch origin " + ref
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func Wrap(err error, stderr []byte, args ...string) error {
	if err == nil {
		return nil
	}
	ge := &Error{
		Args:   append([]string(nil), args...),
		Stderr: strings.TrimSpace(string(stderr)),
		Err:    err,
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		ge.ExitCode = exitErr.ExitCode()
		if ge.Stderr == "" {
			ge.Stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
	}
	ge.MissingRefs = detectMissingRefs(ge.Stderr, args)
	if isRangeDiffUsage(ge.Stderr, args) {
		ge.Stderr = rangeDiffClearMessage(args)
		ge.MissingRefs = nil
	}
	return ge
}

func Output(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...) //nolint:gosec // operator-supplied args
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, Wrap(err, stderr.Bytes(), args...)
	}
	return out, nil
}

func CombinedOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...) //nolint:gosec // operator-supplied args
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, Wrap(err, out, args...)
	}
	return out, nil
}

func detectMissingRefs(stderr string, args []string) []string {
	if !looksLikeMissingRef(stderr) {
		return nil
	}
	candidates := filterRefCandidates(refsFromArgs(args))
	if len(candidates) == 0 {
		return nil
	}
	lower := strings.ToLower(stderr)
	single := strings.Contains(lower, "needed a single revision") || len(candidates) == 1
	var missing []string
	for _, ref := range candidates {
		if single || strings.Contains(stderr, ref) {
			missing = append(missing, ref)
		}
	}
	if len(missing) == 0 {
		return candidates
	}
	return missing
}

func looksLikeMissingRef(stderr string) bool {
	if stderr == "" {
		return false
	}
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "unknown revision") ||
		strings.Contains(lower, "bad revision") ||
		strings.Contains(lower, "needed a single revision") ||
		strings.Contains(lower, "ambiguous argument") ||
		strings.Contains(lower, "invalid object name")
}

func filterRefCandidates(refs []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, ref := range refs {
		if ref == "" || ref == "HEAD" || strings.HasPrefix(ref, "--") {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
	}
	return out
}

func refsFromArgs(args []string) []string {
	var refs []string
	skippedSubcmd := false
	for _, a := range args {
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		if !skippedSubcmd {
			skippedSubcmd = true
			continue
		}
		if strings.Contains(a, "..") {
			parts := strings.Split(a, "..")
			for _, p := range parts {
				p = strings.TrimPrefix(p, ".")
				if p != "" {
					refs = append(refs, p)
				}
			}
			continue
		}
		refs = append(refs, a)
	}
	return refs
}

func isRangeDiffUsage(stderr string, args []string) bool {
	if len(args) == 0 || args[0] != "range-diff" {
		return false
	}
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "usage: git range-diff") ||
		strings.Contains(lower, "need two commit ranges")
}

func rangeDiffClearMessage(args []string) string {
	left, right := "", ""
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if left == "" {
			left = a
			continue
		}
		if right == "" {
			right = a
			break
		}
	}
	msg := "fatal: need two commit ranges — invalid range for range-diff"
	if left != "" || right != "" {
		msg = fmt.Sprintf("%s (%s vs %s)", msg, left, right)
	}
	if emptyRange(left) {
		msg += "; base..approved looks empty (approved-sha matches the base?)"
	}
	return msg
}

func emptyRange(rangeSpec string) bool {
	parts := strings.Split(rangeSpec, "..")
	if len(parts) != 2 {
		return false
	}
	return parts[0] != "" && parts[0] == parts[1]
}

func sanitizeStderr(stderr string) string {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return ""
	}
	if !strings.Contains(strings.ToLower(stderr), "usage: git ") {
		return collapseWhitespace(stderr)
	}
	lines := strings.Split(stderr, "\n")
	var keep []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		lower := strings.ToLower(trim)
		if strings.HasPrefix(lower, "usage:") || strings.HasPrefix(lower, "or:") {
			break
		}
		if trim == "" && len(keep) > 0 {
			break
		}
		keep = append(keep, trim)
	}
	return collapseWhitespace(strings.Join(keep, "\n"))
}

func collapseWhitespace(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}
