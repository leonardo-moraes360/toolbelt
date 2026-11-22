// Package envelope is the single place that defines the "v" field of every
// JSON document tb prints with --json and every MCP tool result body
// (docs/cli.md, "JSON envelopes").
//
// The convention is "v": "<kind>/<major>.<minor>". A minor bump adds fields;
// any other change bumps the major. Consumers ignore unknown fields.
package envelope

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

// Frozen kinds, each starting at 1.0. Kinds that already carried a "v" before
// the 1.0 freeze (bus/0.1, bus-wake/0.1, bus-discover/0.1, router-task/0.1,
// egm-source-drift/0.2, egm-draft/0.1, egm-brainstorm/0.1) stay at their own
// numbers and are not stamped here.
const (
	Error        = "error/1.0"
	Version      = "version/1.0"
	Init         = "init/1.0"
	Mem          = "mem/1.0"
	Verify       = "verify/1.0"
	Guard        = "guard/1.0"
	Review       = "review/1.0"
	Drift        = "drift/1.0"
	Stack        = "stack/1.0"
	SandboxRun   = "sandbox-run/1.0"
	SandboxCheck = "sandbox-check/1.0"
	GraphSearch  = "graph-search/1.0"
	GraphCallers = "graph-callers/1.0"
	GraphDeps    = "graph-deps/1.0"
	GraphImpact  = "graph-impact/1.0"
	GraphExplain = "graph-explain/1.0"
	GraphPath    = "graph-path/1.0"
	GraphStats   = "graph-stats/1.0"
	GraphIndex   = "graph-index/1.0"
	GraphRebuild = "graph-rebuild/1.0"
	Ticket       = "ticket/1.0"
	TicketList   = "ticket-list/1.0"
	Workspace    = "workspace/1.0"
	Merge        = "merge/1.0"

	// Experimental kinds (not frozen).
	Board          = "board/1.0"
	Router         = "router/1.0"
	RouterStatus   = "router-status/1.0"
	Watchdog       = "watchdog/1.0"
	WatchdogAlerts = "watchdog-alerts/1.0"
)

// ToolKinds maps an MCP tool to the kind of its result body. The registry
// stamps successful results from this table, so the MCP body matches the CLI's.
var ToolKinds = map[string]string{
	"tb_version":         Version,
	"tb_run":             SandboxRun,
	"tb_mem_add":         Mem,
	"tb_mem_edit":        Mem,
	"tb_mem_retire":      Mem,
	"tb_mem_init":        Mem,
	"tb_mem_list":        Mem,
	"tb_mem_show":        Mem,
	"tb_mem_search":      Mem,
	"tb_mem_check":       Mem,
	"tb_verify_run":      Verify,
	"tb_review":          Review,
	"tb_drift":           Drift,
	"tb_guard_check":     Guard,
	"tb_stack":           Stack,
	"tb_graph_search":    GraphSearch,
	"tb_graph_callers":   GraphCallers,
	"tb_graph_deps":      GraphDeps,
	"tb_graph_impact":    GraphImpact,
	"tb_graph_explain":   GraphExplain,
	"tb_graph_path":      GraphPath,
	"tb_graph_index":     GraphIndex,
	"tb_ticket_list":     TicketList,
	"tb_ticket_show":     Ticket,
	"tb_ticket_create":   Ticket,
	"tb_merge":           Merge,
	"tb_board":           Board,
	"tb_router_status":   RouterStatus,
	"tb_watchdog_alerts": WatchdogAlerts,
}

// listKeys names the field that holds a payload that is a JSON array, since
// an array has no room for "v": the envelope wraps it.
var listKeys = map[string]string{
	Mem:            "invariants",
	TicketList:     "tickets",
	Workspace:      "workspaces",
	Board:          "items",
	WatchdogAlerts: "alerts",
	Router:         "items",
}

var kindPattern = regexp.MustCompile(`^[a-z-]+/\d+\.\d+$`)

// ValidKind reports whether v has the form <kind>/<major>.<minor>.
func ValidKind(v string) bool { return kindPattern.MatchString(v) }

// Stamp returns the JSON of payload with "v": kind as its first field. A
// payload that is a JSON array is wrapped under the kind's list key; one that
// already has a "v" is returned unchanged.
func Stamp(kind string, payload any) (json.RawMessage, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	raw = bytes.TrimSpace(raw)
	switch {
	case len(raw) > 0 && raw[0] == '{':
		return stampObject(kind, raw)
	case len(raw) > 0 && (raw[0] == '[' || bytes.Equal(raw, []byte("null"))):
		key, ok := listKeys[kind]
		if !ok {
			return nil, fmt.Errorf("envelope %s: list payload without a list key", kind)
		}
		if raw[0] == 'n' {
			raw = []byte("[]")
		}
		return stampObject(kind, append(append([]byte(`{"`+key+`":`), raw...), '}'))
	default:
		return nil, fmt.Errorf("envelope %s: payload is not a JSON object or array", kind)
	}
}

func stampObject(kind string, obj []byte) (json.RawMessage, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(obj, &probe); err != nil {
		return nil, err
	}
	if _, has := probe["v"]; has {
		return obj, nil
	}
	head, _ := json.Marshal(kind)
	var out []byte
	out = append(out, `{"v":`...)
	out = append(out, head...)
	if len(probe) > 0 {
		out = append(out, ',')
	}
	return append(out, obj[1:]...), nil
}

// ErrorBody is the error/1.0 document: printed on stdout under --json, and
// the text of an MCP tool error.
type ErrorBody struct {
	V        string `json:"v"`
	Error    string `json:"error"`
	Reason   string `json:"reason"`
	ExitCode int    `json:"exit_code"`
}

// NewError builds an error/1.0 body.
func NewError(msg, reason string, exitCode int) ErrorBody {
	return ErrorBody{V: Error, Error: msg, Reason: reason, ExitCode: exitCode}
}

// ReasonError attaches a machine-readable reason to an error; ReasonOf finds
// it through wrapping.
type ReasonError struct {
	Reason string
	Err    error
}

func (e *ReasonError) Error() string { return e.Err.Error() }
func (e *ReasonError) Unwrap() error { return e.Err }

// knownReasons are the "code: message" prefixes that are reasons, as opposed
// to a package name ("git: ...") that merely looks like one.
var knownReasons = map[string]bool{
	"vectors_disabled": true, "vectors_stale": true, "vectors_invalid": true, "vectors_too_many": true,
	"vectors_root": true, "model_path": true, "model_mismatch": true, "model_unsupported": true,
	"model_load": true, "embed_unavailable": true, "embed_cpu_unsupported": true, "embed_unsandboxed": true,
	"embed_protocol": true, "embed_crashed": true, "embed_timeout": true, "config_untrusted": true,
	"prompt_cli_only": true, "receipt_tampered": true, "receipt_stale": true, "receipt_mismatch": true,
	"receipt_incomplete": true, "secret_files": true, "router_tasks": true, "notify_unsupported": true,
}

var reasonPrefix = regexp.MustCompile(`^([a-z][a-z0-9_]*)(?::|$)`)

// ReasonOf is the reason of an error/1.0 body: a ReasonError's, else a known
// "code:" prefix of the message, else "error".
func ReasonOf(err error) string {
	var re *ReasonError
	if errors.As(err, &re) && re.Reason != "" {
		return re.Reason
	}
	if m := reasonPrefix.FindStringSubmatch(err.Error()); len(m) == 2 && knownReasons[m[1]] {
		return m[1]
	}
	return "error"
}
