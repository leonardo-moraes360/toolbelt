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

// busMember authenticates --channel/--as against the CLI session.
func busMember(cmd *cobra.Command) (*bus.Store, bus.Member, error) {
	channel, _ := cmd.Flags().GetString("channel")
	as, _ := cmd.Flags().GetString("as")
	if channel == "" || as == "" {
		return nil, bus.Member{}, errors.New("--channel and --as are required")
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
	c.Flags().String("channel", "", "channel")
	c.Flags().String("as", "", "agent name (session created by tb bus join)")
}

// Indented JSON without escaping <, > and &.
func printBusJSON(v any) error {
	b, err := bus.MarshalIndent(v)
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(b))
	return err
}

// Accepts literal JSON or @file.
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
	Short: "Local bus between agents of any vendor (ADR-013, B1)",
	Long: `Local bus (.toolbelt/bus/bus.db, SQLite WAL) for agents from different
vendors to exchange bus/0.1 messages. Only the human creates channels and invites;
agents join with a single-use invite and a fixed role.

Example:
  tb bus channel create feature-x --owner claude-main --ttl 2h
  tb bus invite feature-x --role worker
  tb bus join feature-x --invite K7Q2-9XMD --name grok-1 --vendor xai
  tb bus send --channel feature-x --as claude-main --to role:worker --type task.assign --thread TKT-001
  tb bus recv --channel feature-x --as grok-1 --wait 20s`,
}

var busChannelCmd = &cobra.Command{
	Use:   "channel",
	Short: "Manages channels (human only)",
}

var busChannelCreateCmd = &cobra.Command{
	Use:   "create <canal>",
	Short: "Creates a channel and a single-use invite for the owner (supervisor)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		owner, _ := cmd.Flags().GetString("owner")
		ttl, _ := cmd.Flags().GetDuration("ttl")
		inviteTTL, _ := cmd.Flags().GetDuration("invite-ttl")
		if owner == "" {
			return errors.New("--owner is required")
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
			return printBusJSON(map[string]any{"channel": ch, "invite": inv})
		}
		fmt.Printf("channel: %s  owner: %s\n", ch.Name, ch.Owner)
		fmt.Printf("owner invite: %s (role %s, single-use, expires %s)\n", inv.Code, inv.Role, inv.ExpiresAt.Format("15:04:05Z07:00"))
		return nil
	},
}

var busChannelListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists channels",
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
			return printBusJSON(chs)
		}
		if len(chs) == 0 {
			fmt.Println("no channels")
		}
		for _, ch := range chs {
			exp := "-"
			if ch.ExpiresAt != nil {
				exp = ch.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
			}
			fmt.Printf("%s\towner=%s\texpires=%s\n", ch.Name, ch.Owner, exp)
		}
		return nil
	},
}

var busChannelShowCmd = &cobra.Command{
	Use:   "show <canal>",
	Short: "Shows agents registered in the channel",
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
			return printBusJSON(agents)
		}
		if len(agents) == 0 {
			fmt.Println("no agents in the channel")
		}
		for _, a := range agents {
			fmt.Printf("%s\trole=%s\tvendor=%s\twake=%s\n", a.AgentID, a.Role, a.Card.Vendor, a.Card.Wake)
		}
		return nil
	},
}

var busChannelExtendCmd = &cobra.Command{
	Use:   "extend <canal>",
	Short: "Resets the channel expiry to now + --ttl (reopens an expired channel)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ttl, _ := cmd.Flags().GetDuration("ttl")
		if !cmd.Flags().Changed("ttl") {
			return errors.New("--ttl is required (0 = no expiry)")
		}
		s, err := openBusStore()
		if err != nil {
			return err
		}
		defer s.Close()
		ch, err := s.ExtendChannel(args[0], ttl)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printBusJSON(ch)
		}
		exp := "no expiry"
		if ch.ExpiresAt != nil {
			exp = "expires " + ch.ExpiresAt.Local().Format("2006-01-02T15:04:05Z07:00")
		}
		fmt.Printf("channel: %s  %s\n", ch.Name, exp)
		return nil
	},
}

var busInviteCmd = &cobra.Command{
	Use:   "invite <canal>",
	Short: "Issues a single-use invite with a fixed role (human only)",
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
			return printBusJSON(inv)
		}
		fmt.Printf("invite: %s (channel %s, role %s, single-use, expires %s)\n", inv.Code, inv.Channel, inv.Role, inv.ExpiresAt.Format("15:04:05Z07:00"))
		return nil
	},
}

