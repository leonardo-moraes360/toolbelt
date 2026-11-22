package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/envelope"
	"github.com/leonardo-moraes360/toolbelt/internal/review"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

func openTicketStore() (*ticket.Store, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return ticket.Open(ticket.FindRoot(cwd)), nil
}

var ticketCmd = &cobra.Command{
	Use:   "ticket",
	Short: "Work tickets for agents (.toolbelt/tickets)",
}

var ticketCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates a ticket (--title or --from rd.md)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		title, _ := f.GetString("title")
		from, _ := f.GetString("from")
		desc, _ := f.GetString("description")
		scope, _ := f.GetStringSlice("scope")
		forbid, _ := f.GetStringSlice("forbid")
		accept, _ := f.GetStringArray("accept")
		deps, _ := f.GetStringSlice("dep")
		prio, _ := f.GetInt("priority")
		timeout, _ := f.GetString("timeout")
		dry, _ := f.GetBool("dry-run")

		t := ticket.Ticket{Title: title, Description: desc, Priority: prio}
		if from != "" {
			sc, err := review.LoadTicket(from)
			if err != nil {
				return err
			}
			if t.Title == "" {
				t.Title = sc.Title
			}
			if t.Description == "" {
				t.Description = sc.Description
			}
			t.Mandate.Scope = append(t.Mandate.Scope, sc.Scope...)
			t.Mandate.Forbidden = append(t.Mandate.Forbidden, sc.Forbidden...)
			t.Acceptance.Criteria = append(t.Acceptance.Criteria, sc.Acceptance...)
			t.Source = from
		}
		t.Mandate.Scope = append(t.Mandate.Scope, scope...)
		t.Mandate.Forbidden = append(t.Mandate.Forbidden, forbid...)
		t.Acceptance.Criteria = append(t.Acceptance.Criteria, accept...)
		t.Acceptance.Timeout = timeout
		t.Dependencies = deps

		s, err := openTicketStore()
		if err != nil {
			return err
		}
		if dry {
			if err := t.Validate(); err != nil {
				return err
			}
			if err := s.CheckNew(&t); err != nil {
				return err
			}
			if jsonOutput {
				return emitJSONIndent(envelope.Ticket, t)
			}
			fmt.Printf("dry-run: ticket %q would be created (valid)\n", t.Title)
			return nil
		}
		created, err := s.Create(&t)
		if err != nil {
			return err
		}
		if jsonOutput {
			return emitJSONIndent(envelope.Ticket, created)
		}
		fmt.Printf("ticket %s created: %s\n", created.ID, created.Title)
		return nil
	},
}

var ticketListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists tickets",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		statuses, _ := cmd.Flags().GetString("status")
		dep, _ := cmd.Flags().GetString("deps")
		f := ticket.Filter{DependsOn: dep}
		sts, err := parseStatusList(statuses)
		if err != nil {
			return err
		}
		f.Statuses = sts
		s, err := openTicketStore()
		if err != nil {
			return err
		}
		list, err := s.List(f)
		if err != nil {
			return err
		}
		if jsonOutput {
			if list == nil {
				list = []ticket.Ticket{}
			}
			return emitJSONIndent(envelope.TicketList, list)
		}
		if len(list) == 0 {
			fmt.Println("no tickets")
			return nil
		}
		for i := range list {
			fmt.Println(ticketLine(&list[i]))
		}
		return nil
	},
}

var ticketShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Shows a ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openTicketStore()
		if err != nil {
			return err
		}
		t, err := s.Get(args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			return emitJSONIndent(envelope.Ticket, t)
		}
		printTicketDetail(&t)
		return nil
	},
}

var ticketCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "Cancels a pending/running ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openTicketStore()
		if err != nil {
			return err
		}
		t, err := s.Cancel(args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			return emitJSONIndent(envelope.Ticket, t)
		}
		fmt.Printf("ticket %s canceled\n", t.ID)
		return nil
	},
}

