package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

func openBusStore() (*bus.Store, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return bus.Open(ticket.FindRoot(cwd))
}

// busMember abre o store e autentica --channel/--as pela sessão CLI.
func busMember(cmd *cobra.Command) (*bus.Store, bus.Member, error) {
	channel, _ := cmd.Flags().GetString("channel")
	as, _ := cmd.Flags().GetString("as")
	if channel == "" || as == "" {
		return nil, bus.Member{}, errors.New("--channel e --as são obrigatórios")
	}
	s, err := openBusStore()
	if err != nil {
		return nil, bus.Member{}, err
	}
	m, err := s.LoadMember(channel, as)
	if err != nil {
		_ = s.Close()
		return nil, bus.Member{}, err
	}
	return s, m, nil
}

func addMemberFlags(c *cobra.Command) {
	c.Flags().String("channel", "", "canal")
	c.Flags().String("as", "", "nome do agente (sessão criada por tb bus join)")
}

// readPayload aceita JSON literal ou @arquivo.
func readPayload(v string) (json.RawMessage, error) {
	if v == "" {
		return nil, nil
	}
	if strings.HasPrefix(v, "@") {
		b, err := os.ReadFile(v[1:])
		if err != nil {
			return nil, err
		}
		return b, nil
	}
	return json.RawMessage(v), nil
}

var busCmd = &cobra.Command{
	Use:   "bus",
	Short: "Barramento local entre agentes de qualquer vendor (ADR-013, B1)",
	Long: `Barramento local (.toolbelt/bus/bus.db, SQLite WAL) para agentes de vendors
diferentes trocarem mensagens bus/0.1. Só o humano cria canais e convites;
agentes entram com convite de uso único e papel fixado.

Exemplo:
  tb bus channel create feature-x --owner claude-main --ttl 2h
  tb bus invite feature-x --role worker
  tb bus join feature-x --invite K7Q2-9XMD --name grok-1 --vendor xai
  tb bus send --channel feature-x --as claude-main --to role:worker --type task.assign --thread TKT-001
  tb bus recv --channel feature-x --as grok-1 --wait 20s`,
}

var busChannelCmd = &cobra.Command{
	Use:   "channel",
	Short: "Gerencia canais (só humano)",
}

var busChannelCreateCmd = &cobra.Command{
	Use:   "create <canal>",
	Short: "Cria canal e convite de uso único para o dono (supervisor)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		owner, _ := cmd.Flags().GetString("owner")
		ttl, _ := cmd.Flags().GetDuration("ttl")
		inviteTTL, _ := cmd.Flags().GetDuration("invite-ttl")
		if owner == "" {
			return errors.New("--owner é obrigatório")
		}
		s, err := openBusStore()
		if err != nil {
			return err
		}
		defer s.Close()
		ch, inv, err := s.CreateChannel(args[0], owner, ttl, inviteTTL)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"channel": ch, "invite": inv})
		}
		fmt.Printf("canal: %s  dono: %s\n", ch.Name, ch.Owner)
		fmt.Printf("convite do dono: %s (papel %s, uso único, expira %s)\n", inv.Code, inv.Role, inv.ExpiresAt.Format("15:04:05Z07:00"))
		return nil
	},
}

var busChannelListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista canais",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openBusStore()
		if err != nil {
			return err
		}
		defer s.Close()
		chs, err := s.Channels()
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(chs)
		}
		if len(chs) == 0 {
			fmt.Println("nenhum canal")
		}
		for _, ch := range chs {
			exp := "-"
			if ch.ExpiresAt != nil {
				exp = ch.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
			}
			fmt.Printf("%s\tdono=%s\texpira=%s\n", ch.Name, ch.Owner, exp)
		}
		return nil
	},
}

var busChannelShowCmd = &cobra.Command{
	Use:   "show <canal>",
	Short: "Mostra agentes registrados no canal",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openBusStore()
		if err != nil {
			return err
		}
		defer s.Close()
		agents, err := s.Agents(args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(agents)
		}
		if len(agents) == 0 {
			fmt.Println("nenhum agente no canal")
		}
		for _, a := range agents {
			fmt.Printf("%s\tpapel=%s\tvendor=%s\twake=%s\n", a.AgentID, a.Role, a.Card.Vendor, a.Card.Wake)
		}
		return nil
	},
}

var busInviteCmd = &cobra.Command{
	Use:   "invite <canal>",
	Short: "Emite convite de uso único com papel fixado (só humano)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		roleFlag, _ := cmd.Flags().GetString("role")
		ttl, _ := cmd.Flags().GetDuration("ttl")
		name, _ := cmd.Flags().GetString("name")
		role, err := bus.ParseRole(roleFlag)
		if err != nil {
			return err
		}
		s, err := openBusStore()
		if err != nil {
			return err
		}
		defer s.Close()
		inv, err := s.CreateInvite(args[0], role, name, ttl)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(inv)
		}
		fmt.Printf("convite: %s (canal %s, papel %s, uso único, expira %s)\n", inv.Code, inv.Channel, inv.Role, inv.ExpiresAt.Format("15:04:05Z07:00"))
		return nil
	},
}

