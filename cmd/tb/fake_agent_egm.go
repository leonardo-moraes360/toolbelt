//go:build egmfake

package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/bench"
)

// fakeAgentEgmCmd exists only in binaries built with -tags egmfake (e2e
// tests of tb egm); release builds never contain it.
var fakeAgentEgmCmd = &cobra.Command{
	Use:                bench.FakeAgentEgmSubcommand,
	Hidden:             true,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return bench.RunFakeAgentEgm(args, os.Stdin, os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(fakeAgentEgmCmd)
}
