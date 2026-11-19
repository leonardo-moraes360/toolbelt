package registry

// ToolAnnotations are the MCP behavior hints for a tool. All four fields are
// always serialized: the spec defaults (readOnly=false, destructive=true,
// idempotent=false, openWorld=true) are the pessimistic case, so an explicit
// false carries information.
//
// Each value must reflect what the handler actually does, not what the tool
// name suggests (see docs/pesquisa/mcp-sinais-m8ven.md).
type ToolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint"`
	DestructiveHint bool `json:"destructiveHint"`
	IdempotentHint  bool `json:"idempotentHint"`
	OpenWorldHint   bool `json:"openWorldHint"`
}

var (
	// Reads process or .toolbelt state, writes nothing.
	annReadOnly = ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}
	// Overwrites existing state; repeating with the same input converges.
	annOverwrite = ToolAnnotations{DestructiveHint: true, IdempotentHint: true}
	// Appends a new item on every call.
	annAppend = ToolAnnotations{}
	// Creates missing defaults only; never overwrites.
	annEnsure = ToolAnnotations{IdempotentHint: true}
	// Writes to the bus that other agents read; each call has a new effect.
	annBus = ToolAnnotations{OpenWorldHint: true}
	// Runs user/agent commands: any effect, including network.
	annExec = ToolAnnotations{DestructiveHint: true, OpenWorldHint: true}
)

// toolAnnotations is the single source of hints for every built-in tool.
// TestAllToolsAnnotated fails when a registered tool is missing here.
var toolAnnotations = map[string]ToolAnnotations{
	"tb_version": annReadOnly,
	"tb_echo":    annReadOnly,
	"tb_env":     annReadOnly,
	"tb_run":     annExec,

	// MCP graph queries never reindex; they report stale:true instead.
	"tb_graph_search":  annReadOnly,
	"tb_graph_callers": annReadOnly,
	"tb_graph_deps":    annReadOnly,
	"tb_graph_impact":  annReadOnly,
	"tb_graph_explain": annReadOnly,
	"tb_graph_path":    annReadOnly,
	"tb_graph_index":   annOverwrite,

	// apply:true moves refs; hints cover the worst case, not the dry-run default.
	"tb_merge": {DestructiveHint: true},

	"tb_ticket_list":   annReadOnly,
	"tb_ticket_show":   annReadOnly,
	"tb_ticket_create": annAppend,
	// pause/resume/drain change router state; list only reads.
	"tb_board": annOverwrite,

	"tb_bus_join":     annBus,
	"tb_bus_send":     annBus,
	"tb_bus_recv":     annBus, // advances the cursor: a repeat returns another batch
	"tb_bus_claim":    annBus,
	"tb_bus_complete": annBus,

	"tb_mem_add":    annAppend,
	"tb_mem_edit":   annOverwrite,
	"tb_mem_retire": annOverwrite,
	"tb_mem_init":   annEnsure,
	"tb_mem_list":   annReadOnly,
	"tb_mem_show":   annReadOnly,
	"tb_mem_search": annReadOnly,
	"tb_mem_check":  annReadOnly,

	"tb_verify_run":  annExec,
	"tb_review":      annReadOnly,
	"tb_drift":       annReadOnly,
	"tb_guard_check": annReadOnly,
	"tb_stack":       annExec,

	// status reads the state outside the repository; resolve rewrites it, and
	// a transition to the current status is a no-op (R28).
	"tb_egm_status":  annReadOnly,
	"tb_egm_resolve": annOverwrite,

	"tb_router_status":   annReadOnly,
	"tb_watchdog_alerts": annReadOnly,
}

// annotationsFor returns the built-in hints for name, if any.
func annotationsFor(name string) *ToolAnnotations {
	a, ok := toolAnnotations[name]
	if !ok {
		return nil
	}
	return &a
}
