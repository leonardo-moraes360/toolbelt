package registry

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/leonardo-moraes360/toolbelt/internal/vault"
)

// vault MCP tools (docs/specs/vault.md, R18): note text comes back inside
// "data" objects, redacted; tb_vault_new always writes source: inferred.
func registerVaultTools() {
	cwd := Property{Type: "string", Description: "Absolute directory inside the repository; the vault is found from it"}
	note := Property{Type: "string", Description: "Note path (with or without .md) or a link like a, [[a]], a#heading"}
	Register(&Tool{
		Name:        "tb_vault_list",
		Description: "Lists vault notes with path, type, source and data{title, tags} (parity with tb vault list)",
		InputSchema: &InputSchema{Type: "object", Properties: map[string]Property{
			"cwd": cwd, "type": {Type: "string", Description: "Only notes of this type"},
			"tag":    {Type: "string", Description: "Only notes with this tag"},
			"source": {Type: "string", Description: "Only notes with this source (manual, extracted, inferred)"},
		}, Required: []string{"cwd"}},
		Handler: func(_ context.Context, args map[string]any) (any, error) {
			return withVault(args, func(v *vault.Vault) (any, error) {
				return v.List(vault.Filter{Type: StringArg(args, "type"), Tag: StringArg(args, "tag"), Source: StringArg(args, "source")})
			})
		},
	})
	Register(&Tool{
		Name:        "tb_vault_show",
		Description: "Shows a vault note: metadata, data{title, tags, aliases, body}, links and backlinks (parity with tb vault show)",
		InputSchema: &InputSchema{Type: "object", Properties: map[string]Property{"cwd": cwd, "note": note}, Required: []string{"cwd", "note"}},
		Handler: func(_ context.Context, args map[string]any) (any, error) {
			return withVault(args, func(v *vault.Vault) (any, error) { return v.Show(StringArg(args, "note")) })
		},
	})
	Register(&Tool{
		Name:        "tb_vault_search",
		Description: "Case- and accent-insensitive search over redacted note text; one hit per note with data{title, snippet} (parity with tb vault search)",
		InputSchema: &InputSchema{Type: "object", Properties: map[string]Property{
			"cwd": cwd, "query": {Type: "string", Description: "Text to find"},
			"limit": {Type: "integer", Description: "Maximum hits", Default: 20},
		}, Required: []string{"cwd", "query"}},
		Handler: func(_ context.Context, args map[string]any) (any, error) {
			return withVault(args, func(v *vault.Vault) (any, error) {
				return v.Search(StringArg(args, "query"), IntArg(args, "limit", 20))
			})
		},
	})
	Register(&Tool{
		Name:        "tb_vault_links",
		Description: "Lists a note's outgoing wikilinks with state ok, broken, ambiguous, broken_fragment, attachment, missing_attachment (parity with tb vault links)",
		InputSchema: &InputSchema{Type: "object", Properties: map[string]Property{"cwd": cwd, "note": note}, Required: []string{"cwd", "note"}},
		Handler: func(_ context.Context, args map[string]any) (any, error) {
			return withVault(args, func(v *vault.Vault) (any, error) { return v.Links(StringArg(args, "note")) })
		},
	})
	Register(&Tool{
		Name:        "tb_vault_backlinks",
		Description: "Lists the notes that link to a note (parity with tb vault backlinks)",
		InputSchema: &InputSchema{Type: "object", Properties: map[string]Property{"cwd": cwd, "note": note}, Required: []string{"cwd", "note"}},
		Handler: func(_ context.Context, args map[string]any) (any, error) {
			return withVault(args, func(v *vault.Vault) (any, error) { return v.Backlinks(StringArg(args, "note")) })
		},
	})
	Register(&Tool{
		Name:        "tb_vault_check",
		Description: "Validates the vault; exit_code 1 on broken or ambiguous links, bad frontmatter or oversized notes (parity with tb vault check)",
		InputSchema: &InputSchema{Type: "object", Properties: map[string]Property{"cwd": cwd}, Required: []string{"cwd"}},
		Handler: func(_ context.Context, args map[string]any) (any, error) {
			return withVault(args, func(v *vault.Vault) (any, error) { return v.Check() })
		},
	})
	Register(&Tool{
		Name: "tb_vault_new",
		Description: "Creates a vault note written by an agent: always source inferred with the given confidence; never overwrites " +
			"(parity with tb vault new, which a human uses for source manual)",
		InputSchema: &InputSchema{Type: "object", Properties: map[string]Property{
			"cwd": cwd, "title": {Type: "string", Description: "Note title (no newlines); the file name is its slug"},
			"confidence": {Type: "number", Description: "Confidence in [0, 1] (required: agent notes are source inferred)"},
			"type":       {Type: "string", Description: "One of vault.types (default fact)"},
			"tags":       {Type: "array", Description: "Tags matching ^[a-z0-9][a-z0-9/_-]*$", Items: &Property{Type: "string"}},
			"dir":        {Type: "string", Description: "Subdirectory of the vault"},
			"body":       {Type: "string", Description: "Markdown body after the title heading"},
		}, Required: []string{"cwd", "title", "confidence"}},
		Handler: handleVaultNew,
	})
}

func withVault(args map[string]any, fn func(*vault.Vault) (any, error)) (any, error) {
	cwd := StringArg(args, "cwd")
	if !filepath.IsAbs(cwd) {
		return nil, fmt.Errorf("cwd must be an absolute path")
	}
	v, err := vault.Open(cwd)
	if err != nil {
		return nil, err
	}
	return fn(v)
}

func handleVaultNew(_ context.Context, args map[string]any) (any, error) {
	c, ok := args["confidence"].(float64) // a JSON number only: no "NaN" strings
	if !ok {
		return nil, fmt.Errorf("confidence is required as a number in [0, 1]: agent notes are source inferred")
	}
	return withVault(args, func(v *vault.Vault) (any, error) {
		return v.New(&vault.NewOptions{
			Title: StringArg(args, "title"), Type: StringArg(args, "type"), Tags: StringSliceArg(args, "tags", nil),
			Dir: StringArg(args, "dir"), Body: StringArg(args, "body"),
			Source: vault.SourceInferred, Confidence: &c, // never source manual over MCP (R18)
		})
	})
}
