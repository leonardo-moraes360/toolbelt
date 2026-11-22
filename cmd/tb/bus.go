package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

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
	Use:   "create <channel>",
	Short: "Creates a channel and a single-use invite for the owner (supervisor)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		owner, _ := cmd.Flags().GetString("owner")
		ttl, _ := cmd.Flags().GetDuration("ttl")
		inviteTTL, _ := cmd.Flags().GetDuration("invite-ttl")
		lease, _ := cmd.Flags().GetDuration("lease")
		if owner == "" {
			return errors.New("--owner is required")
		}
		s, err := openBusStore()
		if err != nil {
			return err
		}
		defer s.Close()
		ch, inv, err := s.CreateChannelLease(args[0], owner, ttl, inviteTTL, lease)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printBusJSON(map[string]any{"channel": ch, "invite": inv})
		}
		fmt.Printf("channel: %s  owner: %s  lease: %s\n", ch.Name, ch.Owner, ch.Lease)
		fmt.Printf("owner invite: %s (role %s, single-use, expires %s)\n", inv.Code, inv.Role, inv.ExpiresAt.Local().Format(time.RFC3339))
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
	Use:   "show <channel>",
	Short: "Shows the channel's agents (alive, last_seen) and open claims (owner, epoch, state)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openBusStore()
		if err != nil {
			return err
		}
		defer s.Close()
		st, err := s.Status(args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			return printBusJSON(st) // no Agent Cards: their session is a vendor resume handle
		}
		fmt.Printf("lease: %s\n", st.Lease)
		if len(st.Agents) == 0 {
			fmt.Println("no agents in the channel")
		}
		for _, a := range st.Agents {
			fmt.Printf("agent:%s\trole=%s\talive=%t\tlast_seen=%s\n", a.Name, a.Role, a.Alive, a.LastSeen.Format(time.RFC3339))
		}
		for _, c := range st.Claims {
			outcome := "-"
			if c.Outcome != nil {
				outcome = *c.Outcome
			}
			fmt.Printf("claim %s\towner=%s\tepoch=%d\t%s\toutcome=%s\n", c.Thread, c.Owner, c.Epoch, c.State, outcome)
		}
		return nil
	},
}

var busChannelExtendCmd = &cobra.Command{
	Use:   "extend <channel>",
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
	Use:   "invite <channel>",
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
		fmt.Printf("invite: %s (channel %s, role %s, single-use, expires %s)\n", inv.Code, inv.Channel, inv.Role, inv.ExpiresAt.Local().Format(time.RFC3339))
		return nil
	},
}

