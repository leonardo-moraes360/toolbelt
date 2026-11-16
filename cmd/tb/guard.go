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
	Use:   "check <base>..<head>",
	Short: "Analisa diff entre commits",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parts := strings.SplitN(args[0], "..", 2)
		if len(parts) != 2 {
			return fmt.Errorf("use base..head")
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		res, err := guard.NewChecker(cwd).Check(parts[0], parts[1])
		if err != nil {
			return err
		}
		if jsonOutput {
			if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
				return err
			}
		} else {
			for _, v := range res.Violations {
				fmt.Printf("%s %s %s\n", v.Severity, v.Type, v.File)
			}
		}
		strict, _ := cmd.Flags().GetBool("strict")
		if strict && hasStrictError(res.Violations) {
			os.Exit(1)
		}
		return nil
	},
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
	guardCheckCmd.Flags().Bool("strict", false, "falha em erros")
	guardCmd.AddCommand(guardCheckCmd)
}
