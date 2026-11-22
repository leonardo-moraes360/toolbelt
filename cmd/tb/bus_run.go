package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
	"github.com/leonardo-moraes360/toolbelt/internal/busrun"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

var busRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Harness: waits for messages without an LLM and wakes the agent (B3)",
	Long: `Local process, one per agent, that waits for messages in the store without
spending tokens and wakes the agent through the adapter in the wake file
(.toolbelt/bus/wake/<channel>/<name>.yaml, mode 0600): exec (local command with
the wake packet on stdin), harness (claude -p --resume) or notify (desktop or
terminal notification for the human, for GUI-only agents). The wake packet only
carries headers; the agent reads the content with recv. Specs: docs/bus-harness.md, docs/bus-notify-discover.md.`,
	Args: cobra.NoArgs,
	RunE: runBusRun,
}

func runBusRun(cmd *cobra.Command, _ []string) error {
	f := cmd.Flags()
	channel, _ := f.GetString("channel")
	as, _ := f.GetString("as")
	invite, _ := f.GetString("invite")
	roleFlag, _ := f.GetString("role")
	cfgPath, _ := f.GetString("config")
	once, _ := f.GetBool("once")
	check, _ := f.GetBool("check")
	dry, _ := f.GetBool("dry-run")
	unsandboxed, _ := f.GetBool("allow-unsandboxed")
	if channel == "" || as == "" {
		return errors.New("--channel and --as are required")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root := ticket.FindRoot(cwd)
	if invite != "" {
		if err := busRunJoin(root, channel, as, invite, roleFlag); err != nil {
			return err
		}
	}
	h, err := busrun.Open(&busrun.Options{Root: root, Channel: channel, Name: as, ConfigPath: cfgPath, AllowUnsandboxed: unsandboxed, NoLock: check || dry})
	if err != nil {
		return err
	}
	defer h.Close()
	if err := h.Check(); err != nil {
		return err
	}
	for _, w := range h.Warnings() {
		fmt.Fprintln(os.Stderr, "warning: "+w)
	}
	switch {
	case check:
		if jsonOutput {
			return printBusJSON(map[string]any{"ok": true, "agent": h.Member().ID()})
		}
		fmt.Printf("ok: %s ready for tb bus run\n", h.Member().ID())
		return nil
	case dry:
		p, err := h.DryRun(cmd.Context())
		if err != nil {
			return err
		}
		return printBusJSON(p)
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Info("bus harness started", "agent", h.Member().ID(), "channel", channel, "once", once)
	return h.Run(ctx, once)
}

// Joins before running only when there is no session yet.
func busRunJoin(root, channel, name, invite, roleFlag string) error {
	s, err := bus.Open(root)
	if err != nil {
		return err
	}
	defer s.Close()
	if _, err := s.LoadMember(channel, name); err == nil {
		return nil
	}
	var role bus.Role
	if roleFlag != "" {
		if role, err = bus.ParseRole(roleFlag); err != nil {
			return err
		}
	}
	card := bus.AgentCard{Name: name}
	sess, err := s.Join(channel, invite, &card, role)
	if err != nil {
		return err
	}
	return bus.SaveSession(root, &sess)
}

func init() {
	addMemberFlags(busRunCmd)
	rf := busRunCmd.Flags()
	rf.String("invite", "", "joins with this invite before running, if there is no session yet")
	rf.String("role", "", "expected role on join")
	rf.String("config", "", "wake file (default .toolbelt/bus/wake/<channel>/<name>.yaml)")
	rf.Bool("once", false, "processes pending messages, wakes at most once and exits")
	rf.Bool("check", false, "validates config, session, binary and sandbox and exits")
	rf.Bool("dry-run", false, "prints the wake packet that would be delivered")
	rf.Bool("allow-unsandboxed", false, "opt-in to run the child without the Landlock sandbox")
	busCmd.AddCommand(busRunCmd)
}
