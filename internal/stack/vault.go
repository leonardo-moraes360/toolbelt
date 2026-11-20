package stack

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/leonardo-moraes360/toolbelt/internal/vault"
)

// runVault is the vault op (docs/specs/vault.md, R19): tb vault check over
// the repository's working tree (not --head: the vault is read from disk).
// It only reads, so it runs in parallel like guard; it ignores --strict,
// which is an egm flag.
func runVault(ctx context.Context, shared *Shared) (StepResult, error) {
	if err := ctx.Err(); err != nil {
		return StepResult{}, err
	}
	repo, err := filepath.Abs(shared.RepoDir)
	if err != nil {
		return StepResult{}, err
	}
	v, err := vault.Open(repo)
	if err != nil {
		code := vault.ExitError
		var e *vault.Error
		if errors.As(err, &e) {
			code = e.Code
		}
		return StepResult{Op: string(OpVault), OK: false, ExitCode: code, Reason: "vault: " + err.Error()}, nil
	}
	res, err := v.Check()
	if err != nil {
		return StepResult{}, err
	}
	step := StepResult{Op: string(OpVault), OK: res.OK, ExitCode: res.ExitCode, Result: res}
	if !res.OK {
		step.Reason = fmt.Sprintf("vault: %d errors, %d warnings (working tree)", res.Errors, res.Warnings)
	}
	return step, nil
}
