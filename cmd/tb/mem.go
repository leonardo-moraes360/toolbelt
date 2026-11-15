package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/mem"
)

var memCmd = &cobra.Command{
	Use:   "mem",
	Short: "Invariantes fora da compactação (ADR-010)",
	Long: `Gerencia o arquivo de invariantes em .toolbelt/invariants.yaml.

Regras neste arquivo não passam por compactação de contexto e são
reinjetadas intactas após cada ciclo de resumo.`,
}

var memAddCmd = &cobra.Command{
	Use:   "add <regra>",
	Short: "Adiciona invariante",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		dir := mem.LocateRepoToolbeltDir(cwd)
		category, _ := cmd.Flags().GetString("category")
		if category == "" {
			category = "anti_cheat"
		}
		rule := strings.Join(args, " ")
		s := mem.NewStore(dir)
		inv, err := s.Add(category, rule)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(inv)
		}
		fmt.Printf("invariante %s adicionada\n", inv.ID)
		return nil
	},
}

var memListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista invariantes",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		dir := mem.LocateRepoToolbeltDir(cwd)
		category, _ := cmd.Flags().GetString("category")
		s := mem.NewStore(dir)
		list, err := s.List(category)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(list)
		}
		for _, inv := range list {
			fmt.Printf("[%s] %s: %s\n", inv.ID, inv.Category, inv.Rule)
		}
		return nil
	},
}

var memCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Verifica integridade do arquivo de invariantes",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		dir := mem.LocateRepoToolbeltDir(cwd)
		sha, _ := cmd.Flags().GetString("expect-sha")
		s := mem.NewStore(dir)
		list, _ := s.List("")
		ids := make([]string, 0, len(list))
		for _, inv := range list {
			ids = append(ids, inv.ID)
		}
		rep, err := s.Check(sha, ids)
		if err != nil {
			if initErr := mem.EnsureDefaultAntiCheat(dir); initErr == nil && sha == "" {
				list, _ = s.List("")
				ids = ids[:0]
				for _, inv := range list {
					ids = append(ids, inv.ID)
				}
				rep, err = s.Check("", ids)
			}
			if err != nil {
				return err
			}
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(rep)
		}
		fmt.Printf("arquivo: %s\n", rep.Path)
		fmt.Printf("sha256: %s\n", rep.SHA256)
		fmt.Printf("invariantes: %d\n", rep.Count)
		fmt.Printf("compactacao_segura: %v\n", rep.CompactSafe)
		fmt.Printf("retencao: %.2f\n", rep.Retention)
		if len(rep.Missing) > 0 {
			return fmt.Errorf("invariantes ausentes: %v", rep.Missing)
		}
		return nil
	},
}

var memInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Cria invariantes padrão anti-trapaça",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		dir := mem.LocateRepoToolbeltDir(cwd)
		return mem.EnsureDefaultAntiCheat(dir)
	},
}

func init() {
	memAddCmd.Flags().String("category", "anti_cheat", "categoria: anti_cheat, mandate, security")
	memListCmd.Flags().String("category", "", "filtrar categoria")
	memCheckCmd.Flags().String("expect-sha", "", "hash esperado pós-compactação")
	memCmd.AddCommand(memAddCmd, memListCmd, memCheckCmd, memInitCmd)
}
