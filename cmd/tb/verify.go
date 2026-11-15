package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/verify"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verificação adversarial com ruler de testes",
}

var verifyRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Executa loop de verificação",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		cfg := verify.DefaultConfig(cwd)
		if v, _ := cmd.Flags().GetString("base"); v != "" {
			cfg.BaseRef = v
		}
		if v, _ := cmd.Flags().GetString("head"); v != "" {
			cfg.HeadRef = v
		}
		if v, _ := cmd.Flags().GetString("test-cmd"); v != "" {
			cfg.VisibleTest = v
		}
		if v, _ := cmd.Flags().GetInt("max-iter"); v > 0 {
			cfg.MaxIterations = v
		}
		timeout, _ := time.ParseDuration(mustStringFlag(cmd, "timeout", "10m"))
		cfg.Timeout = timeout
		ctx := context.Background()
		res, err := verify.Run(ctx, &cfg)
		if err != nil {
			return err
		}
		outPath, _ := cmd.Flags().GetString("output")
		if outPath != "" {
			data, err := res.JSON()
			if err != nil {
				return err
			}
			return os.WriteFile(outPath, data, 0o600)
		}
		if jsonOutput {
			data, _ := res.JSON()
			fmt.Println(string(data))
		} else {
			fmt.Printf("Veredicto: %s (iterações: %d)\n", res.Verdict, res.Iterations)
			fmt.Printf("Visível: %v  Oculto: %v (disponível: %v)\n", res.VisiblePassed, res.HiddenPassed, res.HiddenAvailable)
		}
		if res.Verdict != verify.VerdictPass {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	verifyRunCmd.Flags().String("base", "", "ref base")
	verifyRunCmd.Flags().String("head", "", "ref head")
	verifyRunCmd.Flags().String("test-cmd", "", "linha de testes visíveis")
	verifyRunCmd.Flags().Int("max-iter", 0, "máximo de iterações")
	verifyRunCmd.Flags().String("timeout", "10m", "timeout")
	verifyRunCmd.Flags().StringP("output", "o", "", "arquivo JSON")
	verifyCmd.AddCommand(verifyRunCmd)
}
