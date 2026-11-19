package stack

import (
	"context"
	"path/filepath"

	"github.com/leonardo-moraes360/toolbelt/internal/egm"
)

// runEgm is the egm op (R33): status --at head of the gated documents of
// merge-base(base, head)..head, plus --egm-doc. egm reads its own user
// config (R31), never the stack's merged config.
func runEgm(ctx context.Context, opts *Options, shared *Shared) (StepResult, error) {
	repo, err := filepath.Abs(shared.RepoDir)
	if err != nil {
		return StepResult{}, err
	}
	res, err := egm.Gate(ctx, &egm.GateOptions{
		RepoDir: repo, Base: shared.Base, Head: shared.Head, Docs: opts.EgmDocs, Strict: opts.Strict,
	})
	if err != nil {
		return StepResult{}, err
	}
	step := StepResult{Op: string(OpEgm), OK: res.OK, Warn: res.OK && res.Warn, Result: res}
	if !res.OK {
		step.ExitCode = 1
		step.Reason = gateReason(res)
	}
	return step, nil
}

// gateReason names the first failing document, or the stack-level warning
// that failed the step.
func gateReason(res *egm.GateResult) string {
	for _, d := range res.Docs {
		if !d.OK {
			why := d.Verdict
			if d.Reason != "" {
				why += " (" + d.Reason + ")"
			}
			return "egm: " + d.Doc + ": " + why
		}
	}
	for _, w := range res.Warnings {
		if w.Code == "uncommitted_gated_docs" {
			return "egm: uncommitted_gated_docs: " + w.Path
		}
	}
	return "egm: no_gated_docs (--strict)"
}
