package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/leonardo-moraes360/toolbelt/internal/lab"
	"github.com/leonardo-moraes360/toolbelt/internal/log"
	"github.com/leonardo-moraes360/toolbelt/internal/secplay"
)

func labRoot() string {
	cwd, _ := os.Getwd()
	return lab.RepoRoot(cwd)
}

var labCmd = &cobra.Command{
	Use:   "lab",
	Short: "Isolated red+blue exercise environments (authorized testing only)",
	Long: `tb lab provisions ephemeral, isolated environments to prove a red
playbook's technique works and its paired blue playbook detects it, against a
lab-local target — never the host, never a third party. Phase 1 implements the
single-step sandbox tier (in-process, pure-Go targets); the multi-service
Podman tier fails closed when Podman is unavailable rather than degrading
isolation.`,
	// This overrides the root PersistentPreRunE, so it re-applies SilenceUsage
	// (fail-closed errors must not dump usage) and the authorized-use notice,
	// once per repo, shared with tb red.
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		log.Init(jsonOutput)
		dir := secToolbeltDir()
		if secplay.NeedsAck(dir) {
			fmt.Fprintln(os.Stderr, secplay.AuthorizedUseNotice)
			_ = secplay.RecordAck(dir)
		}
		return nil
	},
}

func newLabNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Scaffolds labs/<name>/lab.yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			class, _ := cmd.Flags().GetString("class")
			target, _ := cmd.Flags().GetString("target")
			path, err := lab.Scaffold(labRoot(), args[0], class, target)
			if err != nil {
				return err
			}
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"name": args[0], "manifest": path})
			}
			fmt.Printf("wrote %s\n", path)
			return nil
		},
	}
	cmd.Flags().String("class", "", "vulnerability class (ties to the playbook pair), e.g. sql-injection")
	cmd.Flags().String("target", "", "build context for the target node (optional)")
	_ = cmd.MarkFlagRequired("class")
	return cmd
}

func newLabListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lists labs in the repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := labRoot()
			names, err := lab.List(root)
			if err != nil {
				return err
			}
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"labs": names})
			}
			for _, n := range names {
				st, _ := lab.ReadState(root, n)
				status := "down"
				if st != nil {
					status = "up (" + string(st.Tier) + ")"
				}
				fmt.Printf("%-20s %s\n", n, status)
			}
			if len(names) == 0 {
				fmt.Println("no labs")
			}
			return nil
		},
	}
}

func newLabStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <name>",
		Short: "Shows a lab's manifest, resolved tier, and tier availability",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := labRoot()
			l, err := lab.Load(root, args[0])
			if err != nil {
				return err
			}
			tier, err := lab.ResolveTier(l, "")
			if err != nil {
				return err
			}
			st, _ := lab.ReadState(root, l.Name)
			sandboxAvail := lab.ProbeSandbox()
			podmanAvail := lab.ProbePodman()
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"lab": l, "resolved_tier": tier, "provisioned": st != nil,
					"sandbox": sandboxAvail, "podman": podmanAvail,
				})
			}
			fmt.Printf("lab:           %s\n", l.Name)
			fmt.Printf("class:         %s\n", l.Class)
			fmt.Printf("resolved tier: %s\n", tier)
			fmt.Printf("provisioned:   %v\n", st != nil)
			fmt.Printf("sandbox tier:  %s\n", availLine(sandboxAvail))
			fmt.Printf("podman tier:   %s\n", availLine(podmanAvail))
			return nil
		},
	}
}

func availLine(a lab.Availability) string {
	if a.Available {
		return "available"
	}
	return "unavailable (" + a.Remediation + ")"
}

func newLabUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up <name>",
		Short: "Provisions a lab; fails closed on a missing tier",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := labRoot()
			l, err := lab.Load(root, args[0])
			if err != nil {
				return err
			}
			override, _ := cmd.Flags().GetString("tier")
			tier, err := lab.ResolveTier(l, override)
			if err != nil {
				return err
			}
			var avail lab.Availability
			switch tier {
			case lab.TierSandbox:
				avail = lab.ProbeSandbox()
			case lab.TierPodman:
				avail = lab.ProbePodman()
			}
			if !avail.Available {
				// Fail closed: never silently downgrade isolation.
				return fmt.Errorf("%s tier unavailable: %s", tier, avail.Remediation)
			}
			if tier == lab.TierPodman {
				return fmt.Errorf("podman topology tier is not implemented yet (Phase 2); use --tier sandbox")
			}
			st := &lab.State{Name: l.Name, Tier: tier, ProvisionedAt: time.Now().UTC()}
			if err := lab.WriteState(root, st); err != nil {
				return err
			}
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(st)
			}
			fmt.Printf("lab %s up (%s tier)\n", l.Name, tier)
			return nil
		},
	}
	cmd.Flags().String("tier", "", "force a tier: sandbox|podman (default: auto)")
	return cmd
}

func newLabRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <name>",
		Short: "Drives the red technique and blue detector; reports a verdict",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := labRoot()
			l, err := lab.Load(root, args[0])
			if err != nil {
				return err
			}
			st, err := lab.ReadState(root, l.Name)
			if err != nil {
				return err
			}
			if st == nil {
				return fmt.Errorf("lab %q is not up; run `tb lab up %s` first", l.Name, l.Name)
			}
			if st.Tier != lab.TierSandbox {
				return fmt.Errorf("lab %q is on the %s tier; only the sandbox tier runs in Phase 1", l.Name, st.Tier)
			}
			v, err := lab.RunSandbox(l)
			if err != nil {
				return err
			}
			st.LastVerdict = v
			_ = lab.WriteState(root, st)
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(v)
			}
			fmt.Printf("lab %s: attack=%s defense=%s\n", v.Lab, v.Attack, v.Defense)
			if v.MatchedCheck != "" {
				fmt.Printf("  detected by: %s\n", v.MatchedCheck)
			}
			fmt.Printf("  %s\n", v.Detail)
			return nil
		},
	}
}

func newLabLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <name>",
		Short: "Shows the last run's result",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := labRoot()
			st, err := lab.ReadState(root, args[0])
			if err != nil {
				return err
			}
			if st == nil || st.LastVerdict == nil {
				return fmt.Errorf("lab %q has no run yet", args[0])
			}
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(st.LastVerdict)
			}
			v := st.LastVerdict
			fmt.Printf("attack=%s defense=%s\n", v.Attack, v.Defense)
			fmt.Printf("observable: %s\n", v.Observable)
			fmt.Printf("%s\n", v.Detail)
			return nil
		},
	}
}

func newLabDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down <name>",
		Short: "Tears a lab down (guaranteed, idempotent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := labRoot()
			if err := lab.RemoveState(root, args[0]); err != nil {
				return err
			}
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{"lab": args[0], "state": "down"})
			}
			fmt.Printf("lab %s down\n", args[0])
			return nil
		},
	}
}

func newLabVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Detects orphaned lab state (provisioned but no manifest)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := labRoot()
			provisioned, err := lab.Provisioned(root)
			if err != nil {
				return err
			}
			var orphans, invalid []string
			for _, n := range provisioned {
				switch {
				case !lab.ManifestExists(root, n):
					// Live state, no manifest: a true orphan to tear down.
					orphans = append(orphans, n)
				default:
					if _, err := lab.Load(root, n); err != nil {
						// Manifest present but broken: not an orphan — do not
						// suggest tearing down a lab that still has a manifest.
						invalid = append(invalid, n)
					}
				}
			}
			clean := len(orphans) == 0 && len(invalid) == 0
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"provisioned": provisioned, "orphans": orphans, "invalid": invalid, "clean": clean,
				})
			}
			if clean {
				fmt.Printf("clean (%d provisioned, 0 orphans)\n", len(provisioned))
				return nil
			}
			for _, n := range orphans {
				fmt.Printf("orphan: %s (state without manifest; run `tb lab down %s`)\n", n, n)
			}
			for _, n := range invalid {
				fmt.Printf("invalid: %s (manifest present but fails to load; fix labs/%s/lab.yaml)\n", n, n)
			}
			return fmt.Errorf("%d orphan(s), %d invalid manifest(s)", len(orphans), len(invalid))
		},
	}
}

func init() {
	labCmd.AddCommand(
		newLabNewCmd(),
		newLabListCmd(),
		newLabStatusCmd(),
		newLabUpCmd(),
		newLabRunCmd(),
		newLabLogsCmd(),
		newLabDownCmd(),
		newLabVerifyCmd(),
	)
	rootCmd.AddCommand(labCmd)
}
