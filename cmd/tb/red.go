package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/mem"
	"github.com/leonardo-moraes360/toolbelt/internal/secplay"
)

// loadCorpus parses the embedded playbook corpus.
func loadCorpus() (*secplay.Corpus, error) {
	return secplay.Load()
}

func secToolbeltDir() string {
	cwd, _ := os.Getwd()
	return mem.LocateRepoToolbeltDir(cwd)
}

// newSecListCmd builds `tb <color> list`.
func newSecListCmd(color secplay.Color) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("Lists %s playbooks", color),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := loadCorpus()
			if err != nil {
				return err
			}
			area, _ := cmd.Flags().GetString("area")
			pbs := c.List(color, area)
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(secListPayload(pbs))
			}
			for _, pb := range pbs {
				printSecLine(pb.Color, pb.Area, pb.Name, pb.Summary)
			}
			return nil
		},
	}
	cmd.Flags().String("area", "", "filter by area (web, auth, ...)")
	return cmd
}

// newSecSearchCmd builds `tb <color> search`.
func newSecSearchCmd(color secplay.Color) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: fmt.Sprintf("Searches %s playbooks (compact)", color),
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := loadCorpus()
			if err != nil {
				return err
			}
			area, _ := cmd.Flags().GetString("area")
			limit, _ := cmd.Flags().GetInt("limit")
			hits := c.Search(color, strings.Join(args, " "), secplay.SearchOptions{Area: area, Limit: limit})
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"hits": hits})
			}
			for _, h := range hits {
				printSecLine(h.Color, h.Area, h.Name, h.Summary)
			}
			return nil
		},
	}
	cmd.Flags().String("area", "", "filter by area")
	cmd.Flags().Int("limit", 20, "max results")
	return cmd
}

// newSecLoadCmd builds `tb <color> load`.
func newSecLoadCmd(color secplay.Color) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "load <name>",
		Short: fmt.Sprintf("Loads a %s playbook body under a token budget", color),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := loadCorpus()
			if err != nil {
				return err
			}
			pb, ok := c.Get(color, args[0])
			if !ok {
				return fmt.Errorf("%s playbook %q not found", color, args[0])
			}
			section, _ := cmd.Flags().GetString("section")
			budget, _ := cmd.Flags().GetInt("budget")
			full, _ := cmd.Flags().GetBool("full")
			r, err := pb.Render(secplay.RenderOptions{Section: section, Budget: budget, Full: full})
			if err != nil {
				return err
			}
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(r)
			}
			fmt.Printf("# %s\n\n%s\n", r.Name, r.Body)
			if r.Truncated {
				fmt.Printf("\n[truncated to ~%d tokens; omitted sections: %s]\n", r.Tokens, strings.Join(r.Remaining, ", "))
			}
			return nil
		},
	}
	cmd.Flags().String("section", "", "return only this section (heading or slug)")
	cmd.Flags().Int("budget", 0, "token budget (default 2000)")
	cmd.Flags().Bool("full", false, "return the whole body, ignoring the budget")
	return cmd
}

// newSecPairCmd builds `tb <color> pair`.
func newSecPairCmd(color secplay.Color) *cobra.Command {
	return &cobra.Command{
		Use:   "pair <name>",
		Short: "Resolves the mirror playbook in the opposite color",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := loadCorpus()
			if err != nil {
				return err
			}
			m, ok := c.Pair(args[0])
			if !ok {
				return fmt.Errorf("no mirror found for %q", args[0])
			}
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"mirror": m.Name, "color": m.Color, "summary": m.Summary})
			}
			printSecLine(m.Color, m.Area, m.Name, m.Summary)
			return nil
		},
	}
}

func printSecLine(color secplay.Color, area, name, summary string) {
	tag := string(color)
	if area != "" {
		tag = string(color) + "/" + area
	}
	fmt.Printf("[%s] %s — %s\n", tag, name, firstClause(summary))
}

func firstClause(s string) string {
	if i := strings.IndexByte(s, ':'); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	if i := strings.IndexByte(s, '.'); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func secListPayload(pbs []*secplay.Playbook) map[string]any {
	type row struct {
		Name    string        `json:"name"`
		Color   secplay.Color `json:"color"`
		Area    string        `json:"area"`
		Class   string        `json:"class"`
		Summary string        `json:"summary"`
	}
	rows := make([]row, 0, len(pbs))
	for _, pb := range pbs {
		rows = append(rows, row{pb.Name, pb.Color, pb.Area, pb.Class, pb.Summary})
	}
	return map[string]any{"playbooks": rows}
}

var redCmd = &cobra.Command{
	Use:   "red",
	Short: "Offensive AppSec playbooks (authorized testing only)",
	Long: `tb red serves native offensive AppSec methodology: how a vulnerability
class manifests in code, how to confirm it, and the remediation its blue mirror
enforces. Search is compact; load respects a token budget. Playbooks are
reference text — tb red never executes anything.`,
	// Authorized-use notice, once per repo.
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		dir := secToolbeltDir()
		if secplay.NeedsAck(dir) {
			// Always to stderr, so it shows in --json mode too without
			// corrupting stdout; record only after it is shown.
			fmt.Fprintln(os.Stderr, secplay.AuthorizedUseNotice)
			_ = secplay.RecordAck(dir)
		}
		return nil
	},
}

// newSecExplainCmd builds `tb red explain <class>`: attack and defense together.
func newSecExplainCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "explain <class>",
		Short: "Shows the red playbook and its blue mirror side by side",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := loadCorpus()
			if err != nil {
				return err
			}
			class := args[0]
			var red, blue *secplay.Playbook
			for _, pb := range c.List(secplay.Red, "") {
				if pb.Class == class || pb.Name == class {
					red = pb
					break
				}
			}
			if red == nil {
				return fmt.Errorf("no red playbook for class %q", class)
			}
			blue, _ = c.Pair(red.Name)
			if jsonOutput {
				out := map[string]any{"red": red.Name}
				if blue != nil {
					out["blue"] = blue.Name
				}
				return json.NewEncoder(os.Stdout).Encode(out)
			}
			printSecLine(red.Color, red.Area, red.Name, red.Summary)
			if blue != nil {
				printSecLine(blue.Color, blue.Area, blue.Name, blue.Summary)
			}
			return nil
		},
	}
}

func init() {
	redCmd.AddCommand(
		newSecListCmd(secplay.Red),
		newSecSearchCmd(secplay.Red),
		newSecLoadCmd(secplay.Red),
		newSecPairCmd(secplay.Red),
		newSecExplainCmd(),
	)
	rootCmd.AddCommand(redCmd)
}
