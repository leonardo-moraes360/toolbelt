package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
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
		fmt.Fprintf(w, "  %-9s %s  preset: %s  wake: %s%s\n", cl.Name, printable(cl.Path), preset, cl.Wake, warn)
	}
	if len(c.SkippedPathEntries) > 0 {
		fmt.Fprintf(w, "  skipped relative PATH entries: %s\n", printable(strings.Join(c.SkippedPathEntries, ", ")))
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

// printable escapes control and non-printable characters: paths and scope
// keys come from config files, possibly a cloned repository's, and must not
// reach the terminal as escape sequences.
func printable(s string) string {
	q := strconv.Quote(s)
	if q[1:len(q)-1] == s {
		return s
	}
	return q
}

func mcpLine(m *busdiscover.MCPConfig) string {
	head := fmt.Sprintf("%-14s %s", m.Client, printable(m.Path))
	if m.Scope != "" && m.Scope != "user" {
		head += " [" + printable(m.Scope) + "]"
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
		s := "tb mcp: " + printable(e.Path)
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
	noun := "other servers"
	if *m.OtherServers == 1 {
		noun = "other server"
	}
	return fmt.Sprintf("%s  %s; %d %s", head, strings.Join(parts, "; "), *m.OtherServers, noun)
}

func init() {
	busDiscoverCmd.Flags().String("cwd", "", "read this directory's .mcp.json instead of the repository root's")
	busCmd.AddCommand(busDiscoverCmd)
}
