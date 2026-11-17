package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Quadro de tickets: pronto, bloqueado, rodando (pause/resume/drain)",
}

func boardView(cmd *cobra.Command) (ticket.View, error) {
	f := cmd.Flags()
	statuses, _ := f.GetString("status")
	worker, _ := f.GetString("worker")
	strategyName, _ := f.GetString("strategy")
	st, err := ticket.ParseStrategy(strategyName)
	if err != nil {
		return ticket.View{}, err
	}
	sts, err := parseStatusList(statuses)
	if err != nil {
		return ticket.View{}, err
	}
	s, err := openTicketStore()
	if err != nil {
		return ticket.View{}, err
	}
	b, err := ticket.LoadBoard(s)
	if err != nil {
		return ticket.View{}, err
	}
	return b.View(st, sts, worker), nil
}

func renderBoard(v *ticket.View) {
	state := "ativo"
	switch {
	case v.Paused && v.Draining:
		state = "pausado, drenando"
	case v.Paused:
		state = "pausado"
	case v.Draining:
		state = "drenando"
	}
	fmt.Printf("board: %s (estratégia: %s)\n", state, v.Strategy)
	section := func(name string, list []ticket.Ticket) {
		fmt.Printf("\n%s (%d)\n", name, len(list))
		for i := range list {
			fmt.Println("  " + ticketLine(&list[i]))
		}
	}
	section("RUNNING", v.Running)
	section("READY", v.Ready)
	fmt.Printf("\nBLOCKED (%d)\n", len(v.Blocked))
	for i := range v.Blocked {
		b := &v.Blocked[i]
		fmt.Printf("  %s\t%s\t(aguardando: %s)\n", b.Ticket.ID, b.Ticket.Title, strings.Join(b.Reasons, ", "))
	}
	section("DONE", v.Done)
	section("FAILED", v.Failed)
	section("CANCEL\u004cED", v.Canceled)
	for _, c := range v.Cycles {
		fmt.Printf("\naviso: ciclo de dependências: %s\n", strings.Join(c, " -> "))
	}
}

var boardListCmd = &cobra.Command{
	Use:   "list",
	Short: "Mostra o board (RUNNING / READY / BLOCKED / DONE / FAILED)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := boardView(cmd)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(v)
		}
		renderBoard(&v)
		return nil
	},
}

var boardWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Atualiza o board periodicamente até Ctrl+C",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		if interval <= 0 {
			return fmt.Errorf("--interval deve ser positivo")
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return watchBoard(ctx, cmd, interval)
	},
}

func watchBoard(ctx context.Context, cmd *cobra.Command, interval time.Duration) error {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		v, err := boardView(cmd)
		if err != nil {
			return err
		}
		if jsonOutput {
			if err := printJSONLine(v); err != nil {
				return err
			}
		} else {
			fmt.Print("\033[H\033[2J")
			renderBoard(&v)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

func boardControlCmd(use, short, done string, fn func(root string) (ticket.Control, error)) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openTicketStore()
			if err != nil {
				return err
			}
			c, err := fn(s.Root())
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(c)
			}
			fmt.Println(done)
			return nil
		},
	}
}

func init() {
	boardListCmd.Flags().String("status", "", "filtra seções por status (lista separada por vírgula)")
	boardListCmd.Flags().String("worker", "", "filtra por worker")
	boardListCmd.Flags().String("strategy", "", "ordenação: fifo|priority|dependency (padrão dependency)")
	boardWatchCmd.Flags().AddFlagSet(boardListCmd.Flags())
	boardWatchCmd.Flags().Duration("interval", 2*time.Second, "intervalo de atualização")

	boardCmd.AddCommand(boardListCmd, boardWatchCmd,
		boardControlCmd("pause", "Pausa: nenhuma nova atribuição", "board pausado", ticket.Pause),
		boardControlCmd("resume", "Retoma (limpa pause e drain)", "board retomado", ticket.Resume),
		boardControlCmd("drain", "Conclui o que roda e não atribui novos", "board em drain", ticket.Drain),
	)
}
