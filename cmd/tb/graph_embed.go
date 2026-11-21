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
	graphEmbedModel   string
	graphEmbedSHA     string
	graphEmbedThreads int
)

// graphEmbedCmd is the native embedding child (ADR-018): with --smoke it
// smoke-tests a model, otherwise it serves the JSON-lines protocol on
// stdin/stdout (docs/specs/vector-index.md, Protocol).
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
		// Before Serve and Smoke, so no native code runs on a CPU the
		// kernels were not built for (ADR-019).
		if err := embed.CheckCPU(); err != nil {
			fmt.Fprintln(os.Stderr, "tb __graph-embed:", err)
			os.Exit(2)
		}
		if graphEmbedSmoke == "" {
			if graphEmbedModel == "" {
				return errors.New("embed_protocol: --model or --smoke is required")
			}
			if err := embed.Serve(cmd.Context(), os.Stdin, os.Stdout, graphEmbedModel, graphEmbedSHA, graphEmbedThreads); err != nil {
				fmt.Fprintln(os.Stderr, "tb __graph-embed:", err)
				os.Exit(2)
			}
			return nil
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
	f.StringVar(&graphEmbedModel, "model", "", "model to serve (absolute path)")
	f.StringVar(&graphEmbedSHA, "sha256", "", "expected sha256 of the model")
	f.IntVar(&graphEmbedThreads, "threads", 2, "inference threads")
	rootCmd.AddCommand(graphEmbedCmd)
}
