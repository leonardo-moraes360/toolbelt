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
	egmBootstrapFrom   []string
	egmBootstrapEngine string
)

var egmBootstrapCmd = &cobra.Command{
	Use:   "bootstrap <new-doc> --from <path> [--from <path>...]",
	Short: "Writes the skeleton of a new spec or ADR wired to existing code",
	Long: `Creates a new gated document (it never overwrites) with the title, a
"generated" status line, every section its standard requires (from
egm.doc_types for the path) with TODO, and a Sources section linking each
--from file. Directories expand to their tracked files; untracked, denied,
symlinked or non-regular files are skipped with the reason. The skeleton
is refused unless the review would include exactly the accepted files as
sources. Nothing is staged or committed.

--engine deterministic (the default) runs no agent and puts no file content
in the draft (it is read only for that R7 check). --engine llm runs one
isolated goldfish (egm.agent_cmd) over the accepted files, redacted, and
puts its Markdown body in place of the TODO sections; the body is cleaned
(no HTML comments, title, Status line or Sources section), scanned for
injection and checked to cite only the accepted files, or nothing is
written. Spec: docs/specs/egm-drift-bootstrap.md (B2, B3).`,
	Example: `  tb egm bootstrap docs/specs/router.md --from internal/router/
  tb egm bootstrap docs/adr/020-x.md --from internal/x/x.go --from cmd/tb/x.go
  tb egm bootstrap docs/specs/router.md --from internal/router/ --engine llm`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		res, err := egm.Bootstrap(ctx, &egm.BootstrapOptions{Cwd: cwd, Target: args[0], From: egmBootstrapFrom, ConfigFile: cfgFile, Engine: egmBootstrapEngine})
		stop()
		if err != nil {
			return egmFail(err)
		}
		if jsonOutput {
			b, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(b))
			return nil
		}
		fmt.Printf("wrote %s (%s, %d sources, engine %s)\n", res.Doc, res.Type, len(res.Sources), res.Engine)
		for _, s := range res.Skipped {
			fmt.Printf("  skipped %s (%s)\n", s.Path, s.Reason)
		}
		fmt.Println("next: " + res.Next)
		return nil
	},
}

func init() {
	egmBootstrapCmd.Flags().StringArrayVar(&egmBootstrapFrom, "from", nil, "file or directory the document describes (repeatable)")
	egmBootstrapCmd.Flags().StringVar(&egmBootstrapEngine, "engine", egm.EngineDeterministic, "deterministic | llm (one goldfish drafts the body)")
	egmCmd.AddCommand(egmBootstrapCmd)
}
