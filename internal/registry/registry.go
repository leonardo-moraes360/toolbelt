package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"github.com/leonardo-moraes360/toolbelt/internal/envelope"
)

type Tool struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	InputSchema *InputSchema `json:"inputSchema,omitempty"`
	// Annotations are the MCP behavior hints. Register fills them from the
	// built-in table (annotations.go) when the tool does not set its own.
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
	Handler     ToolHandler      `json:"-"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description,omitempty"`
	Default     any       `json:"default,omitempty"`
	Items       *Property `json:"items,omitempty"`
	Enum        []string  `json:"enum,omitempty"`
}

type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type Registry struct {
	tools map[string]*Tool
}

var defaultRegistry = New()

func New() *Registry {
	return &Registry{tools: make(map[string]*Tool)}
}

func Default() *Registry {
	return defaultRegistry
}

func (r *Registry) Register(tool *Tool) {
	if tool.Annotations == nil {
		tool.Annotations = annotationsFor(tool.Name)
	}
	r.tools[tool.Name] = tool
}

func (r *Registry) Get(name string) (*Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []*Tool {
	out := make([]*Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	// Deterministic order: MCP lists must be cacheable (and stable for the
	// client's prompt cache).
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *Registry) Call(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	tool, ok := r.Get(name)
	if !ok {
		return errorResult(fmt.Sprintf("tool not found: %s", name), "tool_not_found", 2), nil
	}

	result, err := tool.Handler(ctx, args)
	if err != nil {
		return errorResult(err.Error(), errorReason(err.Error()), 1), nil
	}

	if kind, ok := envelope.ToolKinds[name]; ok {
		stamped, serr := stampResult(kind, result)
		if serr != nil {
			return nil, serr
		}
		result = stamped
	}

	text, err := formatResult(result)
	if err != nil {
		return nil, err
	}

	return &ToolResult{Content: []ContentBlock{{Type: "text", Text: text}}}, nil
}

// stampResult adds the envelope "v" to a structured result; strings and byte
// slices are plain text and stay as they are.
func stampResult(kind string, result any) (any, error) {
	switch result.(type) {
	case string, []byte:
		return result, nil
	}
	return envelope.Stamp(kind, result)
}

func errorResult(msg, reason string, exitCode int) *ToolResult {
	b, _ := json.MarshalIndent(envelope.NewError(msg, reason, exitCode), "", "  ")
	return &ToolResult{Content: []ContentBlock{{Type: "text", Text: string(b)}}, IsError: true}
}

var reasonPrefix = regexp.MustCompile(`^([a-z][a-z0-9_]*):`)

// errorReason is the machine-readable reason: the "reason_code:" prefix some
// tool errors already carry (prompt_cli_only, ...), otherwise "error".
func errorReason(msg string) string {
	if m := reasonPrefix.FindStringSubmatch(msg); m != nil {
		return m[1]
	}
	return "error"
}

func formatResult(result any) (string, error) {
	switch v := result.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func Register(tool *Tool) {
	defaultRegistry.Register(tool)
}
