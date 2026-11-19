// Package router implements the foreground ticket scheduler (Phase 3): a
// worker pool, direct delivery (ticket -> worker affinity), retries, and the
// watchdog embedded in the loop (alerts, state.json, file-based control).
//
// On-disk layout (all under .toolbelt/router/, ignored by git):
//
//	state.json         live router state (read by tb router status / watchdog)
//	alerts.jsonl       watchdog alerts (append-only)
//	control/           control requests (kill-<worker>.json)
//	logs/TKT-001.log   per-ticket agent output
package router
