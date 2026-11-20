package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/execenv"
	"github.com/leonardo-moraes360/toolbelt/internal/router"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

var routerCmd = &cobra.Command{
	Use:   "router",
	Short: "Foreground ticket scheduler (workers, direct delivery, watchdog)",
	Long: `Foreground router: distributes board tickets among workers, each in its
own worktree (branch tb-workspace/<ticket>), running the configured agent
inside the sandbox (fail-closed; without Landlock it requires
--allow-unsandboxed). The watchdog runs inside the loop and writes
.toolbelt/router/{state.json,alerts.jsonl}.`,
}

func routerRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return ticket.FindRoot(cwd), nil
}

// Reads the router: section from global + local config, or from -c.
func loadRouterConfig(root string) (config.RouterConfig, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.RouterConfig{}, nil, err
	}
	if cwd, _ := os.Getwd(); config.File() == "" && cwd != root {
		if fc, err := config.LoadFile(filepath.Join(root, config.DefaultConfigDir, config.DefaultConfigFile)); err == nil {
			cfg = fc
		}
	}
	// router.bus only from trusted config: an agent that can write the
	// repository must not repoint the router (docs/bus-router.md, B4b).
	cfg.Router.Bus = config.RouterBus{}
	if trusted, err := config.LoadTrusted(); err == nil {
		cfg.Router.Bus = trusted.Router.Bus
	}
	if err := cfg.Router.Validate(); err != nil {
		return cfg.Router, cfg, err
	}
	return cfg.Router, cfg, nil
}

var routerRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs the router until the queue is empty (Ctrl-C returns running to pending)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		workers, _ := f.GetInt("workers")
		strategy, _ := f.GetString("strategy")
		agentCmd, _ := f.GetString("agent-cmd")
		once, _ := f.GetBool("once")
		allow, _ := f.GetBool("allow-unsandboxed")
		dry, _ := f.GetBool("dry-run")
		check, _ := f.GetBool("check")

		root, err := routerRoot()
		if err != nil {
			return err
		}
		rc, cfg, err := loadRouterConfig(root)
		if err != nil {
			return err
		}
		var argv []string
		if agentCmd != "" {
			if argv, err = router.ParseCommand(agentCmd); err != nil {
				return err
			}
			if len(argv) == 0 {
				return errors.New("--agent-cmd is empty")
			}
		}
		var out io.Writer = os.Stderr
		if jsonOutput {
			out = io.Discard
		}
		r, err := router.New(&router.Options{
			Root: root, Config: rc, Workers: workers, Strategy: strategy, AgentCmd: argv,
			Once: once, AllowUnsandboxed: allow,
			EnvOptions: execenv.Options{
				Allow: append(append([]string{}, cfg.Env.Allow...), envAllow...),
				Pass:  append(append([]string{}, cfg.Env.Pass...), envPass...),
			},
			Out: out,
		})
		if err != nil {
			return err
		}
		if check {
			if err := r.CheckBus(); err != nil {
				return err
			}
			fmt.Printf("ok: router bus %s/%s ready\n", rc.Bus.Channel, rc.Bus.As)
			return nil
		}
		if dry {
			plan, err := r.Plan()
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(plan)
			}
			renderPlan(&plan)
			return nil
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		sum, err := r.Run(ctx)
		if err != nil {
			return err
		}
		if jsonOutput {
			if err := printJSON(sum); err != nil {
				return err
			}
		} else {
			renderSummary(sum)
		}
		if len(sum.Failed) > 0 {
			return fmt.Errorf("%d ticket(s) failed: %s", len(sum.Failed), strings.Join(sum.Failed, ", "))
		}
		if sum.Reason == router.ReasonStalled {
			return fmt.Errorf("router stopped without progress; pending tickets that will never run (failed/canceled dependencies): %s", strings.Join(sum.Unschedulable, ", "))
		}
		return nil
	},
}

func renderPlan(p *router.Plan) {
	dd := "off"
	if p.DirectDelivery {
		dd = "on"
	}
	fmt.Printf("Plan (strategy %s, %d workers, direct delivery %s)\n", p.Strategy, p.Workers, dd)
	if p.Paused {
		fmt.Println("  board paused: nothing would be assigned until `tb board resume`")
	}
	if p.Draining {
		fmt.Println("  board draining: nothing would be assigned until `tb board resume`")
	}
	if len(p.Rounds) == 0 {
		fmt.Println("  no ticket ready")
	}
	for _, rd := range p.Rounds {
		fmt.Printf("Round %d\n", rd.Round)
		for _, a := range rd.Assignments {
			fmt.Printf("  %-3s %-8s %s  [%s, base %s]\n", a.Worker, a.Ticket, a.Title, a.Via, a.Base)
		}
	}
	if len(p.Unschedulable) > 0 {
		fmt.Println("Unschedulable (dependencies never complete):")
		for _, u := range p.Unschedulable {
			fmt.Printf("  %-8s %s\n", u.Ticket, strings.Join(u.Reasons, "; "))
		}
	}
	fmt.Printf("(%s)\n", p.Note)
}

