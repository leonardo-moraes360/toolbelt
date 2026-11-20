// Package lab provisions ephemeral, isolated red+blue exercise environments
// (`tb lab`). Phase 1 implements the single-step sandbox tier (bubblewrap +
// Landlock, ADR-004) driving an in-process, pure-Go target; the multi-service
// Podman tier is probed and fail-closed (it never silently downgrades
// isolation). See docs/specs/red-blue-labs.md and ADR-017.
package lab

import (
	"fmt"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Tier is the isolation engine a lab runs on.
type Tier string

const (
	// TierAuto lets ResolveTier pick based on the lab's shape.
	TierAuto Tier = "auto"
	// TierSandbox is the single-node, single-step sandbox (Landlock + bwrap).
	TierSandbox Tier = "sandbox"
	// TierPodman is the multi-service rootless-Podman topology (Phase 2).
	TierPodman Tier = "podman"
)

// Egress is the lab network egress policy.
type Egress string

const (
	// EgressDeny is the default: no host route, no internet.
	EgressDeny Egress = "deny"
	// EgressExplicitAllow opens only the destinations the manifest lists.
	EgressExplicitAllow Egress = "explicit-allow"
)

// nameRE constrains lab and node names to a safe, path-friendly shape.
var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// Lab is a parsed lab.yaml manifest.
type Lab struct {
	Name    string  `yaml:"name" json:"name"`
	Class   string  `yaml:"class" json:"class"`
	Tier    Tier    `yaml:"tier,omitempty" json:"tier,omitempty"`
	Network Network `yaml:"network,omitempty" json:"network"`
	Nodes   Nodes   `yaml:"nodes" json:"nodes"`
	Limits  Limits  `yaml:"limits,omitempty" json:"limits"`
}

// Network is the lab's network policy.
type Network struct {
	Egress Egress `yaml:"egress,omitempty" json:"egress"`
	// Internal, when true, puts nodes on one private net with no host route.
	Internal bool `yaml:"internal,omitempty" json:"internal"`
}

// Nodes is the minimal target/red/blue topology. Only Target is required; red
// and blue are optional (a run can name playbooks instead).
type Nodes struct {
	Target Node `yaml:"target" json:"target"`
	Red    Node `yaml:"red,omitempty" json:"red,omitempty"`
	Blue   Node `yaml:"blue,omitempty" json:"blue,omitempty"`
}

// Node is one lab node. For the sandbox tier only the playbook wiring and the
// in-process scenario matter; image/build/expose describe the Podman topology.
type Node struct {
	Image    string `yaml:"image,omitempty" json:"image,omitempty"`
	Build    string `yaml:"build,omitempty" json:"build,omitempty"`
	Expose   []int  `yaml:"expose,omitempty" json:"expose,omitempty"`
	Playbook string `yaml:"playbook,omitempty" json:"playbook,omitempty"`
}

// isZero reports whether a node is unset (used by tier resolution; Node is not
// comparable because of the Expose slice).
func (n Node) isZero() bool {
	return n.Image == "" && n.Build == "" && n.Playbook == "" && len(n.Expose) == 0
}

// Limits caps a lab's resources. Enforced by cgroups v2 on the Podman tier
// (Phase 2); the sandbox tier honors only Timeout today.
type Limits struct {
	CPUs    string `yaml:"cpus,omitempty" json:"cpus,omitempty"`
	Memory  string `yaml:"memory,omitempty" json:"memory,omitempty"`
	PIDs    int    `yaml:"pids,omitempty" json:"pids,omitempty"`
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// TimeoutDuration parses Limits.Timeout, defaulting to 10m when empty and
// erroring on a malformed value.
func (l Limits) TimeoutDuration() (time.Duration, error) {
	if l.Timeout == "" {
		return 10 * time.Minute, nil
	}
	d, err := time.ParseDuration(l.Timeout)
	if err != nil {
		return 0, fmt.Errorf("limits.timeout %q: %w", l.Timeout, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("limits.timeout must be positive, got %q", l.Timeout)
	}
	return d, nil
}

// Parse decodes a lab.yaml manifest and validates it. src is the file bytes.
func Parse(src []byte) (*Lab, error) {
	var l Lab
	if err := yaml.Unmarshal(src, &l); err != nil {
		return nil, fmt.Errorf("lab.yaml: %w", err)
	}
	if l.Tier == "" {
		l.Tier = TierAuto
	}
	if l.Network.Egress == "" {
		l.Network.Egress = EgressDeny
	}
	if err := l.Validate(); err != nil {
		return nil, err
	}
	return &l, nil
}

// Validate checks the manifest's structural invariants. It does not cross-check
// the class against the playbook corpus; the runner does that at run time.
func (l *Lab) Validate() error {
	if !nameRE.MatchString(l.Name) {
		return fmt.Errorf("lab name %q must match %s", l.Name, nameRE)
	}
	if l.Class == "" {
		return fmt.Errorf("lab %q: class is required", l.Name)
	}
	switch l.Tier {
	case TierAuto, TierSandbox, TierPodman:
	default:
		return fmt.Errorf("lab %q: tier %q must be one of auto|sandbox|podman", l.Name, l.Tier)
	}
	switch l.Network.Egress {
	case EgressDeny, EgressExplicitAllow:
	default:
		return fmt.Errorf("lab %q: network.egress %q must be deny|explicit-allow", l.Name, l.Network.Egress)
	}
	if l.Nodes.Target.isZero() {
		return fmt.Errorf("lab %q: a target node is required", l.Name)
	}
	if l.Limits.PIDs < 0 {
		return fmt.Errorf("lab %q: limits.pids must be >= 0", l.Name)
	}
	if _, err := l.Limits.TimeoutDuration(); err != nil {
		return fmt.Errorf("lab %q: %w", l.Name, err)
	}
	return nil
}
