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
		Description: "Integra tickets done num branch alvo (paridade com tb merge). Dry-run por padrão; só altera refs com apply:true",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"ids":             {Type: "array", Items: &Property{Type: "string"}, Description: "IDs dos tickets (TKT-001)"},
				"strategy":        {Type: "string", Description: "fail (padrão)|last_wins", Default: "fail"},
				"into":            {Type: "string", Description: "Branch alvo (padrão: branch atual)"},
				"force":           {Type: "boolean", Description: "Refaz merge de tickets já mergeados", Default: false},
				"allow_sensitive": {Type: "boolean", Description: "Permite aplicar tickets que alteram hooks/.toolbelt/workflows/.gitmodules/gitlinks/.gitattributes", Default: false},
				"apply":           {Type: "boolean", Description: "Aplica de verdade (sem isso é dry-run)", Default: false},
				"cwd":             {Type: "string", Description: "Diretório do repositório"},
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
		return nil, fmt.Errorf("ids é obrigatório")
	}
	strategy, err := merge.ParseStrategy(StringArg(args, "strategy"))
	if err != nil {
		return nil, err
	}
	res, err := merge.Run(merge.Options{
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
			// Conflito é resultado, não falha de protocolo: devolve o relatório
			// completo com ok=false (como tb_stack agrega erros no payload).
			return mergeFailure{Result: res, OK: false, Error: err.Error()}, nil
		}
		return nil, err
	}
	return res, nil
}

// mergeFailure é o payload de tb_merge quando há conflito (strategy fail).
type mergeFailure struct {
	*merge.Result
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}