var ticketRetryCmd = &cobra.Command{
	Use:   "retry <id>",
	Short: "Puts a failed/canceled ticket back to pending",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := openTicketStore()
		if err != nil {
			return err
		}
		t, err := s.Retry(args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			return emitJSONIndent(envelope.Ticket, t)
		}
		fmt.Printf("ticket %s returned to pending\n", t.ID)
		return nil
	},
}

func parseStatusList(s string) ([]ticket.Status, error) {
	var out []ticket.Status
	for _, p := range strings.Split(s, ",") {
		if strings.TrimSpace(p) == "" {
			continue
		}
		st, err := ticket.ParseStatus(p)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

func ticketLine(t *ticket.Ticket) string {
	line := fmt.Sprintf("%s\t%-9s\tp%d\t%s", t.ID, t.Status, t.Priority, t.Title)
	if len(t.Dependencies) > 0 {
		line += "\t(deps: " + strings.Join(t.Dependencies, ",") + ")"
	}
	if t.Worker != "" {
		line += "\t[" + t.Worker + "]"
	}
	return line
}

func printTicketDetail(t *ticket.Ticket) {
	fmt.Printf("%s  %s\n", t.ID, t.Title)
	fmt.Printf("status:      %s\n", t.Status)
	fmt.Printf("priority:    %d\n", t.Priority)
	fmt.Printf("created:     %s\n", t.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("updated:     %s\n", t.UpdatedAt.Format("2006-01-02 15:04:05"))
	if t.Source != "" {
		fmt.Printf("source:      %s\n", t.Source)
	}
	if len(t.Dependencies) > 0 {
		fmt.Printf("deps:        %s\n", strings.Join(t.Dependencies, ", "))
	}
	if t.Worker != "" {
		fmt.Printf("worker:      %s (attempts: %d)\n", t.Worker, t.Attempts)
	} else if t.Attempts > 0 {
		fmt.Printf("attempts:    %d\n", t.Attempts)
	}
	if t.Acceptance.Timeout != "" {
		fmt.Printf("timeout:     %s\n", t.Acceptance.Timeout)
	}
	printList("scope", t.Mandate.Scope)
	printList("forbidden", t.Mandate.Forbidden)
	printList("permissions", t.Mandate.Permissions)
	printList("acceptance", t.Acceptance.Criteria)
	if t.Subscription != nil {
		fmt.Printf("signature:   %s until %s\n", t.Subscription.Worker, t.Subscription.ExpiresAt.Format("15:04:05"))
	}
	if r := t.Result; r != nil {
		fmt.Printf("result:      exit=%d branch=%s head=%s\n", r.ExitCode, r.Branch, r.HeadCommit)
		if r.Error != "" {
			fmt.Printf("error:       %s\n", r.Error)
		}
	}
	if t.MergedAt != nil {
		fmt.Printf("merge:       %s\n", t.MergedAt.Format("2006-01-02 15:04:05"))
	}
	if t.Description != "" {
		fmt.Printf("\n%s\n", t.Description)
	}
}

func printList(label string, items []string) {
	if len(items) > 0 {
		fmt.Printf("%-12s %s\n", label+":", strings.Join(items, "; "))
	}
}

func init() {
	c := ticketCreateCmd.Flags()
	c.String("title", "", "ticket title")
	c.String("from", "", "rd.md file / ticket with scope and acceptance")
	c.String("description", "", "description")
	c.StringSlice("scope", nil, "allowed paths (repeatable)")
	c.StringSlice("forbid", nil, "forbidden paths (repeatable)")
	c.StringArray("accept", nil, "acceptance criterion (repeatable)")
	c.StringSlice("dep", nil, "ID of the ticket this one depends on (repeatable)")
	c.Int("priority", 0, "priority (higher first)")
	c.String("timeout", "", "execution timeout (e.g. 10m)")
	c.Bool("dry-run", false, "validates without saving")

	ticketListCmd.Flags().String("status", "", "filters by status (comma-separated list)")
	ticketListCmd.Flags().String("deps", "", "only tickets that depend on this ID")

	ticketCmd.AddCommand(ticketCreateCmd, ticketListCmd, ticketShowCmd, ticketCancelCmd, ticketRetryCmd)
}
