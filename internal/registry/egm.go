package registry

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/leonardo-moraes360/toolbelt/internal/egm"
)

// egm MCP tools (R31): status, resolve and receipt; review and loop next take
// minutes and stay CLI-only. Each call reloads the user config.
func registerEgmTools() {
	Register(&Tool{
		Name:        "tb_egm_status",
		Description: "EGM verdict of a spec/ADR for the worktree, or a commit with at (parity with tb egm status); exit_code 0 PASS, 1 FAIL, 3 not passing",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cwd":    {Type: "string", Description: "Absolute directory; doc resolves against it"},
				"doc":    {Type: "string", Description: "Document path"},
				"at":     {Type: "string", Description: "Gate mode: evaluates the document, sources and config at this commit"},
				"strict": {Type: "boolean", Description: "PASS_PARTIAL gives exit_code 1", Default: false},
			},
			Required: []string{"cwd", "doc"},
		},
		Handler: handleEgmStatus,
	})
	Register(&Tool{
		Name:        "tb_egm_receipt",
		Description: "Writes .toolbelt/egm/receipts/<slug>.json, the committed receipt of the author's claims checked by the CI mode (parity with tb egm receipt); refused unless the verdict is PASS or PASS_PARTIAL",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cwd": {Type: "string", Description: "Absolute directory; doc resolves against it"},
				"doc": {Type: "string", Description: "Document path"},
			},
			Required: []string{"cwd", "doc"},
		},
		Handler: handleEgmReceipt,
	})
	Register(&Tool{
		Name:        "tb_egm_resolve",
		Description: "Marks EGM lineages as fixed, declined, force-settled or contested (parity with tb egm resolve)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"cwd":          {Type: "string", Description: "Absolute directory; doc resolves against it"},
				"doc":          {Type: "string", Description: "Document path"},
				"fix":          {Type: "array", Description: "Lineages fixed by an edit (F-1, F-2)", Items: &Property{Type: "string"}},
				"decline":      {Type: "array", Description: "nit/major lineages declined (a major is a downgrade)", Items: &Property{Type: "string"}},
				"force_settle": {Type: "string", Description: "Lineage settled without a fix (downgrade; needs reason)"},
				"reason":       {Type: "string", Description: "Reason for force_settle"},
				"contest":      {Type: "string", Description: "Lineage contested at the arbiter (llm engine; needs evidence)"},
				"evidence":     {Type: "string", Description: "path:line of the contest evidence"},
			},
			Required: []string{"cwd", "doc"},
		},
		Handler: handleEgmResolve,
	})
}

func egmOptions(args map[string]any) (egm.Options, error) {
	cwd, doc := StringArg(args, "cwd"), StringArg(args, "doc")
	if !filepath.IsAbs(cwd) {
		return egm.Options{}, fmt.Errorf("cwd must be an absolute path")
	}
	if doc == "" {
		return egm.Options{}, fmt.Errorf("doc is required")
	}
	return egm.Options{Doc: doc, Cwd: cwd}, nil
}

func handleEgmStatus(ctx context.Context, args map[string]any) (any, error) {
	o, err := egmOptions(args)
	if err != nil {
		return nil, err
	}
	o.At, o.Strict = StringArg(args, "at"), BoolArg(args, "strict", false)
	return egm.Status(ctx, &o)
}

func handleEgmResolve(ctx context.Context, args map[string]any) (any, error) {
	o, err := egmOptions(args)
	if err != nil {
		return nil, err
	}
	return egm.Resolve(ctx, &egm.ResolveOptions{
		Options:     o,
		Fix:         StringSliceArg(args, "fix", nil),
		Decline:     StringSliceArg(args, "decline", nil),
		ForceSettle: StringArg(args, "force_settle"),
		Reason:      StringArg(args, "reason"),
		Contest:     StringArg(args, "contest"),
		Evidence:    StringArg(args, "evidence"),
	})
}

func handleEgmReceipt(ctx context.Context, args map[string]any) (any, error) {
	o, err := egmOptions(args)
	if err != nil {
		return nil, err
	}
	return egm.WriteReceipt(ctx, &o)
}
