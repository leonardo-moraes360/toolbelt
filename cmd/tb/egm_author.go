package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/egm"
)

var (
	egmBrainstormBrief string
	egmBrainstormFrom  []string
	egmBrainstormOut   string

	egmDraftBrief  string
	egmDraftFrom   []string
	egmDraftBody   string
	egmDraftEngine string
	egmDraftUpdate bool
)

var egmBrainstormCmd = &cobra.Command{
	Use:   "brainstorm --brief <file> [--from <path>...] [--out <file>]",
	Short: "Turns a brief into a concept brief (ideas, trade-offs, a recommendation); not gated",
	Long: `Runs one isolated goldfish (egm.agent_cmd) over the redacted brief and the
optional --from files, and writes a concept brief with the sections Problem,
Ideas, Recommended direction and Open questions. A brief with a secret-like
value or text that reads as an instruction is refused before any agent runs.

The result is never reviewed or gated. By default it lands outside the
repository ($XDG_STATE_HOME/tb/egm/<repo-id>/briefs/); --out names a new file
inside it, refused when it matches egm.doc_globs. Spec: docs/specs/egm-phase-c.md (C1).`,
	Example: `  tb egm brainstorm --brief rd.md
  tb egm brainstorm --brief rd.md --from internal/router/ --out notes/router-ideas.md`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		res, err := egm.Brainstorm(ctx, &egm.BrainstormOptions{Cwd: cwd, Brief: egmBrainstormBrief, From: egmBrainstormFrom, Out: egmBrainstormOut, ConfigFile: cfgFile})
		stop()
		if err != nil {
			return egmFail(err)
		}
		if jsonOutput {
			b, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(b))
			return nil
		}
		fmt.Printf("wrote %s\n", res.Out)
		for _, s := range res.Skipped {
			fmt.Printf("  skipped %s (%s)\n", s.Path, s.Reason)
		}
		fmt.Println("next: " + res.Next)
		return nil
	},
}

var egmDraftCmd = &cobra.Command{
	Use:   "draft <doc> (--body <file> | --engine llm) [--brief <file>] [--from <path>...] [--update]",
	Short: "Writes the first version of a spec or ADR from a brief; it still needs the review loop",
	Long: `Creates a new gated document (it never overwrites) with the title, a
"generated" status line, the body, every section its standard requires (TODO
when the body omits one) and a Sources section linking each --from file.

--body <file> is deterministic: the elephant writes the body, tb assembles and
checks it, no agent runs. --engine llm runs one isolated goldfish
(egm.agent_cmd) over the standard, the redacted --brief and the --from files.
Give exactly one. The body is cleaned, scanned for injection and may cite only
the accepted --from files, or nothing is written. The brief is not cited.

The draft gets no review state, round or receipt: it reaches PASS only through
tb egm review and loop next. With --update the document is never edited: tb
writes a full proposal under the state directory and prints a diff.
Spec: docs/specs/egm-phase-c.md (C2, C3).`,
	Example: `  tb egm draft docs/specs/router.md --brief rd.md --from internal/router/ --body /tmp/body.md
  tb egm draft docs/specs/router.md --brief rd.md --from internal/router/ --engine llm
  tb egm draft docs/specs/router.md --update --brief rd.md --engine llm`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		res, err := egm.Draft(ctx, &egm.DraftOptions{Cwd: cwd, Target: args[0], Brief: egmDraftBrief, From: egmDraftFrom,
			BodyFile: egmDraftBody, Engine: egmDraftEngine, Update: egmDraftUpdate, ConfigFile: cfgFile})
		stop()
		if err != nil {
			return egmFail(err)
		}
		if jsonOutput {
			b, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(b))
			return nil
		}
		if res.Proposal != "" {
			fmt.Printf("wrote proposal %s (engine %s)\n%s", res.Proposal, res.Engine, res.Diff)
		} else {
			fmt.Printf("wrote %s (%d sources, engine %s)\n", res.Doc, len(res.Sources), res.Engine)
		}
		for _, s := range res.Skipped {
			fmt.Printf("  skipped %s (%s)\n", s.Path, s.Reason)
		}
		fmt.Println("next: " + res.Next)
		return nil
	},
}

func init() {
	f := egmBrainstormCmd.Flags()
	f.StringVar(&egmBrainstormBrief, "brief", "", "brief file inside the repository (required)")
	f.StringArrayVar(&egmBrainstormFrom, "from", nil, "code context: file or directory (repeatable)")
	f.StringVar(&egmBrainstormOut, "out", "", "new file inside the repository instead of the state directory (not a gated path)")
	g := egmDraftCmd.Flags()
	g.StringVar(&egmDraftBrief, "brief", "", "brief file (inside the repository or a stored concept brief); not cited")
	g.StringArrayVar(&egmDraftFrom, "from", nil, "file or directory the document describes (repeatable)")
	g.StringVar(&egmDraftBody, "body", "", "file with the body, written by the elephant (deterministic)")
	g.StringVar(&egmDraftEngine, "engine", "", "llm: one goldfish drafts the body")
	g.BoolVar(&egmDraftUpdate, "update", false, "write a proposal for an existing document instead of a new one")
	egmCmd.AddCommand(egmBrainstormCmd, egmDraftCmd)
}
