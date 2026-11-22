package main

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestEveryCommandHasATier(t *testing.T) {
	applyTiers(rootCmd)
	for name := range commandTiers {
		if c, _, err := rootCmd.Find([]string{name}); err != nil || c == rootCmd || c.Name() != name {
			t.Errorf("commandTiers lists %q, which is not a command", name)
		}
	}
	var walk func(c *cobra.Command, path string)
	walk = func(c *cobra.Command, path string) {
		for _, sub := range c.Commands() {
			p := strings.TrimSpace(path + " " + sub.Name())
			if sub.Hidden || (path == "" && builtinCommands[sub.Name()]) {
				continue
			}
			tier := sub.Annotations[tierAnnotation]
			switch tier {
			case tierStable:
				if strings.HasPrefix(sub.Short, experimentalTag) {
					t.Errorf("%s is stable but its help says experimental", p)
				}
			case tierExperimental:
				if !strings.HasPrefix(sub.Short, experimentalTag) {
					t.Errorf("%s is experimental but its help line does not start with %q: %q", p, experimentalTag, sub.Short)
				}
				if sub.Long != "" && !strings.HasPrefix(sub.Long, experimentalTag) {
					t.Errorf("%s is experimental but its long help does not start with %q", p, experimentalTag)
				}
			default:
				t.Errorf("%s has no stability tier: add its top-level command to commandTiers", p)
			}
			walk(sub, p)
		}
	}
	walk(rootCmd, "")
}

func TestApplyTiersIsIdempotent(t *testing.T) {
	applyTiers(rootCmd)
	applyTiers(rootCmd)
	if strings.Contains(rootCmd.Commands()[0].Short, experimentalTag+experimentalTag) {
		t.Fatal("experimental tag stacked")
	}
	for _, c := range rootCmd.Commands() {
		if strings.Count(c.Short, experimentalTag) > 1 {
			t.Fatalf("%s: tag stacked: %q", c.Name(), c.Short)
		}
	}
}

var reOverviewRow = regexp.MustCompile(`(?m)^[├└]── (\S+)\s.*\[(stable|experimental)\]\s*$`)

// The overview tree in docs/cli.md is the written form of the tiers: it must
// list exactly the commands the code tiers, with the same tier.
func TestDocsOverviewTiersMatchCommands(t *testing.T) {
	root := findModuleRoot(t)
	for _, rel := range []string{filepath.Join("docs", "cli.md"), filepath.Join("docs", "pt-BR", "cli.md")} {
		doc := mustReadFile(t, filepath.Join(root, rel))
		rows := map[string]string{}
		for _, m := range reOverviewRow.FindAllStringSubmatch(doc, -1) {
			rows[m[1]] = m[2]
		}
		for name, tier := range commandTiers {
			if got := rows[name]; got != tier {
				t.Errorf("%s: overview tree says %q for %s, code says %q", rel, got, name, tier)
			}
		}
		for name := range rows {
			if _, ok := commandTiers[name]; !ok {
				t.Errorf("%s: overview tree lists %s, which has no tier in commandTiers", rel, name)
			}
		}
	}
}
