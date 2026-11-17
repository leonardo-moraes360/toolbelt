package registry

import (
	"context"
	"errors"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/router"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

func init() {
	registerRouterTools()
}

func registerRouterTools() {
	cwdProp := Property{Type: "string", Description: "Diretório do repositório"}
	Register(&Tool{
		Name:        "tb_router_status",
		Description: "Estado do router (state.json: workers, fila, pausa; detecta estado obsoleto). Paridade com tb router status",
		InputSchema: &InputSchema{
			Type:       "object",
			Properties: map[string]Property{"cwd": cwdProp},
		},
		Handler: handleRouterStatus,
	})
	Register(&Tool{
		Name:        "tb_watchdog_alerts",
		Description: "Alertas do watchdog (alerts.jsonl). Paridade com tb watchdog alerts",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"since": {Type: "string", Description: "Só alertas recentes: duração (10m, 2h) ou RFC3339"},
				"kind":  {Type: "string", Description: "worker_slow|worker_stuck|worker_crash|worker_killed|queue_growing|global_timeout"},
				"limit": {Type: "integer", Description: "Só os N mais recentes (0 = todos)", Default: 0},
				"cwd":   cwdProp,
			},
		},
		Handler: handleWatchdogAlerts,
	})
}

func routerRootFromArgs(args map[string]any) (string, error) {
	cwd, err := resolveToolCWD(args)
	if err != nil {
		return "", err
	}
	return ticket.FindRoot(cwd), nil
}

func handleRouterStatus(_ context.Context, args map[string]any) (any, error) {
	root, err := routerRootFromArgs(args)
	if err != nil {
		return nil, err
	}
	rep, err := router.LoadStatus(root)
	if errors.Is(err, router.ErrNoState) {
		return map[string]any{"status": "none"}, nil
	}
	if err != nil {
		return nil, err
	}
	return rep, nil
}

func handleWatchdogAlerts(_ context.Context, args map[string]any) (any, error) {
	root, err := routerRootFromArgs(args)
	if err != nil {
		return nil, err
	}
	since, err := router.ParseSince(StringArg(args, "since"), time.Now())
	if err != nil {
		return nil, err
	}
	return router.ReadAlerts(root, router.AlertFilter{Since: since, Kind: StringArg(args, "kind"), Limit: IntArg(args, "limit", 0)})
}
