package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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
		// The identity lock lives as long as the server; the kernel also
		// drops the flock on exit or crash.
		releaseBus, err := bindBusAs(cmd)
		if err != nil {
			return err
		}
		defer releaseBus()
		log.Info("MCP server started", "transport", "stdio")
		srv := mcp.NewServer(registry.Default())
		// Bus sessions bound to this connection keep their leases while it
		// is open (docs/bus-lease.md R7).
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()
		// The warm embedding child must not outlive the server (EOF or
		// cancellation both end Run).
		defer registry.Shutdown()
		// Run blocks in a stdin read that a signal does not interrupt, so
		// a signal shuts the pool down here (the child's private tmp would
		// leak otherwise) and exits.
		sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer stop()
		go func() {
			<-sigCtx.Done()
			if ctx.Err() == nil {
				registry.Shutdown()
				os.Exit(130)
			}
		}()
		go registry.RunBusHeartbeats(ctx, 5*time.Second)
		return srv.Run(ctx, os.Stdin, os.Stdout)
	},
}

// bindBusAs handles --bus-as <channel>/<name>: binds the connection to the
// agent's CLI session (same trust boundary as the CLI; used by tb bus run).
func bindBusAs(cmd *cobra.Command) (release func(), err error) {
	as, _ := cmd.Flags().GetString("bus-as")
	if as == "" {
		return func() {}, nil
	}
	channel, name, ok := strings.Cut(as, "/")
	if !ok || channel == "" || name == "" {
		return nil, fmt.Errorf("invalid --bus-as %q (use <channel>/<name>)", as)
	}
	if state, _ := cmd.Flags().GetString("bus-state"); state != "" {
		if err := os.Setenv(bus.EnvStateDir, state); err != nil {
			return nil, err
		}
	}
	root, _ := cmd.Flags().GetString("bus-root")
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		root = ticket.FindRoot(cwd)
	}
	// The harness that spawned this server holds the identity lock for its
	// whole run, and the sandbox it runs us in cannot open the lock file:
	// its pid, passed in the 0600 mcp.json it wrote, stands for the lock.
	if holder, _ := cmd.Flags().GetInt("bus-holder-pid"); holder > 0 {
		if !bus.ProcessAlive(holder) {
			return nil, fmt.Errorf("%w: the tb bus run (pid %d) that owned %s/%s is gone; start the client again through tb bus run", bus.ErrSessionInUse, holder, channel, name)
		}
	} else {
		if release, err = bus.LockIdentity(root, channel, name, "tb mcp --bus-as"); err != nil {
			return nil, err
		}
	}
	if release == nil {
		release = func() {}
	}
	if err := registry.BindBusSession(root, channel, name); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

func init() {
	mcpCmd.Flags().String("bus-as", "", "binds the connection to the bus session <channel>/<name> (used by tb bus run)")
	mcpCmd.Flags().Int("bus-holder-pid", 0, "pid of the tb bus run that holds this identity and spawned this server")
	_ = mcpCmd.Flags().MarkHidden("bus-holder-pid")
	mcpCmd.Flags().String("bus-root", "", "bus repository root (default: searches for .toolbelt from the cwd)")
	mcpCmd.Flags().String("bus-state", "", "base of the bus private directory (default $XDG_STATE_HOME/toolbelt/bus)")
}
