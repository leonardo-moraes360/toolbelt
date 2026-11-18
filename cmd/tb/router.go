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
	Short: "Scheduler de tickets em primeiro plano (workers, entrega direta, watchdog)",
	Long: `Router em primeiro plano: distribui tickets do board entre workers, cada um
numa worktree própria (branch tb-workspace/<ticket>), executando o agente
configurado dentro do sandbox (fail-closed; sem Landlock exige
--allow-unsandboxed). O watchdog roda dentro do loop e grava
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
	if err := cfg.Router.Validate(); err != nil {
		return cfg.Router, cfg, err
	}
	return cfg.Router, cfg, nil
}

var routerRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Executa o router até esvaziar a fila (Ctrl-C devolve running para pending)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		workers, _ := f.GetInt("workers")
		strategy, _ := f.GetString("strategy")
		agentCmd, _ := f.GetString("agent-cmd")
		once, _ := f.GetBool("once")
		allow, _ := f.GetBool("allow-unsandboxed")
		dry, _ := f.GetBool("dry-run")

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
				return errors.New("--agent-cmd vazio")
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
			return fmt.Errorf("%d ticket(s) falharam: %s", len(sum.Failed), strings.Join(sum.Failed, ", "))
		}
		if sum.Reason == router.ReasonStalled {
			return fmt.Errorf("router parou sem progresso; tickets pending que nunca rodarão (dependências failed/canceled): %s", strings.Join(sum.Unschedulable, ", "))
		}
		return nil
	},
}

func renderPlan(p *router.Plan) {
	dd := "desligada"
	if p.DirectDelivery {
		dd = "ligada"
	}
	fmt.Printf("Plano (estratégia %s, %d workers, entrega direta %s)\n", p.Strategy, p.Workers, dd)
	if p.Paused {
		fmt.Println("  board pausado: nada seria atribuído até `tb board resume`")
	}
	if p.Draining {
		fmt.Println("  board em drain: nada seria atribuído até `tb board resume`")
	}
	if len(p.Rounds) == 0 {
		fmt.Println("  nenhum ticket pronto")
	}
	for _, rd := range p.Rounds {
		fmt.Printf("Rodada %d\n", rd.Round)
		for _, a := range rd.Assignments {
			fmt.Printf("  %-3s %-8s %s  [%s, base %s]\n", a.Worker, a.Ticket, a.Title, a.Via, a.Base)
		}
	}
	if len(p.Unschedulable) > 0 {
		fmt.Println("Não agendáveis (dependências nunca concluem):")
		for _, u := range p.Unschedulable {
			fmt.Printf("  %-8s %s\n", u.Ticket, strings.Join(u.Reasons, "; "))
		}
	}
	fmt.Printf("(%s)\n", p.Note)
}

func renderSummary(s *router.Summary) {
	fmt.Printf("Router encerrado: %s\n", s.Reason)
	fmt.Printf("  concluídos: %s\n", listOrDash(s.Done))
	fmt.Printf("  falhos:     %s\n", listOrDash(s.Failed))
	if len(s.Retried) > 0 {
		fmt.Printf("  re-tentados: %s\n", strings.Join(s.Retried, ", "))
	}
	if len(s.Unschedulable) > 0 {
		fmt.Printf("  bloqueados (nunca rodarão): %s\n", strings.Join(s.Unschedulable, ", "))
	}
	if len(s.Requeued) > 0 {
		fmt.Printf("  devolvidos a pending: %s\n", strings.Join(s.Requeued, ", "))
	}
	fmt.Printf("  alertas: %d  duração: %s\n", s.Alerts, (time.Duration(s.DurationMS) * time.Millisecond).Round(time.Millisecond))
}

func listOrDash(l []string) string {
	if len(l) == 0 {
		return "-"
	}
	return strings.Join(l, ", ")
}

var routerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Mostra o estado do router (state.json; detecta estado obsoleto)",
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
			fmt.Println("Nenhum router rodou neste repositório (use `tb router run`).")
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
		state = fmt.Sprintf("OBSOLETO (pid %d não existe mais; o router morreu sem encerrar)", rep.PID)
	case rep.Alive:
		state = fmt.Sprintf("running (pid %d)", rep.PID)
	}
	fmt.Printf("Router: %s\n", state)
	fmt.Printf("  estratégia: %s  iniciado: %s\n", rep.Strategy, rep.StartedAt.Local().Format(time.RFC3339))
	fmt.Printf("  pausado: %s  drain: %s\n", yesNo(rep.Paused), yesNo(rep.Draining))
	fmt.Printf("  fila: ready %d, blocked %d, running %d\n", rep.Queue.Ready, rep.Queue.Blocked, rep.Queue.Running)
	for _, w := range rep.Workers {
		if onlyWorker != "" && w.ID != onlyWorker {
			continue
		}
		line := fmt.Sprintf("  %-3s %-7s", w.ID, w.Status)
		if w.Ticket != "" {
			line += " " + w.Ticket
			if w.StartedAt != nil {
				line += " desde " + w.StartedAt.Local().Format("15:04:05")
			}
		}
		if len(w.Subscriptions) > 0 {
			line += "  reservas: " + strings.Join(w.Subscriptions, ", ")
		}
		fmt.Println(line)
	}
}

func yesNo(b bool) string {
	if b {
		return "sim"
	}
	return "não"
}

var routerConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Mostra a configuração efetiva do router (seção router:)",
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
	routerRunCmd.Flags().Int("workers", 0, "número de workers (padrão: router.max_workers)")
	routerRunCmd.Flags().String("strategy", "", "fifo|priority|dependency (padrão: router.strategy)")
	routerRunCmd.Flags().String("agent-cmd", "", "comando do agente (padrão: router.agent_cmd); roda na worktree do ticket")
	routerRunCmd.Flags().Bool("once", false, "uma rodada de agendamento e sai ao terminar os tickets iniciados")
	routerRunCmd.Flags().Bool("allow-unsandboxed", false, "opt-in explícito para executar sem isolamento Landlock")
	routerRunCmd.Flags().Bool("dry-run", false, "mostra o plano de agendamento sem executar")
	routerCmd.AddCommand(routerRunCmd, routerStatusCmd, routerConfigCmd)
}
