package registry

import "strings"

// Stability tiers (docs/specs/release-1.0.md section 3.1). Experimental
// tools may change in any minor release and say so in their description.
const (
	TierStable       = "stable"
	TierExperimental = "experimental"

	experimentalMarker = "[experimental] "
)

// stableToolPrefixes and experimentalToolPrefixes list every built-in tool
// by exact name or, with a trailing underscore, by family. A tool in neither
// list has no tier and TestEveryToolHasATier fails.
var (
	stableToolPrefixes = []string{
		"tb_version", "tb_run", "tb_mem_", "tb_verify_run", "tb_review", "tb_drift",
		"tb_guard_check", "tb_stack", "tb_graph_", "tb_ticket_", "tb_merge",
	}
	experimentalToolPrefixes = []string{
		"tb_bus_", "tb_router_status", "tb_board", "tb_watchdog_alerts", "tb_egm_",
		"tb_vault_", "tb_red_", "tb_blue_", "tb_sec_pair",
	}
)

// TierOf returns the tier of a tool name, or "" when it has none.
func TierOf(name string) string {
	switch {
	case matchesToolPrefix(name, experimentalToolPrefixes):
		return TierExperimental
	case matchesToolPrefix(name, stableToolPrefixes):
		return TierStable
	}
	return ""
}

func matchesToolPrefix(name string, list []string) bool {
	for _, p := range list {
		if name == p || (strings.HasSuffix(p, "_") && strings.HasPrefix(name, p)) {
			return true
		}
	}
	return false
}
