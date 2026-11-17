package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/sandbox"
)

var sandboxCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "Execução com isolamento Landlock (Linux)",
}

var sandboxRunCmd = &cobra.Command{
	Use:   "run <command> [args...]",
	Short: "Executa processo no sandbox",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workdir, _ := cmd.Flags().GetString("workdir")
		if workdir == "" {
			workdir, _ = os.Getwd()
		}
		timeout, _ := time.ParseDuration(mustStringFlag(cmd, "timeout", "30s"))
		allowUnsandboxed, _ := cmd.Flags().GetBool("allow-unsandboxed")
		cfg := sandbox.Config{
			WorkDir:          workdir,
			Timeout:          timeout,
			AllowUnsandboxed: allowUnsandboxed,
		}
		sb, err := sandbox.New(cfg)
		if err != nil {
			return err
		}
		res, err := sb.Run(context.Background(), args[0], args[1:]...)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(res)
		}
		fmt.Print(res.Output)
		if res.ExitCode != 0 && !res.TimedOut {
			os.Exit(res.ExitCode)
		}
		return nil
	},
}

var sandboxCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Verifica suporte a Landlock",
	RunE: func(cmd *cobra.Command, args []string) error {
		info := map[string]any{"os": runtime.GOOS, "arch": runtime.GOARCH}
		if runtime.GOOS == "linux" {
			ok, ver, err := sandbox.CheckLandlockSupport()
			info["landlock"] = ok
			info["version"] = ver
			if err != nil {
				info["error"] = err.Error()
			}
		} else {
			info["landlock"] = false
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(info)
		}
		fmt.Printf("Landlock: %v\n", info["landlock"])
		if ver, ok := info["version"]; ok {
			fmt.Printf("ABI: %v\n", ver)
		}
		if errMsg, ok := info["error"]; ok {
			fmt.Printf("Erro: %v\n", errMsg)
		}
		return nil
	},
}

func init() {
	sandboxRunCmd.Flags().String("workdir", "", "diretório de trabalho")
	sandboxRunCmd.Flags().String("timeout", "30s", "timeout")
	sandboxRunCmd.Flags().Bool("allow-unsandboxed", false, "opt-in explícito para executar sem isolamento Landlock")
	sandboxCmd.AddCommand(sandboxRunCmd)
	sandboxCmd.AddCommand(sandboxCheckCmd)
}

func mustStringFlag(cmd *cobra.Command, name, def string) string {
	v, _ := cmd.Flags().GetString(name)
	if v == "" {
		return def
	}
	return v
}
