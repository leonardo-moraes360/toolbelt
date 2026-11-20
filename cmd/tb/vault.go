package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/vault"
)

var vaultFilter vault.Filter

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Obsidian-compatible Markdown vault of project knowledge",
	Long: `tb vault reads the notes in vault.path (default .toolbelt/vault/): Markdown
files with optional YAML frontmatter and [[wikilinks]], editable in Obsidian or
any editor (docs/specs/vault.md). The files are the only store: every command
scans the vault from disk. Ambiguous wikilinks are errors, where Obsidian
would pick the shortest path. Note text in --json output sits inside "data"
objects and is redacted.`,
}

var vaultListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists notes (path, title, type, tags, source, updated), sorted by path",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runVault(func(v *vault.Vault) (any, int, error) {
			r, err := v.List(vaultFilter)
			return r, 0, err
		}, printVaultList)
	},
}

var vaultShowCmd = &cobra.Command{
	Use:   "show <note>",
	Short: "Shows a note's frontmatter and body; --json adds links and backlinks",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runVault(func(v *vault.Vault) (any, int, error) {
			r, err := v.Show(args[0])
			return r, 0, err
		}, printVaultShow)
	},
}

var vaultLinksCmd = &cobra.Command{
	Use:   "links <note>",
	Short: "Lists a note's outgoing wikilinks with their state",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runVault(func(v *vault.Vault) (any, int, error) {
			r, err := v.Links(args[0])
			return r, 0, err
		}, printVaultLinks)
	},
}

var vaultBacklinksCmd = &cobra.Command{
	Use:   "backlinks <note>",
	Short: "Lists the notes that link to a note",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runVault(func(v *vault.Vault) (any, int, error) {
			r, err := v.Backlinks(args[0])
			return r, 0, err
		}, printVaultBacklinks)
	},
}

var vaultCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Validates the vault (exit 1 on broken or ambiguous links, bad frontmatter, oversized notes)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runVault(func(v *vault.Vault) (any, int, error) {
			r, err := v.Check()
			if err != nil {
				return nil, 0, err
			}
			return r, r.ExitCode, nil
		}, printVaultCheck)
	},
}

func init() {
	f := vaultListCmd.Flags()
	f.StringVar(&vaultFilter.Type, "type", "", "only notes of this type")
	f.StringVar(&vaultFilter.Tag, "tag", "", "only notes with this tag")
	f.StringVar(&vaultFilter.Source, "source", "", "only notes with this source (manual, extracted, inferred)")
	vaultCmd.AddCommand(vaultListCmd, vaultShowCmd, vaultLinksCmd, vaultBacklinksCmd, vaultCheckCmd)
	rootCmd.AddCommand(vaultCmd)
}

func runVault(fn func(*vault.Vault) (any, int, error), text func(any)) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	v, err := vault.Open(cwd)
	if err != nil {
		return vaultFail(err)
	}
	out, code, err := fn(v)
	if err != nil {
		return vaultFail(err)
	}
	if jsonOutput {
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
	} else {
		text(out)
	}
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

func vaultFail(err error) error {
	code, reason := vault.ExitError, ""
	var e *vault.Error
	if errors.As(err, &e) {
		code, reason = e.Code, e.Reason
	}
	if jsonOutput {
		b, _ := json.Marshal(map[string]any{"error": err.Error(), "reason": reason, "exit_code": code})
		fmt.Println(string(b))
	} else {
		fmt.Fprintln(os.Stderr, "tb vault:", err)
	}
	os.Exit(code)
	return nil
}

// one quotes a single-line field that holds control characters (a file
// name or title with a newline or an ANSI escape would otherwise forge
// output lines); JSON output is escaped already.
func one(s string) string {
	if strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return strconv.Quote(s)
	}
	return s
}

// block keeps newlines and tabs of a multi-line text and drops every other
// control character.
func block(s string) string {
	return strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func ones(list []string) string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = one(s)
	}
	return strings.Join(out, ",")
}

func printVaultList(v any) {
	r := v.(*vault.ListResult)
	for i := range r.Notes {
		n := &r.Notes[i]
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\n", one(n.Path), one(n.Data.Title), n.Type, ones(n.Data.Tags), n.Source, n.Updated)
	}
}

func printVaultShow(v any) {
	r := v.(*vault.ShowResult)
	n := r.Note
	fmt.Println("---")
	fmt.Printf("title: %q\ntype: %s\nsource: %s\n", n.Data.Title, n.Type, n.Source)
	if n.Confidence != nil {
		fmt.Printf("confidence: %g\n", *n.Confidence)
	}
	if len(n.Data.Tags) > 0 {
		fmt.Printf("tags: [%s]\n", ones(n.Data.Tags))
	}
	if len(n.Data.Aliases) > 0 {
		fmt.Printf("aliases: [%s]\n", ones(n.Data.Aliases))
	}
	for _, kv := range [][2]string{{"created", n.Created}, {"updated", n.Updated}} {
		if kv[1] != "" {
			fmt.Printf("%s: %s\n", kv[0], kv[1])
		}
	}
	fmt.Println("---")
	fmt.Println(block(n.Data.Body))
}

func printVaultLinks(v any) {
	r := v.(*vault.LinksResult)
	for i := range r.Links {
		l := &r.Links[i]
		fmt.Printf("%d:%d\t%s\t%s\t%s\n", l.Line, l.Col, l.State, one(linkLabel(l.Data)), one(l.To))
	}
}

func linkLabel(d vault.LinkData) string {
	s := d.Target
	if d.Fragment != "" {
		s += "#" + d.Fragment
	}
	return "[[" + s + "]]"
}

func printVaultBacklinks(v any) {
	r := v.(*vault.BacklinksResult)
	for _, b := range r.Backlinks {
		fmt.Printf("%s\t%s:%d:%d\n", one(b.Path), one(b.File), b.Line, b.Col)
	}
}

func printVaultCheck(v any) {
	r := v.(*vault.CheckResult)
	for _, is := range r.Issues {
		loc := one(is.File)
		if is.Line > 0 {
			loc = fmt.Sprintf("%s:%d", loc, is.Line)
			if is.Col > 0 {
				loc = fmt.Sprintf("%s:%d", loc, is.Col)
			}
		}
		detail := ""
		if is.Data != nil {
			detail = one(strings.TrimSpace(is.Data.Target + " " + is.Data.Detail))
		}
		fmt.Printf("%s: %s %s %s\n", loc, is.Level, is.Code, detail)
	}
	fmt.Printf("tb vault check: %d notes, %d errors, %d warnings\n", r.Notes, r.Errors, r.Warnings)
}
