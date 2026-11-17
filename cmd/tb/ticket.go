package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

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

func printJSONLine(v any) error { return json.NewEncoder(os.Stdout).Encode(v) }

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

var ticketCmd = &cobra.Command{
	Use:   "ticket",
	Short: "Tickets de trabalho para agentes (.toolbelt/tickets)",
}

var ticketCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Cria ticket (--title ou --from rd.md)",
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
				return printJSON(t)
			}
			fmt.Printf("dry-run: ticket %q seria criado (válido)\n", t.Title)
			return nil
		}
		created, err := s.Create(&t)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(created)
		}
		fmt.Printf("ticket %s criado: %s\n", created.ID, created.Title)
		return nil
	},
}

var ticketListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista tickets",
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
			return printJSON(list)
		}
		if len(list) == 0 {
			fmt.Println("nenhum ticket")
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
	Short: "Mostra um ticket",
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
			return printJSON(t)
		}
		printTicketDetail(&t)
		return nil
	},
}

var ticketCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "Cancela ticket pending/running",
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
			return printJSON(t)
		}
		fmt.Printf("ticket %s cancelado\n", t.ID)
		return nil
	},
}

var ticketRetryCmd = &cobra.Command{
	Use:   "retry <id>",
	Short: "Recoloca ticket failed/canceled em pending",
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
			return printJSON(t)
		}
		fmt.Printf("ticket %s voltou para pending\n", t.ID)
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
	fmt.Printf("prioridade:  %d\n", t.Priority)
	fmt.Printf("criado:      %s\n", t.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("atualizado:  %s\n", t.UpdatedAt.Format("2006-01-02 15:04:05"))
	if t.Source != "" {
		fmt.Printf("origem:      %s\n", t.Source)
	}
	if len(t.Dependencies) > 0 {
		fmt.Printf("deps:        %s\n", strings.Join(t.Dependencies, ", "))
	}
	if t.Worker != "" {
		fmt.Printf("worker:      %s (tentativas: %d)\n", t.Worker, t.Attempts)
	} else if t.Attempts > 0 {
		fmt.Printf("tentativas:  %d\n", t.Attempts)
	}
	if t.Acceptance.Timeout != "" {
		fmt.Printf("timeout:     %s\n", t.Acceptance.Timeout)
	}
	printList("escopo", t.Mandate.Scope)
	printList("proibido", t.Mandate.Forbidden)
	printList("permissões", t.Mandate.Permissions)
	printList("aceite", t.Acceptance.Criteria)
	if t.Subscription != nil {
		fmt.Printf("assinatura:  %s até %s\n", t.Subscription.Worker, t.Subscription.ExpiresAt.Format("15:04:05"))
	}
	if r := t.Result; r != nil {
		fmt.Printf("resultado:   exit=%d branch=%s head=%s\n", r.ExitCode, r.Branch, r.HeadCommit)
		if r.Error != "" {
			fmt.Printf("erro:        %s\n", r.Error)
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
	c.String("title", "", "título do ticket")
	c.String("from", "", "arquivo rd.md / ticket com escopo e aceite")
	c.String("description", "", "descrição")
	c.StringSlice("scope", nil, "paths permitidos (repetível)")
	c.StringSlice("forbid", nil, "paths proibidos (repetível)")
	c.StringArray("accept", nil, "critério de aceite (repetível)")
	c.StringSlice("dep", nil, "ID do ticket do qual depende (repetível)")
	c.Int("priority", 0, "prioridade (maior primeiro)")
	c.String("timeout", "", "timeout de execução (ex.: 10m)")
	c.Bool("dry-run", false, "valida sem gravar")

	ticketListCmd.Flags().String("status", "", "filtra por status (lista separada por vírgula)")
	ticketListCmd.Flags().String("deps", "", "só tickets que dependem deste ID")

	ticketCmd.AddCommand(ticketCreateCmd, ticketListCmd, ticketShowCmd, ticketCancelCmd, ticketRetryCmd)
}
