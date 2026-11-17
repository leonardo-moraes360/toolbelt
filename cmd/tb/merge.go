package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/merge"
)

func init() {
	rootCmd.AddCommand(mergeCmd)
	mergeCmd.Flags().String("strategy", "fail", "Estratégia de conflito: fail (padrão) ou last_wins")
	mergeCmd.Flags().Bool("dry-run", false, "Integra num worktree temporário, reporta e descarta")
	mergeCmd.Flags().String("output", "", "Escreve o diff alvo..integrado neste arquivo .patch sem mover refs")
	mergeCmd.Flags().String("into", "", "Branch alvo (padrão: branch atual)")
	mergeCmd.Flags().Bool("force", false, "Permite refazer o merge de tickets já mergeados")
	mergeCmd.Flags().Bool("allow-sensitive", false, "Permite aplicar tickets que alteram hooks, .toolbelt, workflows, .gitmodules, gitlinks ou .gitattributes")
	mergeCmd.Flags().Bool("json", false, "Saída em JSON")
}

var mergeCmd = &cobra.Command{
	Use:   "merge <ticket-id...>",
	Short: "Integra o trabalho de tickets done num branch alvo (fail|last_wins)",
	Long: `Integra os commits dos tickets done (Result.BaseCommit..HeadCommit) num
worktree temporário destacado e só então avança o branch alvo de forma atômica.

Ordem: dependências primeiro, depois a ordem dos argumentos.
Estratégias: fail (padrão; aborta sem alterar refs) e last_wins (a versão do
último ticket vence nos arquivos em conflito). prompt/semantic não são
implementadas.

Todo git roda sem hooks/fsmonitor/diff externo. Tickets que alteram caminhos
sensíveis (hooks, .toolbelt/**, .github/workflows/**, .gitmodules, gitlinks,
.gitattributes, paths protegidos do guard) são recusados ao aplicar, sem
alterar refs, a menos que use --allow-sensitive (dry-run apenas reporta).`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		stratS, _ := f.GetString("strategy")
		dry, _ := f.GetBool("dry-run")
		output, _ := f.GetString("output")
		into, _ := f.GetString("into")
		force, _ := f.GetBool("force")
		allowSens, _ := f.GetBool("allow-sensitive")
		asJSON, _ := f.GetBool("json")

		strategy, err := merge.ParseStrategy(stratS)
		if err != nil {
			return err
		}
		store, err := openTicketStore()
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		res, err := merge.Run(merge.Options{
			Repo: cwd, Store: store, IDs: args, Strategy: strategy,
			DryRun: dry, Output: output, Into: into, Force: force, AllowSensitive: allowSens,
		})
		if res != nil {
			if asJSON {
				if jerr := printJSON(res); jerr != nil {
					return jerr
				}
			} else {
				printMergeTable(res)
			}
		}
		if err != nil && (errors.Is(err, merge.ErrConflict) || errors.Is(err, merge.ErrSensitive)) {
			cmd.SilenceUsage = true
		}
		return err
	},
}

func printMergeTable(r *merge.Result) {
	mode := "aplicado"
	switch {
	case r.Output != "":
		mode = "patch em " + r.Output
	case r.DryRun:
		mode = "dry-run"
	case !r.Applied:
		mode = "não aplicado"
	}
	target := r.Target
	if target == "" {
		target = "(HEAD destacado)"
	}
	fmt.Printf("merge %s -> %s (%s) [%s]\n", ticketIDs(r), target, r.Strategy, mode)
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ARQUIVO\tTICKETS\tSTATUS")
	for _, f := range r.Files {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", f.Path, strings.Join(f.Tickets, ","), f.Status)
	}
	tw.Flush()
	for _, c := range r.Conflicts {
		res := "não resolvido"
		if c.Resolution != "" {
			res = "resolvido por " + c.Resolution
		}
		fmt.Printf("conflito: %s (%s, commit %.8s; também: %s) - %s\n", c.File, c.Ticket, c.Commit, strings.Join(c.Others, ","), res)
	}
	for _, s := range r.Sensitive {
		fmt.Printf("sensível: %s (%s) - %s\n", s.Path, s.Ticket, s.Reason)
	}
	if r.ResultSHA != "" {
		fmt.Printf("resultado: %s (início %.8s)\n", r.ResultSHA, r.StartSHA)
	}
	if r.NoChanges {
		fmt.Println("nada a integrar (sem mudanças)")
	}
}

func ticketIDs(r *merge.Result) string {
	ids := make([]string, 0, len(r.Tickets))
	for _, t := range r.Tickets {
		ids = append(ids, t.ID)
	}
	return strings.Join(ids, " ")
}
