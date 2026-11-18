package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/bus"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
	"github.com/leonardo-moraes360/toolbelt/internal/mcp"
	"github.com/leonardo-moraes360/toolbelt/internal/registry"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Starts an MCP server over stdio",
	Long: `Starts toolbelt as an MCP (Model Context Protocol) server.

The server exposes registered tools via JSON-RPC 2.0 over stdio,
aligned with stateless MCP. The same tools as the CLI are available via tools/list and tools/call.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := bindBusAs(cmd); err != nil {
			return err
		}
		log.Info("MCP server started", "transport", "stdio")
		srv := mcp.NewServer(registry.Default())
		return srv.Run(cmd.Context(), os.Stdin, os.Stdout)
	},
}

// bindBusAs handles --bus-as <channel>/<name>: binds the connection to the
// agent's CLI session (same trust boundary as the CLI; used by tb bus run).
func bindBusAs(cmd *cobra.Command) error {
	as, _ := cmd.Flags().GetString("bus-as")
	if as == "" {
		return nil
	}
	channel, name, ok := strings.Cut(as, "/")
	if !ok || channel == "" || name == "" {
		return fmt.Errorf("invalid --bus-as %q (use <channel>/<name>)", as)
	}
	if state, _ := cmd.Flags().GetString("bus-state"); state != "" {
		if err := os.Setenv(bus.EnvStateDir, state); err != nil {
			return err
		}
	}
	root, _ := cmd.Flags().GetString("bus-root")
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root = ticket.FindRoot(cwd)
	}
	return registry.BindBusSession(root, channel, name)
}

func init() {
	mcpCmd.Flags().String("bus-as", "", "binds the connection to the bus session <channel>/<name> (used by tb bus run)")
	mcpCmd.Flags().String("bus-root", "", "bus repository root (default: searches for .toolbelt from the cwd)")
	mcpCmd.Flags().String("bus-state", "", "base of the bus private directory (default $XDG_STATE_HOME/toolbelt/bus)")
}
