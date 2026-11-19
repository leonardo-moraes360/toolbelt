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
	Short: "Invariants outside compaction (ADR-010)",
	Long: `Manages the invariants file at .toolbelt/invariants.yaml.

Rules in this file are not subject to context compaction and are
re-injected intact after each summarization cycle.`,
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
	Use:   "add <rule>",
	Short: "Adds an invariant",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		dir := mem.LocateRepoToolbeltDir(cwd)
		category, _ := cmd.Flags().GetString("category")
		if category == "" {
			category = "anti_cheat"
		}
		ref, _ := cmd.Flags().GetString("ref")
		immutable, _ := cmd.Flags().GetBool("immutable")
		rule := strings.Join(args, " ")
		s := mem.NewStore(dir)
		inv, err := s.Add(category, rule, ref, immutable)
		if err != nil {
			return err
		}
		out := mem.RedactInvariant(inv)
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(out)
		}
		fmt.Printf("invariant %s added\n", out.ID)
		return nil
	},
}

var memListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists invariants",
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
		list = mem.RedactInvariants(list)
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
	Short: "Shows an invariant by id",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s := memStoreFromCWD()
		inv, err := s.Get(args[0])
		if err != nil {
			return err
		}
		out := mem.RedactInvariant(inv)
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(out)
		}
		printInvariant(&out)
		return nil
	},
}

var memEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edits fields of an invariant",
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
		force, _ := cmd.Flags().GetBool("force")
		reason, _ := cmd.Flags().GetString("reason")
		s := memStoreFromCWD()
		inv, err := s.Edit(args[0], fields, mem.MutateOpts{Force: force, Reason: reason})
		if err != nil {
			return err
		}
		out := mem.RedactInvariant(inv)
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(out)
		}
		fmt.Printf("invariant %s updated\n", out.ID)
		return nil
	},
}

var memRetireCmd = &cobra.Command{
	Use:   "retire <id>",
	Short: "Retires an invariant (soft-retire)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		reason, _ := cmd.Flags().GetString("reason")
		s := memStoreFromCWD()
		inv, err := s.Retire(args[0], mem.MutateOpts{Force: force, Reason: reason})
		if err != nil {
			return err
		}
		out := mem.RedactInvariant(inv)
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(out)
		}
		fmt.Printf("invariant %s retired\n", out.ID)
		return nil
	},
}

var memSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Searches invariants",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		query := strings.Join(args, " ")
		s := memStoreFromCWD()
		list, err := s.Search(query, all)
		if err != nil {
			return err
		}
		list = mem.RedactInvariants(list)
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
	Short: "Checks the integrity of the invariants file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		dir := mem.LocateRepoToolbeltDir(cwd)
		sha, _ := cmd.Flags().GetString("expect-sha")
		all, _ := cmd.Flags().GetBool("all")
		s := mem.NewStore(dir)
		list, err := s.List("", all)
		if err != nil {
			return err
		}
		ids := make([]string, 0, len(list))
		for _, inv := range list {
			ids = append(ids, inv.ID)
		}
		rep, err := s.Check(sha, ids, all)
		if rep != nil && jsonOutput {
			_ = json.NewEncoder(os.Stdout).Encode(rep)
		}
		if err != nil {
			return err
		}
		if jsonOutput {
			return nil
		}
		fmt.Printf("file: %s\n", rep.Path)
		fmt.Printf("sha256: %s\n", rep.SHA256)
		fmt.Printf("invariants: %d\n", rep.Count)
		fmt.Printf("compaction_safe: %v\n", rep.CompactSafe)
		fmt.Printf("retention: %.2f\n", rep.Retention)
		return nil
	},
}

var memInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Creates default anti-cheat invariants",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, _ := os.Getwd()
		dir := mem.LocateRepoToolbeltDir(cwd)
		result, err := mem.EnsureDefaultAntiCheat(dir)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(result)
		}
		if result.Created {
			fmt.Printf("created %d invariants in %s\n", result.Count, result.Path)
		} else {
			fmt.Printf("%d invariants already existed in %s\n", result.Count, result.Path)
		}
		for i := range result.Invariants {
			fmt.Println(formatInvariantLine(&result.Invariants[i]))
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
	memAddCmd.Flags().String("category", "anti_cheat", "category: anti_cheat, mandate, security")
	memAddCmd.Flags().String("ref", "", "ADR/decisions.md path (e.g. docs/adr/0001.md)")
	memAddCmd.Flags().Bool("immutable", false, "marks the invariant as immutable (default: mutable)")
	memListCmd.Flags().String("category", "", "filter by category")
	memListCmd.Flags().Bool("all", false, "include retired invariants")
	memEditCmd.Flags().String("category", "", "new category")
	memEditCmd.Flags().String("rule", "", "new rule")
	memEditCmd.Flags().String("ref", "", "new ADR/decisions.md path")
	memEditCmd.Flags().Bool("force", false, "allows editing an immutable/anti_cheat invariant")
	memEditCmd.Flags().String("reason", "", "reason, required with --force")
	memRetireCmd.Flags().Bool("force", false, "allows retiring an immutable/anti_cheat invariant")
	memRetireCmd.Flags().String("reason", "", "reason, required with --force")
	memSearchCmd.Flags().Bool("all", false, "include retired invariants")
	memCheckCmd.Flags().String("expect-sha", "", "expected hash after compaction")
	memCheckCmd.Flags().Bool("all", false, "include retired invariants")
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
