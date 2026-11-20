package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/busdiscover"
)

var busDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Read-only catalog of the agent CLIs and MCP client configs on this machine",
	Long: `Lists the known agent CLIs on PATH and the MCP client configs that run tb mcp,
with next steps to copy. It creates no invite, joins nothing, writes nothing,
opens no connection and executes no binary; other MCP servers are only
counted. Spec: docs/bus-notify-discover.md.`,
	Args: cobra.NoArgs,
	RunE: runBusDiscover,
}

func runBusDiscover(cmd *cobra.Command, _ []string) error {
	cwdFlag, _ := cmd.Flags().GetString("cwd")
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	cat, err := busdiscover.Discover(&busdiscover.Options{
		PathEnv: os.Getenv("PATH"), Home: home, Cwd: cwdFlag, WorkDir: wd, Self: self, GOOS: runtime.GOOS,
	})
	if err != nil {
		return err
	}
	if jsonOutput {
		return printBusJSON(cat)
	}
	printDiscover(cmd.OutOrStdout(), cat)
	return nil
}

func printDiscover(w io.Writer, c *busdiscover.Catalog) {
	fmt.Fprintf(w, "tb: %s\n\nCLIs\n", c.TB)
	if len(c.CLIs) == 0 {
		fmt.Fprintln(w, "  (none of the known agent CLIs is on PATH)")
	}
	for i := range c.CLIs {
		cl := &c.CLIs[i]
		preset := cl.Preset
		if preset == "" {
			preset = "none"
		}
		warn := ""
		if cl.WritableByOthers {
			warn = "  (writable by others)"
		}
		fmt.Fprintf(w, "  %-9s %s  preset: %s  wake: %s%s\n", cl.Name, cl.Path, preset, cl.Wake, warn)
	}
	if len(c.SkippedPathEntries) > 0 {
		fmt.Fprintf(w, "  skipped relative PATH entries: %s\n", strings.Join(c.SkippedPathEntries, ", "))
	}
	fmt.Fprintln(w, "\nMCP configs")
	for i := range c.MCPConfigs {
		fmt.Fprintf(w, "  %s\n", mcpLine(&c.MCPConfigs[i]))
	}
	if c.Project == nil {
		fmt.Fprintln(w, "  project: none (not in a git repository; use --cwd)")
	}
	fmt.Fprint(w, `
next (CLI agents):  tb bus invite <channel> --role worker --name <name>
                    tb bus run --channel <channel> --as <name> --invite <code>   (write the wake file first, see docs/bus-harness.md)
next (GUI agents):  tb bus join <channel> --invite <code> --name <name> --wake notify
                    point the client at: tb mcp --bus-as <channel>/<name> --bus-root <root>   (the agent must not call tb_bus_join)
`)
}

func mcpLine(m *busdiscover.MCPConfig) string {
	head := fmt.Sprintf("%-14s %s", m.Client, m.Path)
	if m.Scope != "" && m.Scope != "user" {
		head += " [" + m.Scope + "]"
	}
	switch {
	case m.Kind != busdiscover.KindFile:
		return head + "  " + m.Kind
	case m.Parsed != nil:
		return head + "  exists (not parsed)"
	case m.Parses != nil && !*m.Parses:
		return head + "  does not parse"
	}
	parts := []string{}
	for _, e := range *m.TB {
		s := "tb mcp: " + e.Path
		switch {
		case e.SameTB == nil:
			s += " (resolved on the client's PATH)"
		case *e.SameTB:
			s += " (same tb)"
		default:
			s += " (another tb)"
		}
		if e.BusAs != nil {
			s += " --bus-as " + *e.BusAs
		}
		parts = append(parts, s)
	}
	if len(parts) == 0 {
		parts = append(parts, "tb mcp: not configured")
	}
	return fmt.Sprintf("%s  %s; %d other servers", head, strings.Join(parts, "; "), *m.OtherServers)
}

func init() {
	busDiscoverCmd.Flags().String("cwd", "", "read this directory's .mcp.json instead of the repository root's")
	busCmd.AddCommand(busDiscoverCmd)
}
