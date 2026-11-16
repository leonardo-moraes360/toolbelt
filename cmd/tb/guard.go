package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/guard"
)

var guardCmd = &cobra.Command{
	Use:   "guard",
	Short: "Proteções anti-trapaça",
}

var guardCheckCmd = &cobra.Command{
	Use:   "check [base..head]",
	Short: "Analisa diff entre commits",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		base, head, err := resolveGuardRange(cmd, args)
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		res, err := guard.NewChecker(cwd).Check(base, head)
		if err != nil {
			return err
		}
		if jsonOutput {
			if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
				return err
			}
		} else {
			printGuardText(res)
		}
		warnOnly, _ := cmd.Flags().GetBool("warn-only")
		if hasStrictError(res.Violations) && !warnOnly {
			os.Exit(1)
		}
		return nil
	},
}

func printGuardText(res *guard.CheckResult) {
	for _, v := range res.Violations {
		fmt.Printf("%s %s %s\n", v.Severity, v.Type, v.File)
	}
	if len(res.Violations) == 0 {
		fmt.Printf("guard: ok (%d regras, 0 violações)\n", res.RulesChecked)
	}
}

func resolveGuardRange(cmd *cobra.Command, args []string) (base, head string, err error) {
	base, _ = cmd.Flags().GetString("base")
	head, _ = cmd.Flags().GetString("head")
	if len(args) == 1 {
		parts := strings.SplitN(args[0], "..", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("use base..head ou --base/--head")
		}
		return parts[0], parts[1], nil
	}
	if base == "" || head == "" {
		return "", "", fmt.Errorf("use base..head ou --base/--head")
	}
	return base, head, nil
}

func hasStrictError(violations []guard.Violation) bool {
	for _, v := range violations {
		if v.Severity == guard.SeverityError {
			return true
		}
	}
	return false
}

func init() {
	guardCheckCmd.Flags().Bool("warn-only", false, "não falha o exit code em SeverityError (opt-out do default CI-safe)")
	guardCheckCmd.Flags().Bool("strict", false, "falha em SeverityError (já é o default)")
	_ = guardCheckCmd.Flags().MarkDeprecated("strict", "falha em SeverityError já é o default; use --warn-only para opt-out")
	guardCheckCmd.Flags().String("base", "HEAD~1", "ref base")
	guardCheckCmd.Flags().String("head", "HEAD", "ref head")
	guardCmd.AddCommand(guardCheckCmd)
}
