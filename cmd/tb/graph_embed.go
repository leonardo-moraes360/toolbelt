package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/embed"
	"github.com/leonardo-moraes360/toolbelt/internal/embed/native"
)

var (
	graphEmbedSmoke   string
	graphEmbedSHA     string
	graphEmbedThreads int
)

// graphEmbedCmd is the native embedding child (ADR-018). In V0 it only
// smoke-tests a model; the JSON-lines protocol arrives in V1.
var graphEmbedCmd = &cobra.Command{
	Use:    "__graph-embed",
	Short:  "Native embedding child (internal use)",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !native.Available {
			fmt.Fprintln(os.Stderr, "tb __graph-embed:", native.ErrUnavailable)
			os.Exit(2)
		}
		if graphEmbedSmoke == "" {
			return errors.New("embed_protocol: only --smoke is available in this release")
		}
		res, err := embed.Smoke(graphEmbedSmoke, graphEmbedSHA, graphEmbedThreads)
		if err != nil {
			fmt.Fprintln(os.Stderr, "tb __graph-embed:", err)
			os.Exit(2)
		}
		b, _ := json.Marshal(res)
		fmt.Println(string(b))
		return nil
	},
}

func init() {
	f := graphEmbedCmd.Flags()
	f.StringVar(&graphEmbedSmoke, "smoke", "", "model to smoke-test (absolute path)")
	f.StringVar(&graphEmbedSHA, "sha256", "", "expected sha256 of the model")
	f.IntVar(&graphEmbedThreads, "threads", 2, "inference threads")
	rootCmd.AddCommand(graphEmbedCmd)
}
