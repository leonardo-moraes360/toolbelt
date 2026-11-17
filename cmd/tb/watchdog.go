package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/router"
)

var watchdogCmd = &cobra.Command{
	Use:   "watchdog",
	Short: "Alertas e controle dos workers do router (lê .toolbelt/router)",
}

var watchdogStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Estado dos workers e contagem de alertas",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		worker, _ := cmd.Flags().GetString("worker")
		root, err := routerRoot()
		if err != nil {
			return err
		}
		rep, err := router.LoadStatus(root)
		if errors.Is(err, router.ErrNoState) {
			if jsonOutput {
				return printJSON(map[string]any{"status": "none"})
			}
			fmt.Println("Nenhum router rodou neste repositório (use `tb router run`).")
			return nil
		}
		if err != nil {
			return err
		}
		if worker != "" {
			found := false
			ws := rep.Workers[:0:0]
			for _, w := range rep.Workers {
				if w.ID == worker {
					found = true
					ws = append(ws, w)
				}
			}
			if !found {
				return fmt.Errorf("worker %q não existe no estado atual", worker)
			}
			rep.Workers = ws
		}
		alerts, err := router.ReadAlerts(root, router.AlertFilter{})
		if err != nil {
			return err
		}
		counts := map[string]int{}
		for _, a := range alerts {
			if worker == "" || a.Worker == worker {
				counts[a.Kind]++
			}
		}
		if jsonOutput {
			return printJSON(map[string]any{"router": rep, "alerts": counts})
		}
		renderRouterStatus(rep, worker)
		if len(counts) == 0 {
			fmt.Println("  alertas: nenhum")
			return nil
		}
		parts := make([]string, 0, len(counts))
		for _, k := range []string{router.AlertWorkerSlow, router.AlertWorkerStuck, router.AlertWorkerCrash, router.AlertWorkerKilled, router.AlertQueueGrowing, router.AlertGlobalTimeout} {
			if n := counts[k]; n > 0 {
				parts = append(parts, fmt.Sprintf("%s=%d", k, n))
			}
		}
		fmt.Printf("  alertas: %s\n", strings.Join(parts, " "))
		return nil
	},
}

var watchdogAlertsCmd = &cobra.Command{
	Use:   "alerts",
	Short: "Lista alertas (alerts.jsonl)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		since, _ := f.GetString("since")
		kind, _ := f.GetString("kind")
		limit, _ := f.GetInt("limit")
		root, err := routerRoot()
		if err != nil {
			return err
		}
		sinceT, err := router.ParseSince(since, time.Now())
		if err != nil {
			return err
		}
		alerts, err := router.ReadAlerts(root, router.AlertFilter{Since: sinceT, Kind: kind, Limit: limit})
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(alerts)
		}
		if len(alerts) == 0 {
			fmt.Println("Nenhum alerta.")
			return nil
		}
		for _, a := range alerts {
			who := a.Worker
			if a.Ticket != "" {
				who += " " + a.Ticket
			}
			fmt.Printf("%s  %-8s %-14s %-14s %s [ação: %s]\n", a.Time.Local().Format("15:04:05"), a.Severity, a.Kind, strings.TrimSpace(who), a.Message, a.Action)
		}
		return nil
	},
}

var watchdogKillCmd = &cobra.Command{
	Use:   "kill <worker>",
	Short: "Pede ao router que cancele o ticket em execução de um worker (conta como tentativa)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reason, _ := cmd.Flags().GetString("reason")
		worker := args[0]
		if !router.ValidWorkerID(worker) {
			return fmt.Errorf("worker inválido %q (esperado w1, w2, ...)", worker)
		}
		root, err := routerRoot()
		if err != nil {
			return err
		}
		rep, err := router.LoadStatus(root)
		if errors.Is(err, router.ErrNoState) || (err == nil && !rep.Alive) {
			return errors.New("nenhum router em execução; nada para encerrar")
		}
		if err != nil {
			return err
		}
		var ticketID string
		known := false
		for _, w := range rep.Workers {
			if w.ID == worker {
				known = true
				if w.Status != "working" {
					return fmt.Errorf("worker %s está ocioso", worker)
				}
				ticketID = w.Ticket
			}
		}
		if !known {
			return fmt.Errorf("worker %q não existe (workers: %d)", worker, len(rep.Workers))
		}
		path, err := router.RequestKill(root, worker, reason, time.Now())
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"worker": worker, "ticket": ticketID, "reason": reason, "request": path})
		}
		fmt.Printf("Pedido de kill enviado: %s (%s). O router aplica no próximo tick.\n", worker, ticketID)
		return nil
	},
}

func init() {
	watchdogStatusCmd.Flags().String("worker", "", "filtra por worker (w1, w2, ...)")
	watchdogAlertsCmd.Flags().String("since", "", "só alertas recentes: duração (10m, 2h) ou RFC3339")
	watchdogAlertsCmd.Flags().String("kind", "", "worker_slow|worker_stuck|worker_crash|worker_killed|queue_growing|global_timeout|ticket_canceled|multi_dep_base|agent_escaped|orphan_killed|board_unreadable")
	watchdogAlertsCmd.Flags().Int("limit", 0, "só os N mais recentes (0 = todos)")
	watchdogKillCmd.Flags().String("reason", "", "motivo (registrado no alerta e no ticket)")
	_ = watchdogKillCmd.MarkFlagRequired("reason")
	watchdogCmd.AddCommand(watchdogStatusCmd, watchdogAlertsCmd, watchdogKillCmd)
}