var busJoinCmd = &cobra.Command{
	Use:   "join <channel>",
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

// warnBusDependencies: the payload is opaque to the broker, so a task.assign
// that names dependencies is not ordered by the bus (tb router orders by
// ticket dependencies).
func warnBusDependencies(typ bus.Type, payload []byte) {
	if typ != bus.TypeTaskAssign || len(payload) == 0 {
		return
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(payload, &obj) != nil {
		return
	}
	for _, k := range []string{"deps", "dependencies"} {
		if _, ok := obj[k]; ok {
			fmt.Fprintf(os.Stderr, "warning: the bus does not enforce %q in a task.assign payload (the payload is opaque to the broker); order dependent work with tb router and ticket dependencies, or send the next task when the first completes\n", k)
			return
		}
	}
}

var busSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Publishes a bus/0.1 message",
	Long: `Publishes a bus/0.1 message.

The payload is opaque to the broker: the bus never reads, validates or orders
by it. A task.assign payload with "deps" or "dependencies" is not enforced
(a warning is printed); dependency order between tasks is tb router's job,
from ticket dependencies.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		to, _ := f.GetString("to")
		typ, _ := f.GetString("type")
		thread, _ := f.GetString("thread")
		parent, _ := f.GetString("parent")
		causation, _ := f.GetString("causation")
		idem, _ := f.GetString("idempotency-key")
		ttl, _ := f.GetString("ttl")
		payloadFlag, _ := f.GetString("payload")
		payload, err := readPayload(payloadFlag)
		if err != nil {
			return err
		}
		warnBusDependencies(bus.Type(typ), payload)
		s, m, err := busMember(cmd)
		if err != nil {
			return err
		}
		defer s.Close()
		res, err := s.Send(m, &bus.SendInput{
			To: to, Type: bus.Type(typ), Thread: thread, Parent: parent, CausationID: causation,
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
		fmt.Printf("%s owns %s (epoch %d: pass it to complete)\n", c.Owner, c.Thread, c.Epoch)
		return nil
	},
}

var busCompleteCmd = &cobra.Command{
	Use:   "complete <thread>",
	Short: "Completes the thread (owner only) and publishes task.result",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resultFlag, _ := cmd.Flags().GetString("result")
		epoch, _ := cmd.Flags().GetInt64("epoch")
		if epoch <= 0 {
			return errors.New("--epoch is required: the epoch claim returned (docs/bus-lease.md)")
		}
		result, err := readPayload(resultFlag)
		if err != nil {
			return err
		}
		s, m, err := busMember(cmd)
		if err != nil {
			return err
		}
		defer s.Close()
		res, err := s.Complete(m, args[0], epoch, result)
		if err != nil {
			return err
		}
		if len(res.OpenChildren) > 0 {
			fmt.Fprintf(os.Stderr, "warning: %s completed with %d open children (%s)\n",
				res.Claim.Thread, len(res.OpenChildren), strings.Join(res.OpenChildren, ", "))
		}
		if jsonOutput {
			out := map[string]any{"claim": res.Claim, "envelope": bus.Wrap(&res.Message)}
			if len(res.OpenChildren) > 0 {
				out["open_children"] = res.OpenChildren
			}
			return printBusJSON(out)
		}
		fmt.Printf("%s completed by %s (task.result %s)\n", res.Claim.Thread, res.Claim.Owner, res.Message.ID)
		return nil
	},
}

var busHeartbeatCmd = &cobra.Command{
	Use:   "heartbeat",
	Short: "Renews the agent's lease; lists held threads (with epochs) and lost ones",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, m, err := busMember(cmd)
		if err != nil {
			return err
		}
		defer s.Close()
		hb, err := s.Heartbeat(m)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printBusJSON(hb)
		}
		fmt.Printf("lease ends %s (heartbeat every %s)\n", hb.LeaseEndsAt.Format(time.RFC3339), hb.HeartbeatInterval)
		for _, h := range hb.Held {
			fmt.Printf("held %s\tepoch=%d\n", h.Thread, h.Epoch)
		}
		for _, l := range hb.Lost {
			fmt.Printf("lost %s\tnew_owner=%s\n", l.Thread, l.NewOwner)
		}
		return nil
	},
}

var busWatchdogCmd = &cobra.Command{
	Use:   "watchdog <channel>",
	Short: "Releases claims of agents whose lease ended and publishes lease_expired to topic:board (human/router only)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dry, _ := cmd.Flags().GetBool("dry-run")
		s, err := openBusStore()
		if err != nil {
			return err
		}
		defer s.Close()
		exp, err := s.Watchdog(args[0], dry)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printBusJSON(map[string]any{"dry_run": dry, "expired": exp})
		}
		if len(exp) == 0 {
			fmt.Println("no expired claims")
		}
		verb := "released"
		if dry {
			verb = "would release"
		}
		for _, e := range exp {
			fmt.Printf("%s %s\tprevious_owner=%s\tepoch=%d\tlast_seen=%s\n", verb, e.Thread, e.PreviousOwner, e.Epoch, e.LastSeen.Format(time.RFC3339))
		}
		return nil
	},
}

var busLogCmd = &cobra.Command{
	Use:   "log <channel>",
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

var busTreeCmd = &cobra.Command{
	Use:   "tree <channel>",
	Short: "Delegation trees of a channel: threads linked by task.assign --parent, with their state (human only, no payloads)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		thread, _ := f.GetString("thread")
		all, _ := f.GetBool("all")
		limit, _ := f.GetInt("limit")
		s, err := openBusStore()
		if err != nil {
			return err
		}
		defer s.Close()
		tr, err := s.Tree(args[0], thread, all, limit)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printBusJSON(tr)
		}
		if len(tr.Nodes) == 0 {
			fmt.Println("no delegation trees (--all also lists threads without children)")
			return nil
		}
		fmt.Print(formatTree(tr.Nodes))
		return nil
	},
}

// formatTree prints one node per line, indented two spaces per level below the
// first node of each tree, with the columns aligned.
func formatTree(nodes []bus.TreeNode) string {
	base := nodes[0].Depth
	labels := make([]string, len(nodes))
	var wThread, wState, wOwner int
	for i := range nodes {
		n := &nodes[i]
		if n.Parent == "" {
			base = n.Depth
		}
		labels[i] = strings.Repeat("  ", max(n.Depth-base, 0)) + n.Thread
		wThread, wState, wOwner = max(wThread, len(labels[i])), max(wState, len(n.State)), max(wOwner, len(n.Owner))
	}
	var sb strings.Builder
	for i := range nodes {
		n := &nodes[i]
		line := fmt.Sprintf("%-*s  %-*s  %-*s", wThread, labels[i], wState, n.State, wOwner, n.Owner)
		if n.Epoch > 0 {
			line += fmt.Sprintf("  epoch %d", n.Epoch)
		}
		if n.OpenChildren > 0 {
			line += fmt.Sprintf("  open %d", n.OpenChildren)
		}
		sb.WriteString(strings.TrimRight(line, " ") + "\n")
	}
	return sb.String()
}

func init() {
	busChannelCreateCmd.Flags().String("owner", "", "owner agent name (receives supervisor invite)")
	busChannelCreateCmd.Flags().Duration("ttl", 0, "channel duration (0 = no expiry)")
	busChannelCreateCmd.Flags().Duration("invite-ttl", bus.DefaultInviteTTL, "owner invite duration")
	busChannelCreateCmd.Flags().Duration("lease", bus.DefaultLease, "agent lease: a claim whose owner has not heartbeated for this long can be taken over (30s to 1h)")
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
	sf.String("parent", "", "task.assign only: parent thread you hold, linking this sub-task into its delegation tree (max depth 3, 8 open children)")
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
	busCompleteCmd.Flags().Int64("epoch", 0, "epoch returned by claim (required: fences a stale owner)")
	addMemberFlags(busHeartbeatCmd)
	busWatchdogCmd.Flags().Bool("dry-run", false, "lists expired claims without releasing them")

	busLogCmd.Flags().String("thread", "", "filters by thread")
	busLogCmd.Flags().Int("limit", 100, "last N messages")

	busTreeCmd.Flags().String("thread", "", "only the subtree rooted at this thread")
	busTreeCmd.Flags().Bool("all", false, "also list threads without children")
	busTreeCmd.Flags().Int("limit", bus.DefaultTreeLimit, "at most N trees, newest first")

	busCmd.AddCommand(busChannelCmd, busInviteCmd, busJoinCmd, busSendCmd, busRecvCmd, busClaimCmd, busCompleteCmd, busHeartbeatCmd, busWatchdogCmd, busLogCmd, busTreeCmd)
	rootCmd.AddCommand(busCmd)
}
