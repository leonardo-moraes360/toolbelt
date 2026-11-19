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
	Short: "Execution with Landlock isolation (Linux)",
}

var sandboxRunCmd = &cobra.Command{
	Use:   "run <command> [args...]",
	Short: "Runs a process in the sandbox",
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
	Short: "Checks Landlock support",
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
			fmt.Printf("Error: %v\n", errMsg)
		}
		return nil
	},
}

func init() {
	sandboxRunCmd.Flags().String("workdir", "", "working directory")
	sandboxRunCmd.Flags().String("timeout", "30s", "timeout")
	sandboxRunCmd.Flags().Bool("allow-unsandboxed", false, "explicit opt-in to run without Landlock isolation")
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
