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
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
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
		ref, _ := cmd.Flags().GetString("ref")
		rule := strings.Join(args, " ")
		s := mem.NewStore(dir)
		inv, err := s.Add(category, rule, ref)
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
		all, _ := cmd.Flags().GetBool("all")
		s := mem.NewStore(dir)
		list, err := s.List(category, all)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(list)
		}
		for i := range list {
			fmt.Println(formatInvariantLine(&list[i]))
		}
		return nil
	},
}

var memShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Exibe invariante por id",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s := memStoreFromCWD()
		inv, err := s.Get(args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(inv)
		}
		printInvariant(inv)
		return nil
	},
}

var memEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edita campos de uma invariante",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fields := mem.EditFields{}
		if cmd.Flags().Changed("category") {
			v, _ := cmd.Flags().GetString("category")
			fields.Category = &v
		}
		if cmd.Flags().Changed("rule") {
			v, _ := cmd.Flags().GetString("rule")
			fields.Rule = &v
		}
		if cmd.Flags().Changed("ref") {
			v, _ := cmd.Flags().GetString("ref")
			fields.Ref = &v
		}
		s := memStoreFromCWD()
		inv, err := s.Edit(args[0], fields)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(inv)
		}
		fmt.Printf("invariante %s atualizada\n", inv.ID)
		return nil
	},
}

var memRetireCmd = &cobra.Command{
	Use:   "retire <id>",
	Short: "Aposenta invariante (soft-retire)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s := memStoreFromCWD()
		inv, err := s.Retire(args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(inv)
		}
		fmt.Printf("invariante %s aposentada\n", inv.ID)
		return nil
	},
}

var memSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Busca invariantes",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		query := strings.Join(args, " ")
		s := memStoreFromCWD()
		list, err := s.Search(query, all)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(list)
		}
		for i := range list {
			fmt.Println(formatInvariantLine(&list[i]))
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
		all, _ := cmd.Flags().GetBool("all")
		s := mem.NewStore(dir)
		list, _ := s.List("", all)
		ids := make([]string, 0, len(list))
		for _, inv := range list {
			ids = append(ids, inv.ID)
		}
		rep, err := s.Check(sha, ids, all)
		if err != nil {
			if _, initErr := mem.EnsureDefaultAntiCheat(dir); initErr == nil && sha == "" {
				list, _ = s.List("", all)
				ids = ids[:0]
				for _, inv := range list {
					ids = append(ids, inv.ID)
				}
				rep, err = s.Check("", ids, all)
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
		res, err := mem.EnsureDefaultAntiCheat(dir)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(res)
		}
		if res.Created {
			fmt.Printf("invariantes padrão criadas em %s (%d)\n", res.Path, res.Count)
		} else {
			fmt.Printf("invariantes já existentes em %s (%d)\n", res.Path, res.Count)
		}
		return nil
	},
}

func memStoreFromCWD() *mem.Store {
	cwd, _ := os.Getwd()
	return mem.NewStore(mem.LocateRepoToolbeltDir(cwd))
}

func formatInvariantLine(inv *mem.Invariant) string {
	line := fmt.Sprintf("[%s] %s: %s", inv.ID, inv.Category, inv.Rule)
	if inv.Ref != "" {
		line += fmt.Sprintf(" (ref: %s)", inv.Ref)
	}
	if inv.Retired {
		line += " [retired]"
	}
	return line
}

func printInvariant(inv *mem.Invariant) {
	fmt.Printf("id: %s\n", inv.ID)
	fmt.Printf("category: %s\n", inv.Category)
	fmt.Printf("rule: %s\n", inv.Rule)
	if inv.Ref != "" {
		fmt.Printf("ref: %s\n", inv.Ref)
	}
	fmt.Printf("immutable: %v\n", inv.Immutable)
	fmt.Printf("retired: %v\n", inv.Retired)
	if inv.RetiredAt != nil {
		fmt.Printf("retired_at: %s\n", inv.RetiredAt.Format("2006-01-02T15:04:05Z07:00"))
	}
	fmt.Printf("added_at: %s\n", inv.AddedAt.Format("2006-01-02T15:04:05Z07:00"))
}

func init() {
	memAddCmd.Flags().String("category", "anti_cheat", "categoria: anti_cheat, mandate, security")
	memAddCmd.Flags().String("ref", "", "caminho ADR/decisions.md (ex: docs/adr/0001.md)")
	memListCmd.Flags().String("category", "", "filtrar categoria")
	memListCmd.Flags().Bool("all", false, "incluir invariantes aposentadas")
	memEditCmd.Flags().String("category", "", "nova categoria")
	memEditCmd.Flags().String("rule", "", "nova regra")
	memEditCmd.Flags().String("ref", "", "novo caminho ADR/decisions.md")
	memSearchCmd.Flags().Bool("all", false, "incluir invariantes aposentadas")
	memCheckCmd.Flags().String("expect-sha", "", "hash esperado pós-compactação")
	memCheckCmd.Flags().Bool("all", false, "incluir invariantes aposentadas")
	memCmd.AddCommand(
		memAddCmd,
		memListCmd,
		memShowCmd,
		memEditCmd,
		memRetireCmd,
		memSearchCmd,
		memCheckCmd,
		memInitCmd,
	)
}