var busJoinCmd = &cobra.Command{
	Use:   "join <canal>",
	Short: "Joins the channel with an invite and an Agent Card",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		invite, _ := f.GetString("invite")
		roleFlag, _ := f.GetString("role")
		cardFile, _ := f.GetString("card")
		if invite == "" {
			return errors.New("--invite is required")
		}
		var card bus.AgentCard
		if cardFile != "" {
			b, err := os.ReadFile(cardFile)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(b, &card); err != nil {
				return fmt.Errorf("invalid agent card: %w", err)
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
			return fmt.Errorf("writing session: %w", err)
		}
		if jsonOutput {
			return printBusJSON(sess)
		}
		fmt.Printf("%s joined %s (role %s, %s)\n", sess.AgentID, sess.Channel, sess.Role, sess.Version)
		fmt.Printf("use --channel %s --as %s in the next commands\n", sess.Channel, sess.Name)
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
	Short: "Publishes a bus/0.1 message",
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
			return printBusJSON(map[string]any{"envelope": bus.Wrap(&res.Message), "duplicate": res.Duplicate})
		}
		dup := ""
		if res.Duplicate {
			dup = " (duplicate: already recorded)"
		}
		fmt.Printf("sent %s seq=%d %s → %s%s\n", res.Message.ID, res.Message.Seq, res.Message.Type, res.Message.To, dup)
		return nil
	},
}

var busRecvCmd = &cobra.Command{
	Use:   "recv",
	Short: "Reads the agent's queue (short long-poll with --wait)",
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
			return printBusJSON(bus.WrapAll(msgs))
		}
		printMessages(msgs, "no messages")
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
	Short: "Takes the thread/ticket as sole owner",
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
			return printBusJSON(c)
		}
		fmt.Printf("%s owns %s\n", c.Owner, c.Thread)
		return nil
	},
}

var busCompleteCmd = &cobra.Command{
	Use:   "complete <thread>",
	Short: "Completes the thread (owner only) and publishes task.result",
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
			return printBusJSON(map[string]any{"claim": res.Claim, "envelope": bus.Wrap(&res.Message)})
		}
		fmt.Printf("%s completed by %s (task.result %s)\n", res.Claim.Thread, res.Claim.Owner, res.Message.ID)
		return nil
	},
}

var busLogCmd = &cobra.Command{
	Use:   "log <canal>",
	Short: "Channel audit trail (payload redacted)",
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
			return printBusJSON(bus.WrapAll(msgs))
		}
		printMessages(msgs, "no messages in the channel")
		return nil
	},
}

func init() {
	busChannelCreateCmd.Flags().String("owner", "", "owner agent name (receives supervisor invite)")
	busChannelCreateCmd.Flags().Duration("ttl", 0, "channel duration (0 = no expiry)")
	busChannelCreateCmd.Flags().Duration("invite-ttl", bus.DefaultInviteTTL, "owner invite duration")
	busChannelExtendCmd.Flags().Duration("ttl", 0, "new duration from now (0 = no expiry)")
	busChannelCmd.AddCommand(busChannelCreateCmd, busChannelListCmd, busChannelShowCmd, busChannelExtendCmd)

	busInviteCmd.Flags().String("role", "worker", "fixed role: router|supervisor|worker|reviewer")
	busInviteCmd.Flags().Duration("ttl", bus.DefaultInviteTTL, "invite duration")
	busInviteCmd.Flags().String("name", "", "binds the invite to an agent; if it is already in the channel, replaces the lost session")

	jf := busJoinCmd.Flags()
	jf.String("invite", "", "invite code (e.g. K7Q2-9XMD)")
	jf.String("role", "", "expected role (must match the invite)")
	jf.String("card", "", "JSON file with the Agent Card")
	jf.String("name", "", "agent name (Agent Card)")
	jf.String("vendor", "", "vendor (Agent Card)")
	jf.String("model", "", "model (Agent Card)")
	jf.String("tier", "", "tier (Agent Card)")
	jf.String("wake", "", "exec|harness|notify|manual (default manual)")
	jf.String("session", "", "vendor session handle")
	jf.StringSlice("capability", nil, "capabilities (repeatable)")

	addMemberFlags(busSendCmd)
	sf := busSendCmd.Flags()
	sf.String("to", "", "destination: agent:<name> | role:<role> | topic:<name>")
	sf.String("type", "", "task.assign|task.status|query|reply|ack|urgent")
	sf.String("thread", "", "thread (ex.: TKT-001)")
	sf.String("causation", "", "ID of the message that caused this one")
	sf.String("idempotency-key", "", "dedupe key per sender")
	sf.String("ttl", "", "message duration (e.g. 30m)")
	sf.String("payload", "", "JSON object or @file (references, not blobs)")

	addMemberFlags(busRecvCmd)
	rf := busRecvCmd.Flags()
	rf.Duration("wait", 0, "long-poll up to this duration (max. 25s)")
	rf.Int("limit", 50, "maximum messages")
	rf.Bool("peek", false, "reads without advancing the cursor")
	rf.Int64("since", 0, "rereads from this seq (inclusive)")

	addMemberFlags(busClaimCmd)
	addMemberFlags(busCompleteCmd)
	busCompleteCmd.Flags().String("result", "", "result: JSON object or @file")

	busLogCmd.Flags().String("thread", "", "filters by thread")
	busLogCmd.Flags().Int("limit", 100, "last N messages")

	busCmd.AddCommand(busChannelCmd, busInviteCmd, busJoinCmd, busSendCmd, busRecvCmd, busClaimCmd, busCompleteCmd, busLogCmd)
	rootCmd.AddCommand(busCmd)
}
