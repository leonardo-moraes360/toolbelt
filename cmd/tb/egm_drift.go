package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/egm"
)

var (
	egmDriftAt     string
	egmDriftEngine string
)

var egmDriftCmd = &cobra.Command{
	Use:   "drift-report [doc...]",
	Short: "Compares gated documents with the sources their receipt certified",
	Long: `Compares each gated document (or the given ones) with the sources its
baseline certified: the committed receipt for these document bytes, or a
converged local round. Drift is decided by the hashes of the baseline's
source paths at --at (default HEAD), read from blobs.

--engine deterministic (the default) runs no agent. --engine llm also runs
one isolated drift goldfish per drifted document (up to egm.drift.max_docs)
over the document, each changed source and, when the receipt's advisory
commit is usable, its diff; its findings are reported, never stored, and do
not change the exit code.

Exit 0: every document fresh or without a baseline; 1: some document
drifted, lost a source or cites a new one; 2: unreadable or usage error.
Spec: docs/specs/egm-drift-bootstrap.md.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		rep, err := egm.Drift(ctx, &egm.DriftOptions{Cwd: cwd, Docs: args, At: egmDriftAt, ConfigFile: cfgFile, Engine: egmDriftEngine})
		stop()
		if err != nil {
			return egmFail(err)
		}
		if jsonOutput {
			b, _ := json.MarshalIndent(rep, "", "  ")
			fmt.Println(string(b))
		} else {
			printDrift(os.Stdout, rep)
		}
		if code := rep.ExitCode(); code != 0 {
			os.Exit(code)
		}
		return nil
	},
}

func printDrift(w io.Writer, rep *egm.DriftReport) {
	fmt.Fprintf(w, "Source drift at %s\n", shortSHA(rep.At))
	if len(rep.Documents) == 0 {
		fmt.Fprintln(w, "  (no gated documents)")
	}
	for i := range rep.Documents {
		d := &rep.Documents[i]
		line := fmt.Sprintf("  %-15s %s", d.Status, d.Doc)
		if d.Baseline != egm.BaselineNone {
			line += "  (baseline: " + d.Baseline + ")"
		}
		fmt.Fprintln(w, line)
		for _, p := range d.Changed {
			fmt.Fprintln(w, "      changed: "+p)
		}
		for _, m := range d.Missing {
			fmt.Fprintf(w, "      missing: %s (%s)\n", m.Path, m.Reason)
		}
		for _, p := range d.Added {
			fmt.Fprintln(w, "      added:   "+p)
		}
		if d.Reason != "" {
			fmt.Fprintln(w, "      "+d.Reason)
		}
		printDriftLLM(w, d)
	}
	if rep.ExitCode() == egm.ExitFail {
		fmt.Fprintln(w, "\nUpdate the drifted documents, or re-review them: tb egm review <doc>.")
	}
}

func printDriftLLM(w io.Writer, d *egm.DriftDoc) {
	if d.LLM != nil {
		line := fmt.Sprintf("      drift goldfish: %s", d.LLM.Status)
		if d.LLM.Reason != "" {
			line += " (" + d.LLM.Reason + ")"
		}
		fmt.Fprintln(w, line+", "+d.LLM.Diff)
		for _, o := range d.LLM.Omitted {
			fmt.Fprintf(w, "      omitted: %s (%s)\n", o.Path, o.Reason)
		}
	}
	for i := range d.Findings {
		f := &d.Findings[i]
		fmt.Fprintf(w, "      [%s] %s: %s\n", f.Severity, f.Reviewer, f.Message)
	}
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return strings.TrimSpace(s)
}

func init() {
	egmDriftCmd.Flags().StringVar(&egmDriftAt, "at", "", "commit to report as of (default HEAD), read from blobs")
	egmDriftCmd.Flags().StringVar(&egmDriftEngine, "engine", egm.EngineDeterministic, "deterministic | llm (a drift goldfish per drifted document)")
	egmCmd.AddCommand(egmDriftCmd)
}
