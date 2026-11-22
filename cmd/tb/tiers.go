package main

import (
	"strings"

	"github.com/spf13/cobra"
)

// Stability tiers (docs/specs/release-1.0.md section 3.1). Subcommands
// inherit the tier of their top-level command; experimental surface says so
// at the start of its help line.
const (
	tierStable       = "stable"
	tierExperimental = "experimental"

	tierAnnotation  = "tier"
	experimentalTag = "(experimental) "
)

// commandTiers has one row per public top-level command. A new command with
// no row fails TestEveryCommandHasATier.
var commandTiers = map[string]string{
	"init": tierStable, "version": tierStable, "mem": tierStable, "verify": tierStable,
	"guard": tierStable, "review": tierStable, "drift": tierStable, "stack": tierStable,
	"sandbox": tierStable, "graph": tierStable, "ticket": tierStable, "workspace": tierStable,
	"merge": tierStable, "mcp": tierStable,

	"board": tierExperimental, "router": tierExperimental, "watchdog": tierExperimental,
	"bus": tierExperimental, "egm": tierExperimental, "vault": tierExperimental,
	"red": tierExperimental, "blue": tierExperimental, "lab": tierExperimental,
	"bench": tierExperimental,
}

// builtinCommands are the Cobra-provided commands, outside the freeze.
var builtinCommands = map[string]bool{"help": true, "completion": true}

// applyTiers stamps every public command with its tier annotation and marks
// the experimental ones in their help line. Hidden commands are internal and
// carry no tier. It is idempotent.
func applyTiers(root *cobra.Command) {
	for _, top := range root.Commands() {
		tier, ok := commandTiers[top.Name()]
		if !ok || top.Hidden {
			continue
		}
		stampTier(top, tier)
	}
}

func stampTier(c *cobra.Command, tier string) {
	if c.Hidden {
		return
	}
	if c.Annotations == nil {
		c.Annotations = map[string]string{}
	}
	c.Annotations[tierAnnotation] = tier
	if tier == tierExperimental && !strings.HasPrefix(c.Short, experimentalTag) {
		c.Short = experimentalTag + c.Short
	}
	for _, sub := range c.Commands() {
		stampTier(sub, tier)
	}
}