func renderSummary(s *router.Summary) {
	fmt.Printf("Router finished: %s\n", s.Reason)
	fmt.Printf("  done:        %s\n", listOrDash(s.Done))
	fmt.Printf("  failed:      %s\n", listOrDash(s.Failed))
	if len(s.Retried) > 0 {
		fmt.Printf("  retried:    %s\n", strings.Join(s.Retried, ", "))
	}
	if len(s.Unschedulable) > 0 {
		fmt.Printf("  blocked (will never run): %s\n", strings.Join(s.Unschedulable, ", "))
	}
	if len(s.Requeued) > 0 {
		fmt.Printf("  returned to pending: %s\n", strings.Join(s.Requeued, ", "))
	}
	fmt.Printf("  alerts: %d  duration: %s\n", s.Alerts, (time.Duration(s.DurationMS) * time.Millisecond).Round(time.Millisecond))
}

func listOrDash(l []string) string {
	if len(l) == 0 {
		return "-"
	}
	return strings.Join(l, ", ")
}

var routerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Shows the router state (state.json; detects stale state)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
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
		if jsonOutput {
			return printJSON(rep)
		}
		renderRouterStatus(rep, "")
		return nil
	},
}

func renderRouterStatus(rep *router.StatusReport, onlyWorker string) {
	state := rep.Status
	switch {
	case rep.Stale:
		state = fmt.Sprintf("STALE (pid %d no longer exists; the router died without shutting down)", rep.PID)
	case rep.Alive:
		state = fmt.Sprintf("running (pid %d)", rep.PID)
	}
	fmt.Printf("Router: %s\n", state)
	fmt.Printf("  strategy: %s  started: %s\n", rep.Strategy, rep.StartedAt.Local().Format(time.RFC3339))
	fmt.Printf("  paused: %s  drain: %s\n", yesNo(rep.Paused), yesNo(rep.Draining))
	fmt.Printf("  queue: ready %d, blocked %d, running %d\n", rep.Queue.Ready, rep.Queue.Blocked, rep.Queue.Running)
	for i := range rep.Workers {
		w := &rep.Workers[i]
		if onlyWorker != "" && w.ID != onlyWorker {
			continue
		}
		line := fmt.Sprintf("  %-3s %-7s", w.ID, w.Status)
		if w.Ticket != "" {
			line += " " + w.Ticket
			if w.StartedAt != nil {
				line += " since " + w.StartedAt.Local().Format("15:04:05")
			}
		}
		if w.Thread != "" {
			line += fmt.Sprintf("  bus %s %s", w.Phase, w.Thread)
			if w.Agent != "" {
				line += " by " + w.Agent
			}
		}
		if len(w.Subscriptions) > 0 {
			line += "  reservations: " + strings.Join(w.Subscriptions, ", ")
		}
		fmt.Println(line)
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

var routerConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Shows the effective router configuration (router: section)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := routerRoot()
		if err != nil {
			return err
		}
		rc, _, err := loadRouterConfig(root)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(rc)
		}
		data, err := yaml.Marshal(map[string]config.RouterConfig{"router": rc})
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	},
}

func init() {
	routerRunCmd.Flags().Int("workers", 0, "number of workers (default: router.max_workers)")
	routerRunCmd.Flags().String("strategy", "", "fifo|priority|dependency (default: router.strategy)")
	routerRunCmd.Flags().String("agent-cmd", "", "agent command (default: router.agent_cmd); runs in the ticket worktree")
	routerRunCmd.Flags().Bool("once", false, "one scheduling round; exits when the started tickets finish")
	routerRunCmd.Flags().Bool("allow-unsandboxed", false, "explicit opt-in to run without Landlock isolation")
	routerRunCmd.Flags().Bool("dry-run", false, "shows the scheduling plan without executing")
	routerRunCmd.Flags().Bool("check", false, "validates the bus session and channel (router.bus) and exits")
	routerCmd.AddCommand(routerRunCmd, routerStatusCmd, routerConfigCmd)
}
