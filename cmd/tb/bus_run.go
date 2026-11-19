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
	Short: "Harness: espera mensagens sem LLM e acorda o agente (B3)",
	Long: `Processo local, um por agente, que espera mensagens no store sem gastar
tokens e acorda o agente pelo adapter do arquivo de wake
(.toolbelt/bus/wake/<canal>/<nome>.yaml, modo 0600): exec (comando local com
o wake packet no stdin) ou harness (claude -p --resume). O wake packet só
tem cabeçalhos; o agente lê o conteúdo com recv. Spec: docs/bus-harness.md.`,
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
		return errors.New("--channel e --as são obrigatórios")
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
	h, err := busrun.Open(&busrun.Options{Root: root, Channel: channel, Name: as, ConfigPath: cfgPath, AllowUnsandboxed: unsandboxed})
	if err != nil {
		return err
	}
	defer h.Close()
	if err := h.Check(); err != nil {
		return err
	}
	switch {
	case check:
		if jsonOutput {
			return printBusJSON(map[string]any{"ok": true, "agent": h.Member().ID()})
		}
		fmt.Printf("ok: %s pronto para tb bus run\n", h.Member().ID())
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
	log.Info("harness do bus iniciado", "agent", h.Member().ID(), "channel", channel, "once", once)
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
	rf.String("invite", "", "faz join com este convite antes de rodar, se ainda não houver sessão")
	rf.String("role", "", "papel esperado no join")
	rf.String("config", "", "arquivo de wake (padrão .toolbelt/bus/wake/<canal>/<nome>.yaml)")
	rf.Bool("once", false, "processa o pendente, faz no máximo um despertar e sai")
	rf.Bool("check", false, "valida config, sessão, binário e sandbox e sai")
	rf.Bool("dry-run", false, "imprime o wake packet que seria entregue")
	rf.Bool("allow-unsandboxed", false, "opt-in para rodar o filho sem sandbox Landlock")
	busCmd.AddCommand(busRunCmd)
}
