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
	Short: "Alerts and control of router workers (reads .toolbelt/router)",
}

var watchdogStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Worker state and alert counts",
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
			fmt.Println("No router has run in this repository (use `tb router run`).")
			return nil
		}
		if err != nil {
			return err
		}
		if worker != "" {
			found := false
			ws := rep.Workers[:0:0]
			for i := range rep.Workers {
				if rep.Workers[i].ID == worker {
					found = true
					ws = append(ws, rep.Workers[i])
				}
			}
			if !found {
				return fmt.Errorf("worker %q does not exist in the current state", worker)
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
			fmt.Println("  alerts: none")
			return nil
		}
		parts := make([]string, 0, len(counts))
		for _, k := range []string{router.AlertWorkerSlow, router.AlertWorkerStuck, router.AlertWorkerCrash, router.AlertWorkerKilled, router.AlertQueueGrowing, router.AlertGlobalTimeout,
			router.AlertBusUnclaimed, router.AlertBusClaimRejected, router.AlertBusLeaseLost, router.AlertBusVerdictFail, router.AlertBusChannelExpired, router.AlertBusWriteFailed} {
			if n := counts[k]; n > 0 {
				parts = append(parts, fmt.Sprintf("%s=%d", k, n))
			}
		}
		fmt.Printf("  alerts: %s\n", strings.Join(parts, " "))
		return nil
	},
}

var watchdogAlertsCmd = &cobra.Command{
	Use:   "alerts",
	Short: "Lists alerts (alerts.jsonl)",
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
			fmt.Println("No alerts.")
			return nil
		}
		for _, a := range alerts {
			who := a.Worker
			if a.Ticket != "" {
				who += " " + a.Ticket
			}
			fmt.Printf("%s  %-8s %-14s %-14s %s [action: %s]\n", a.Time.Local().Format("15:04:05"), a.Severity, a.Kind, strings.TrimSpace(who), a.Message, a.Action)
		}
		return nil
	},
}

var watchdogKillCmd = &cobra.Command{
	Use:   "kill <worker>",
	Short: "Asks the router to cancel a worker's running ticket (counts as an attempt)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reason, _ := cmd.Flags().GetString("reason")
		worker := args[0]
		if !router.ValidWorkerID(worker) {
			return fmt.Errorf("invalid worker %q (expected w1, w2, ...)", worker)
		}
		root, err := routerRoot()
		if err != nil {
			return err
		}
		rep, err := router.LoadStatus(root)
		if errors.Is(err, router.ErrNoState) || (err == nil && !rep.Alive) {
			return errors.New("no router running; nothing to stop")
		}
		if err != nil {
			return err
		}
		var ticketID string
		known := false
		for i := range rep.Workers {
			if w := &rep.Workers[i]; w.ID == worker {
				known = true
				if w.Status != "working" {
					return fmt.Errorf("worker %s is idle", worker)
				}
				ticketID = w.Ticket
			}
		}
		if !known {
			return fmt.Errorf("worker %q does not exist (workers: %d)", worker, len(rep.Workers))
		}
		path, err := router.RequestKill(root, worker, reason, time.Now())
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"worker": worker, "ticket": ticketID, "reason": reason, "request": path})
		}
		fmt.Printf("Kill request sent: %s (%s). The router applies it on the next tick.\n", worker, ticketID)
		if strings.HasPrefix(worker, "b") {
			fmt.Println("Bus slot: the router revokes the thread; the remote agent's own harness stops its child, the router cannot.")
		}
		return nil
	},
}

func init() {
	watchdogStatusCmd.Flags().String("worker", "", "filters by worker (w1, w2, ... or bus slots b1, b2, ...)")
	watchdogAlertsCmd.Flags().String("since", "", "only recent alerts: duration (10m, 2h) or RFC3339")
	watchdogAlertsCmd.Flags().String("kind", "", "worker_slow|worker_stuck|worker_crash|worker_killed|queue_growing|global_timeout|ticket_canceled|multi_dep_base|agent_escaped|orphan_killed|board_unreadable|bus_unclaimed|bus_claim_rejected|bus_lease_lost|bus_verdict_fail|bus_channel_expired|bus_write_failed")
	watchdogAlertsCmd.Flags().Int("limit", 0, "only the N most recent (0 = all)")
	watchdogKillCmd.Flags().String("reason", "", "reason (recorded in the alert and the ticket)")
	_ = watchdogKillCmd.MarkFlagRequired("reason")
	watchdogCmd.AddCommand(watchdogStatusCmd, watchdogAlertsCmd, watchdogKillCmd)
}
