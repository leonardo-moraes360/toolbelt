package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/log"
	"github.com/leonardo-moraes360/toolbelt/internal/mcp"
	"github.com/leonardo-moraes360/toolbelt/internal/registry"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Inicia servidor MCP sobre stdio",
	Long: `Inicia o toolbelt como servidor MCP (Model Context Protocol).

O servidor expõe ferramentas registradas via JSON-RPC 2.0 sobre stdio,
alinhado ao MCP stateless. As mesmas ferramentas do CLI ficam disponíveis via tools/list e tools/call.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Info("servidor MCP iniciado", "transport", "stdio")
		srv := mcp.NewServer(registry.Default())
		return srv.Run(cmd.Context(), os.Stdin, os.Stdout)
	},
}
