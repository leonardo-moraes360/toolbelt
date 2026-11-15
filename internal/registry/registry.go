package registry

import (
	"context"
	"encoding/json"
	"fmt"
)

type Tool struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	InputSchema *InputSchema `json:"inputSchema,omitempty"`
	Handler     ToolHandler  `json:"-"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`
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
	return out
}

func (r *Registry) Call(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	tool, ok := r.Get(name)
	if !ok {
		return &ToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("ferramenta não encontrada: %s", name)}},
			IsError: true,
		}, nil
	}

	result, err := tool.Handler(ctx, args)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{Type: "text", Text: err.Error()}},
			IsError: true,
		}, nil
	}

	text, err := formatResult(result)
	if err != nil {
		return nil, err
	}

	return &ToolResult{Content: []ContentBlock{{Type: "text", Text: text}}}, nil
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
