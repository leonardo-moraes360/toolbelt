package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/envelope"
	"github.com/leonardo-moraes360/toolbelt/internal/workspace"
)

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Aliases: []string{"ws"},
	Short:   "Isolated workspaces via git worktree",
}

var wsCreateCmd = &cobra.Command{
	Use:   "create <id>",
	Short: "Creates a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		base, _ := cmd.Flags().GetString("base")
		allow, _ := cmd.Flags().GetStringSlice("allow")
		forbid, _ := cmd.Flags().GetStringSlice("forbid")
		mgr, err := workspace.NewManager(cwd)
		if err != nil {
			return err
		}
		ws, err := mgr.Create(args[0], base, workspace.Scope{AllowedPaths: allow, ForbiddenPaths: forbid})
		if err != nil {
			return err
		}
		if jsonOutput {
			return emitJSON(envelope.Workspace, ws)
		}
		fmt.Printf("workspace %s at %s\n", ws.ID, ws.Path)
		return nil
	},
}

var wsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists workspaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		mgr, err := workspace.NewManager(cwd)
		if err != nil {
			return err
		}
		list, err := mgr.List()
		if err != nil {
			return err
		}
		if jsonOutput {
			return emitJSON(envelope.Workspace, list)
		}
		for _, w := range list {
			fmt.Printf("%s\t%s\n", w.ID, w.Path)
		}
		return nil
	},
}

var wsRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Removes a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		mgr, err := workspace.NewManager(cwd)
		if err != nil {
			return err
		}
		return mgr.Remove(args[0])
	},
}

func init() {
	wsCreateCmd.Flags().String("base", "HEAD", "base ref")
	wsCreateCmd.Flags().StringSlice("allow", nil, "allowed paths")
	wsCreateCmd.Flags().StringSlice("forbid", nil, "forbidden paths")
	workspaceCmd.AddCommand(wsCreateCmd, wsListCmd, wsRemoveCmd)
}
