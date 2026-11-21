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

var egmDriftAt string

var egmDriftCmd = &cobra.Command{
	Use:   "drift-report [doc...]",
	Short: "Compares gated documents with the sources their receipt certified",
	Long: `Compares each gated document (or the given ones) with the sources its
baseline certified: the committed receipt for these document bytes, or a
converged local round. Drift is decided by the hashes of the baseline's
source paths at --at (default HEAD), read from blobs; no agent runs.

Exit 0: every document fresh or without a baseline; 1: some document
drifted, lost a source or cites a new one; 2: unreadable or usage error.
Spec: docs/specs/egm-drift-bootstrap.md.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		rep, err := egm.Drift(ctx, &egm.DriftOptions{Cwd: cwd, Docs: args, At: egmDriftAt, ConfigFile: cfgFile})
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
	}
	if rep.ExitCode() == egm.ExitFail {
		fmt.Fprintln(w, "\nUpdate the drifted documents, or re-review them: tb egm review <doc>.")
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
	egmCmd.AddCommand(egmDriftCmd)
}