var busJoinCmd = &cobra.Command{
	Use:   "join <canal>",
	Short: "Entra no canal com convite e Agent Card",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		invite, _ := f.GetString("invite")
		roleFlag, _ := f.GetString("role")
		cardFile, _ := f.GetString("card")
		if invite == "" {
			return errors.New("--invite é obrigatório")
		}
		var card bus.AgentCard
		if cardFile != "" {
			b, err := os.ReadFile(cardFile)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(b, &card); err != nil {
				return fmt.Errorf("agent card inválido: %w", err)
			}
		}
		overrideString(f.Changed("name"), &card.Name, f, "name")
		overrideString(f.Changed("vendor"), &card.Vendor, f, "vendor")
		overrideString(f.Changed("model"), &card.Model, f, "model")
		overrideString(f.Changed("tier"), &card.Tier, f, "tier")
		overrideString(f.Changed("session"), &card.Session, f, "session")
		if f.Changed("wake") {
			w, _ := f.GetString("wake")
			card.Wake = bus.Wake(w)
		}
		if f.Changed("capability") {
			card.Capabilities, _ = f.GetStringSlice("capability")
		}
		var role bus.Role
		if roleFlag != "" {
			r, err := bus.ParseRole(roleFlag)
			if err != nil {
				return err
			}
			role = r
		}
		s, err := openBusStore()
		if err != nil {
			return err
		}
		defer s.Close()
		sess, err := s.Join(args[0], invite, &card, role)
		if err != nil {
			return err
		}
		if err := bus.SaveSession(s.Root(), &sess); err != nil {
			return fmt.Errorf("gravando sessão: %w", err)
		}
		if jsonOutput {
			return printJSON(sess)
		}
		fmt.Printf("%s entrou em %s (papel %s, %s)\n", sess.AgentID, sess.Channel, sess.Role, sess.Version)
		fmt.Printf("use --channel %s --as %s nos próximos comandos\n", sess.Channel, sess.Name)
		return nil
	},
}

func overrideString(changed bool, dst *string, f interface {
	GetString(string) (string, error)
}, name string) {
	if changed {
		*dst, _ = f.GetString(name)
	}
}

var busSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Publica mensagem bus/0.1",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		to, _ := f.GetString("to")
		typ, _ := f.GetString("type")
		thread, _ := f.GetString("thread")
		causation, _ := f.GetString("causation")
		idem, _ := f.GetString("idempotency-key")
		ttl, _ := f.GetString("ttl")
		payloadFlag, _ := f.GetString("payload")
		payload, err := readPayload(payloadFlag)
		if err != nil {
			return err
		}
		s, m, err := busMember(cmd)
		if err != nil {
			return err
		}
		defer s.Close()
		res, err := s.Send(m, &bus.SendInput{
			To: to, Type: bus.Type(typ), Thread: thread, CausationID: causation,
			IdempotencyKey: idem, TTL: ttl, Payload: payload,
		})
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"envelope": bus.Wrap(&res.Message), "duplicate": res.Duplicate})
		}
		dup := ""
		if res.Duplicate {
			dup = " (duplicata: já gravada)"
		}
		fmt.Printf("enviada %s seq=%d %s → %s%s\n", res.Message.ID, res.Message.Seq, res.Message.Type, res.Message.To, dup)
		return nil
	},
}

var busRecvCmd = &cobra.Command{
	Use:   "recv",
	Short: "Lê a fila do agente (long-poll curto com --wait)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		wait, _ := f.GetDuration("wait")
		limit, _ := f.GetInt("limit")
		peek, _ := f.GetBool("peek")
		since, _ := f.GetInt64("since")
		s, m, err := busMember(cmd)
		if err != nil {
			return err
		}
		defer s.Close()
		msgs, err := s.Recv(cmd.Context(), m, bus.RecvOptions{Wait: wait, Limit: limit, Peek: peek, Since: since})
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(bus.WrapAll(msgs))
		}
		printMessages(msgs, "nenhuma mensagem")
		return nil
	},
}

func printMessages(msgs []bus.Message, empty string) {
	if len(msgs) == 0 {
		fmt.Println(empty)
		return
	}
	for i := range msgs {
		m := &msgs[i]
		thread := ""
		if m.Thread != "" {
			thread = " [" + m.Thread + "]"
		}
		fmt.Printf("#%d %s %s → %s %s%s %s\n", m.Seq, m.CreatedAt.Format("15:04:05"), m.From, m.To, m.Type, thread, m.Payload)
	}
}

