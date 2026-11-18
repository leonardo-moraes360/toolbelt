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
	Short: "Ticket board: ready, blocked, running (pause/resume/drain)",
}

func boardView(cmd *cobra.Command) (ticket.View, error) {
	f := cmd.Flags()
	statuses, _ := f.GetString("status")
	worker, _ := f.GetString("worker")
	strategy, _ := f.GetString("strategy")
	st, err := ticket.ParseStrategy(strategy)
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
	state := "active"
	switch {
	case v.Paused && v.Draining:
		state = "paused, draining"
	case v.Paused:
		state = "paused"
	case v.Draining:
		state = "draining"
	}
	fmt.Printf("board: %s (strategy: %s)\n", state, v.Strategy)
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
		fmt.Printf("  %s\t%s\t(waiting: %s)\n", b.Ticket.ID, b.Ticket.Title, strings.Join(b.Reasons, ", "))
	}
	section("DONE", v.Done)
	section("FAILED", v.Failed)
	section("CANCELED", v.Canceled)
	for _, c := range v.Cycles {
		fmt.Printf("\nwarning: dependency cycle: %s\n", strings.Join(c, " -> "))
	}
}

var boardListCmd = &cobra.Command{
	Use:   "list",
	Short: "Shows the board (RUNNING / READY / BLOCKED / DONE / FAILED)",
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
	Short: "Refreshes the board periodically until Ctrl+C",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		if interval <= 0 {
			return fmt.Errorf("--interval must be positive")
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
	boardListCmd.Flags().String("status", "", "filters sections by status (comma-separated list)")
	boardListCmd.Flags().String("worker", "", "filters by worker")
	boardListCmd.Flags().String("strategy", "", "ordering: fifo|priority|dependency (default dependency)")
	boardWatchCmd.Flags().AddFlagSet(boardListCmd.Flags())
	boardWatchCmd.Flags().Duration("interval", 2*time.Second, "refresh interval")

	boardCmd.AddCommand(boardListCmd, boardWatchCmd,
		boardControlCmd("pause", "Pause: no new assignments", "board paused", ticket.Pause),
		boardControlCmd("resume", "Resume (clears pause and drain)", "board resumed", ticket.Resume),
		boardControlCmd("drain", "Finish what is running and assign no new work", "board draining", ticket.Drain),
	)
}
