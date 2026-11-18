package registry

import (
	"context"
	"errors"
	"fmt"

	"github.com/leonardo-moraes360/toolbelt/internal/merge"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

func init() {
	Register(&Tool{
		Name:        "tb_merge",
		Description: "Merges done tickets into a target branch (parity with tb merge). Dry-run by default; only updates refs with apply:true",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"ids":             {Type: "array", Items: &Property{Type: "string"}, Description: "Ticket IDs (TKT-001)"},
				"strategy":        {Type: "string", Description: "fail (default)|last_wins", Default: "fail"},
				"into":            {Type: "string", Description: "Target branch (default: current branch)"},
				"force":           {Type: "boolean", Description: "Re-merges tickets that were already merged", Default: false},
				"allow_sensitive": {Type: "boolean", Description: "Allows applying tickets that change hooks/.toolbelt/workflows/.gitmodules/gitlinks/.gitattributes", Default: false},
				"apply":           {Type: "boolean", Description: "Actually applies the merge (without this it is a dry-run)", Default: false},
				"cwd":             {Type: "string", Description: "Repository directory"},
			},
			Required: []string{"ids"},
		},
		Handler: handleMerge,
	})
}

func handleMerge(_ context.Context, args map[string]any) (any, error) {
	cwd, err := resolveToolCWD(args)
	if err != nil {
		return nil, err
	}
	ids := StringSliceArg(args, "ids", nil)
	if len(ids) == 0 {
		return nil, fmt.Errorf("ids is required")
	}
	strategy, err := merge.ParseStrategy(StringArg(args, "strategy"))
	if err != nil {
		return nil, err
	}
	res, err := merge.Run(&merge.Options{
		Repo:           cwd,
		Store:          ticket.Open(ticket.FindRoot(cwd)),
		IDs:            ids,
		Strategy:       strategy,
		DryRun:         !BoolArg(args, "apply", false),
		Into:           StringArg(args, "into"),
		Force:          BoolArg(args, "force", false),
		AllowSensitive: BoolArg(args, "allow_sensitive", false),
	})
	if err != nil {
		if (errors.Is(err, merge.ErrConflict) || errors.Is(err, merge.ErrSensitive)) && res != nil {
			// A conflict is a result, not a protocol failure: return the full
			// report with ok=false (like tb_stack aggregates errors in the payload).
			return mergeFailure{Result: res, OK: false, Error: err.Error()}, nil
		}
		return nil, err
	}
	return res, nil
}

// mergeFailure is tb_merge's payload on conflict (strategy fail).
type mergeFailure struct {
	*merge.Result
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}