var busClaimCmd = &cobra.Command{
	Use:   "claim <thread>",
	Short: "Assume a thread/ticket como dono único",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, m, err := busMember(cmd)
		if err != nil {
			return err
		}
		defer s.Close()
		c, err := s.ClaimThread(m, args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(c)
		}
		fmt.Printf("%s é dono de %s\n", c.Owner, c.Thread)
		return nil
	},
}

var busCompleteCmd = &cobra.Command{
	Use:   "complete <thread>",
	Short: "Conclui a thread (só o dono) e publica task.result",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resultFlag, _ := cmd.Flags().GetString("result")
		result, err := readPayload(resultFlag)
		if err != nil {
			return err
		}
		s, m, err := busMember(cmd)
		if err != nil {
			return err
		}
		defer s.Close()
		res, err := s.Complete(m, args[0], result)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(map[string]any{"claim": res.Claim, "envelope": bus.Wrap(&res.Message)})
		}
		fmt.Printf("%s concluída por %s (task.result %s)\n", res.Claim.Thread, res.Claim.Owner, res.Message.ID)
		return nil
	},
}

var busLogCmd = &cobra.Command{
	Use:   "log <canal>",
	Short: "Trilha de auditoria do canal (payload redigido)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		thread, _ := cmd.Flags().GetString("thread")
		limit, _ := cmd.Flags().GetInt("limit")
		s, err := openBusStore()
		if err != nil {
			return err
		}
		defer s.Close()
		msgs, err := s.Log(args[0], thread, limit)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(bus.WrapAll(msgs))
		}
		printMessages(msgs, "nenhuma mensagem no canal")
		return nil
	},
}

func init() {
	busChannelCreateCmd.Flags().String("owner", "", "nome do agente dono (recebe convite supervisor)")
	busChannelCreateCmd.Flags().Duration("ttl", 0, "duração do canal (0 = sem expiração)")
	busChannelCreateCmd.Flags().Duration("invite-ttl", bus.DefaultInviteTTL, "duração do convite do dono")
	busChannelCmd.AddCommand(busChannelCreateCmd, busChannelListCmd, busChannelShowCmd)

	busInviteCmd.Flags().String("role", "worker", "papel fixado: router|supervisor|worker|reviewer")
	busInviteCmd.Flags().Duration("ttl", bus.DefaultInviteTTL, "duração do convite")
	busInviteCmd.Flags().String("name", "", "vincula o convite a um agente; se ele já estiver no canal, substitui a sessão perdida")

	jf := busJoinCmd.Flags()
	jf.String("invite", "", "código do convite (ex.: K7Q2-9XMD)")
	jf.String("role", "", "papel esperado (precisa coincidir com o convite)")
	jf.String("card", "", "arquivo JSON com o Agent Card")
	jf.String("name", "", "nome do agente (Agent Card)")
	jf.String("vendor", "", "vendor (Agent Card)")
	jf.String("model", "", "modelo (Agent Card)")
	jf.String("tier", "", "tier (Agent Card)")
	jf.String("wake", "", "harness|notify|manual (padrão manual)")
	jf.String("session", "", "handle de sessão do vendor")
	jf.StringSlice("capability", nil, "capabilities (repetível)")

	addMemberFlags(busSendCmd)
	sf := busSendCmd.Flags()
	sf.String("to", "", "destino: agent:<nome> | role:<papel> | topic:<nome>")
	sf.String("type", "", "task.assign|task.status|query|reply|ack|urgent")
	sf.String("thread", "", "thread (ex.: TKT-001)")
	sf.String("causation", "", "ID da mensagem que causou esta")
	sf.String("idempotency-key", "", "chave de dedupe por remetente")
	sf.String("ttl", "", "duração da mensagem (ex.: 30m)")
	sf.String("payload", "", "objeto JSON ou @arquivo (referências, não blobs)")

	addMemberFlags(busRecvCmd)
	rf := busRecvCmd.Flags()
	rf.Duration("wait", 0, "long-poll até esta duração (máx. 25s)")
	rf.Int("limit", 50, "máximo de mensagens")
	rf.Bool("peek", false, "lê sem avançar o cursor")
	rf.Int64("since", 0, "relê a partir deste seq (inclusive)")

	addMemberFlags(busClaimCmd)
	addMemberFlags(busCompleteCmd)
	busCompleteCmd.Flags().String("result", "", "resultado: objeto JSON ou @arquivo")

	busLogCmd.Flags().String("thread", "", "filtra por thread")
	busLogCmd.Flags().Int("limit", 100, "últimas N mensagens")

	busCmd.AddCommand(busChannelCmd, busInviteCmd, busJoinCmd, busSendCmd, busRecvCmd, busClaimCmd, busCompleteCmd, busLogCmd)
	rootCmd.AddCommand(busCmd)
}
