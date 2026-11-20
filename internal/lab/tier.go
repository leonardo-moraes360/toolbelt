package lab

import "fmt"

// ResolveTier decides which isolation tier a lab runs on. An explicit override
// (sandbox|podman) wins. Otherwise TierAuto picks sandbox for a single-node,
// single-step lab (no inter-node network, no red/blue container nodes) and
// podman for anything with a network topology or more than the target node.
func ResolveTier(l *Lab, override string) (Tier, error) {
	switch Tier(override) {
	case TierSandbox:
		return TierSandbox, nil
	case TierPodman:
		return TierPodman, nil
	case "", TierAuto:
		// fall through to resolution
	default:
		return "", fmt.Errorf("tier override %q must be sandbox|podman", override)
	}

	switch l.Tier {
	case TierSandbox, TierPodman:
		return l.Tier, nil
	case TierAuto, "":
		if isSingleStep(l) {
			return TierSandbox, nil
		}
		return TierPodman, nil
	default:
		return "", fmt.Errorf("lab %q: unknown tier %q", l.Name, l.Tier)
	}
}

// isSingleStep reports whether a lab fits the single-node sandbox tier: only a
// target node, and no private inter-node network.
func isSingleStep(l *Lab) bool {
	if l.Network.Internal {
		return false
	}
	if !l.Nodes.Red.isZero() || !l.Nodes.Blue.isZero() {
		return false
	}
	return true
}
