package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/secplay"
)

var blueCmd = &cobra.Command{
	Use:   "blue",
	Short: "Defensive AppSec playbooks and runnable checks",
	Long: `tb blue serves native defensive AppSec methodology — detection signals,
hardening, and incident response — paired to the red offensive playbooks. Blue
playbooks declare grep/regex checks that 'tb blue apply' registers into
.toolbelt/secchecks.yaml and 'tb blue check' runs over the working tree.`,
}

// newBlueApplyCmd builds `tb blue apply <class>`.
func newBlueApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply <class>",
		Short: "Registers a blue playbook's checks into .toolbelt/secchecks.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := loadCorpus()
			if err != nil {
				return err
			}
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			res, err := c.Apply(secToolbeltDir(), args[0], dryRun)
			if err != nil {
				return err
			}
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(res)
			}
			verb := "registered"
			if res.DryRun {
				verb = "would register"
			}
			fmt.Printf("%s %d check(s), updated %d, for class %s\n", verb, len(res.Added), len(res.Updated), res.Class)
			for _, ch := range res.Added {
				fmt.Printf("  + %s (%s)\n", ch.ID, ch.Severity)
			}
			if !res.DryRun && (len(res.Added) > 0 || len(res.Updated) > 0) {
				fmt.Printf("wrote %s\n", res.Path)
			}
			return nil
		},
	}
	cmd.Flags().Bool("dry-run", false, "show what would be registered; write nothing")
	return cmd
}

// newBlueCheckCmd builds `tb blue check`.
func newBlueCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Runs the registered checks over the working tree",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, _ := os.Getwd()
			tb := secToolbeltDir()
			vs, err := secplay.Run(cmd.Context(), cwd, tb)
			if err != nil {
				return err
			}
			if jsonOutput {
				if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"violations": vs}); err != nil {
					return err
				}
			} else {
				for _, v := range vs {
					fmt.Printf("%s:%d: %s: %s [%s]\n", v.File, v.Line, v.Severity, v.CheckID, v.Match)
				}
				if len(vs) == 0 {
					fmt.Println("no findings")
				}
			}
			if secplay.HasError(vs) {
				os.Exit(1)
			}
			return nil
		},
	}
}

func init() {
	blueCmd.AddCommand(
		newSecListCmd(secplay.Blue),
		newSecSearchCmd(secplay.Blue),
		newSecLoadCmd(secplay.Blue),
		newSecPairCmd(secplay.Blue),
		newBlueApplyCmd(),
		newBlueCheckCmd(),
	)
	rootCmd.AddCommand(blueCmd)
}
