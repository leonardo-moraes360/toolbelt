package registry

import (
	"context"
	"fmt"
	"os"

	"github.com/leonardo-moraes360/toolbelt/internal/mem"
	"github.com/leonardo-moraes360/toolbelt/internal/secplay"
)

func init() {
	registerSecplayTools()
}

func registerSecplayTools() {
	for _, color := range []secplay.Color{secplay.Red, secplay.Blue} {
		c := color
		Register(&Tool{
			Name:        fmt.Sprintf("tb_%s_search", c),
			Description: fmt.Sprintf("Searches %s AppSec playbooks, compact (parity with tb %s search)", c, c),
			InputSchema: &InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {Type: "string", Description: "Query"},
					"area":  {Type: "string", Description: "Filter by area (web, auth, ...)"},
					"limit": {Type: "integer", Description: "Maximum results", Default: 20},
				},
				Required: []string{"query"},
			},
			Handler: secSearchHandler(c),
		})
		Register(&Tool{
			Name:        fmt.Sprintf("tb_%s_load", c),
			Description: fmt.Sprintf("Loads a %s playbook body under a token budget (parity with tb %s load)", c, c),
			InputSchema: &InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name":    {Type: "string", Description: "Playbook name"},
					"section": {Type: "string", Description: "Return only this section (heading or slug)"},
					"budget":  {Type: "integer", Description: "Token budget", Default: secplay.DefaultLoadBudget},
					"full":    {Type: "boolean", Description: "Return the whole body, ignoring the budget", Default: false},
				},
				Required: []string{"name"},
			},
			Handler: secLoadHandler(c),
		})
	}

	Register(&Tool{
		Name:        "tb_sec_pair",
		Description: "Resolves the red/blue mirror playbook (parity with tb red/blue pair)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"name": {Type: "string", Description: "Playbook name (red or blue)"},
			},
			Required: []string{"name"},
		},
		Handler: handleSecPair,
	})

	Register(&Tool{
		Name:        "tb_blue_apply",
		Description: "Registers a blue playbook's checks into .toolbelt/secchecks.yaml (parity with tb blue apply)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"class":   {Type: "string", Description: "Vulnerability class (e.g. sql-injection)"},
				"dry_run": {Type: "boolean", Description: "Compute the diff without writing", Default: false},
			},
			Required: []string{"class"},
		},
		Handler: handleBlueApply,
	})
}

func secToolbeltDir() string {
	cwd, _ := os.Getwd()
	return mem.LocateRepoToolbeltDir(cwd)
}

func secSearchHandler(c secplay.Color) ToolHandler {
	return func(_ context.Context, args map[string]any) (any, error) {
		corpus, err := secplay.Load()
		if err != nil {
			return nil, err
		}
		hits := corpus.Search(c, StringArg(args, "query"), secplay.SearchOptions{
			Area:  StringArg(args, "area"),
			Limit: IntArg(args, "limit", 20),
		})
		return map[string]any{"hits": hits}, nil
	}
}

func secLoadHandler(c secplay.Color) ToolHandler {
	return func(_ context.Context, args map[string]any) (any, error) {
		corpus, err := secplay.Load()
		if err != nil {
			return nil, err
		}
		name := StringArg(args, "name")
		pb, ok := corpus.Get(c, name)
		if !ok {
			return nil, fmt.Errorf("%s playbook %q not found", c, name)
		}
		return pb.Render(secplay.RenderOptions{
			Section: StringArg(args, "section"),
			Budget:  IntArg(args, "budget", secplay.DefaultLoadBudget),
			Full:    BoolArg(args, "full", false),
		})
	}
}

func handleSecPair(_ context.Context, args map[string]any) (any, error) {
	corpus, err := secplay.Load()
	if err != nil {
		return nil, err
	}
	name := StringArg(args, "name")
	m, ok := corpus.Pair(name)
	if !ok {
		return nil, fmt.Errorf("no mirror found for %q", name)
	}
	return map[string]any{"mirror": m.Name, "color": m.Color, "summary": m.Summary}, nil
}

func handleBlueApply(_ context.Context, args map[string]any) (any, error) {
	corpus, err := secplay.Load()
	if err != nil {
		return nil, err
	}
	return corpus.Apply(secToolbeltDir(), StringArg(args, "class"), BoolArg(args, "dry_run", false))
}
