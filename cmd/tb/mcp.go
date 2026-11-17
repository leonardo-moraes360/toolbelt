package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/log"
	"github.com/leonardo-moraes360/toolbelt/internal/mcp"
	"github.com/leonardo-moraes360/toolbelt/internal/registry"
	"github.com/leonardo-moraes360/toolbelt/internal/ticket"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Inicia servidor MCP sobre stdio",
	Long: `Inicia o toolbelt como servidor MCP (Model Context Protocol).

O servidor expõe ferramentas registradas via JSON-RPC 2.0 sobre stdio,
alinhado ao MCP stateless. As mesmas ferramentas do CLI ficam disponíveis via tools/list e tools/call.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := bindBusAs(cmd); err != nil {
			return err
		}
		log.Info("servidor MCP iniciado", "transport", "stdio")
		srv := mcp.NewServer(registry.Default())
		return srv.Run(cmd.Context(), os.Stdin, os.Stdout)
	},
}

// bindBusAs trata --bus-as <canal>/<nome>: liga a conexão à sessão CLI do
// agente (mesmo limite de confiança da CLI; usado por tb bus run).
func bindBusAs(cmd *cobra.Command) error {
	as, _ := cmd.Flags().GetString("bus-as")
	if as == "" {
		return nil
	}
	channel, name, ok := strings.Cut(as, "/")
	if !ok || channel == "" || name == "" {
		return fmt.Errorf("--bus-as inválido %q (use <canal>/<nome>)", as)
	}
	root, _ := cmd.Flags().GetString("bus-root")
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root = ticket.FindRoot(cwd)
	}
	return registry.BindBusSession(root, channel, name)
}

func init() {
	mcpCmd.Flags().String("bus-as", "", "prende a conexão à sessão do bus <canal>/<nome> (usado por tb bus run)")
	mcpCmd.Flags().String("bus-root", "", "raiz do repositório do bus (padrão: procura .toolbelt a partir do cwd)")
}
