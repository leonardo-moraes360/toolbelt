package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/envelope"
	"github.com/leonardo-moraes360/toolbelt/internal/merge"
)

func init() {
	rootCmd.AddCommand(mergeCmd)
	mergeCmd.Flags().String("strategy", "fail", "Conflict strategy: fail (default), last_wins, semantic or prompt (experimental)")
	mergeCmd.Flags().Bool("dry-run", false, "Integrates in a temporary worktree, reports and discards")
	mergeCmd.Flags().String("output", "", "Writes the target..integrated diff to this .patch file without moving refs")
	mergeCmd.Flags().String("into", "", "Target branch (default: current branch)")
	mergeCmd.Flags().Bool("force", false, "Allows redoing the merge of already merged tickets")
	mergeCmd.Flags().Bool("allow-sensitive", false, "Allows applying tickets that change hooks, .toolbelt, workflows, .gitmodules, gitlinks or .gitattributes")
	mergeCmd.Flags().String("resolutions", "", "Replays a prompt receipt (ID printed by the proposal) instead of calling the agent; prompt only (experimental)")
	mergeCmd.Flags().Bool("json", false, "JSON output")
}

var mergeCmd = &cobra.Command{
	Use:   "merge <ticket-id...>",
	Short: "Integrates the work of done tickets into a target branch (fail|last_wins|semantic|prompt)",
	Long: `Integrates the commits of done tickets (Result.BaseCommit..HeadCommit) in a
detached temporary worktree and only then advances the target branch atomically.

Order: dependencies first, then the order of the arguments.
Strategies:
  fail       (default) aborts on the first conflict, without changing refs.
  last_wins  the last ticket's whole file wins on conflicting files.
  semantic   resolves git's remaining conflict hunks with deterministic rules
             (disjoint edits; Go import blocks) and reports every resolved
             hunk. Binary, symlink, gitlink, delete/modify, add/add, mode and
             sensitive-path conflicts are never resolved. If any hunk stays
             unresolved it behaves like fail. Reads merge.semantic.* from the
             user config only.
  prompt     runs semantic, then a sandboxed one-shot agent (egm.agent_cmd,
             user config only) on the hunks left. Two steps: the proposal
             writes a receipt and never moves a ref; review it, then replay it
             with --resolutions <id> (no agent is started).

Every git call runs without hooks/fsmonitor/external diff. Tickets that change
sensitive paths (hooks, .toolbelt/**, .github/workflows/**, .gitmodules, gitlinks,
.gitattributes, guard-protected paths) are refused when applying, without
changing refs, unless --allow-sensitive is used (dry-run only reports).`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		stratS, _ := f.GetString("strategy")
		dry, _ := f.GetBool("dry-run")
		output, _ := f.GetString("output")
		into, _ := f.GetString("into")
		force, _ := f.GetBool("force")
		allowSens, _ := f.GetBool("allow-sensitive")
		resolutions, _ := f.GetString("resolutions")
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
		res, err := merge.Run(&merge.Options{
			Repo: cwd, Store: store, IDs: args, Strategy: strategy,
			DryRun: dry, Output: output, Into: into, Force: force, AllowSensitive: allowSens,
			Resolutions: resolutions, Config: cfgFile,
		})
		if res != nil {
			if asJSON {
				if jerr := emitJSONIndent(envelope.Merge, res); jerr != nil {
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
	mode := "applied"
	switch {
	case r.Output != "":
		mode = "patch at " + r.Output
	case r.DryRun:
		mode = "dry-run"
	case !r.Applied:
		mode = "not applied"
	}
	target := r.Target
	if target == "" {
		target = "(detached HEAD)"
	}
	fmt.Printf("merge %s -> %s (%s) [%s]\n", ticketIDs(r), target, r.Strategy, mode)
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "FILE\tTICKETS\tSTATUS")
	for _, f := range r.Files {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", f.Path, strings.Join(f.Tickets, ","), f.Status)
	}
	tw.Flush()
	for i := range r.Conflicts {
		c := &r.Conflicts[i]
		fmt.Printf("conflict: %s (%s, commit %.8s; also: %s) - %s\n", c.File, c.Ticket, c.Commit, strings.Join(c.Others, ","), conflictStatus(c))
		for _, h := range c.Hunks {
			if h.Reason != "" {
				fmt.Printf("  region %d: ours %d-%d, theirs %d-%d: %s\n", h.Region, h.OursLines[0], h.OursLines[1], h.TheirsLines[0], h.TheirsLines[1], h.Reason)
			}
		}
	}
	for _, w := range r.Warnings {
		fmt.Printf("warning: %s: %s\n", w.Code, w.Message)
	}
	if r.Receipt != nil {
		fmt.Printf("receipt: %s (%s)\n", r.Receipt.ID, r.Receipt.Path)
	}
	for _, s := range r.Sensitive {
		fmt.Printf("sensitive: %s (%s) - %s\n", s.Path, s.Ticket, s.Reason)
	}
	if r.ResultSHA != "" {
		fmt.Printf("result: %s (start %.8s)\n", r.ResultSHA, r.StartSHA)
	}
	if r.ResultTree != "" {
		fmt.Printf("result tree: %s\n", r.ResultTree)
	}
	if r.NoChanges {
		fmt.Println("nothing to integrate (no changes)")
	}
}

func ticketIDs(r *merge.Result) string {
	ids := make([]string, 0, len(r.Tickets))
	for _, t := range r.Tickets {
		ids = append(ids, t.ID)
	}
	return strings.Join(ids, " ")
}

// conflictStatus renders the resolution column of one conflict: the rules that
// resolved its hunks, or the reason it stayed unresolved. Without hunks (fail,
// last_wins) it is the plain text of earlier releases.
func conflictStatus(c *merge.Conflict) string {
	if c.Resolution == "" {
		if c.Reason == "" {
			return "unresolved"
		}
		return "unresolved (" + c.Reason + ")"
	}
	out := "resolved by " + c.Resolution
	var rules []string
	novel := 0
	for _, h := range c.Hunks {
		if h.Rule != "" && !slices.Contains(rules, h.Rule) {
			rules = append(rules, h.Rule)
		}
		novel += h.NovelLines
	}
	if len(rules) > 0 {
		out += " (" + strings.Join(rules, ", ")
		if novel > 0 {
			out += fmt.Sprintf("; %d novel line(s)", novel)
		}
		out += ")"
	}
	return out
}
