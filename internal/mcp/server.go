package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/leonardo-moraes360/toolbelt/internal/registry"
	"github.com/leonardo-moraes360/toolbelt/internal/version"
)

const ProtocolVersion = "2024-11-05"

type Server struct {
	reg    *registry.Registry
	input  io.Reader
	output io.Writer
	mu     sync.Mutex
}

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

const (
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
)

func NewServer(reg *registry.Registry) *Server {
	return &Server{
		reg:    reg,
		input:  nil,
		output: nil,
	}
}

func (s *Server) SetIO(input io.Reader, output io.Writer) {
	s.input = input
	s.output = output
}

func (s *Server) Run(ctx context.Context, input io.Reader, output io.Writer) error {
	if input != nil {
		s.input = input
	}
	if output != nil {
		s.output = output
	}

	scanner := bufio.NewScanner(s.input)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			_ = s.sendError(nil, ErrCodeParse, "erro ao parsear JSON", err.Error())
			continue
		}

		if req.JSONRPC != "2.0" {
			_ = s.sendError(req.ID, ErrCodeInvalidRequest, "versão JSON-RPC inválida", nil)
			continue
		}

		if err := s.dispatch(ctx, &req); err != nil {
			slog.Error("erro ao processar requisição MCP", "method", req.Method, "error", err)
		}
	}

	return scanner.Err()
}

func (s *Server) dispatch(ctx context.Context, req *Request) error {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized", "notifications/initialized":
		return nil
	case "ping":
		return s.handlePing(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		_ = s.sendError(req.ID, ErrCodeMethodNotFound, fmt.Sprintf("método não encontrado: %s", req.Method), nil)
		return nil
	}
}

func (s *Server) handleInitialize(req *Request) error {
	result := map[string]any{
		"protocolVersion": ProtocolVersion,
		"serverInfo": map[string]any{
			"name":    "toolbelt",
			"version": version.Version,
		},
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
	}
	return s.sendResult(req.ID, result)
}

func (s *Server) handlePing(req *Request) error {
	return s.sendResult(req.ID, map[string]any{})
}

func (s *Server) handleToolsList(req *Request) error {
	tools := s.reg.List()
	list := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		item := map[string]any{
			"name":        t.Name,
			"description": t.Description,
		}
		if t.InputSchema != nil {
			item["inputSchema"] = t.InputSchema
		} else {
			item["inputSchema"] = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		list = append(list, item)
	}
	return s.sendResult(req.ID, map[string]any{"tools": list})
}

func (s *Server) handleToolsCall(ctx context.Context, req *Request) error {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		_ = s.sendError(req.ID, ErrCodeInvalidParams, "parâmetros inválidos", err.Error())
		return nil
	}

	result, err := s.reg.Call(ctx, params.Name, params.Arguments)
	if err != nil {
		_ = s.sendError(req.ID, ErrCodeInternal, "erro interno", err.Error())
		return nil
	}

	return s.sendResult(req.ID, result)
}

func (s *Server) sendResult(id, result any) error {
	return s.send(&Response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) sendError(id any, code int, message string, data any) error {
	return s.send(&Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &Error{Code: code, Message: message, Data: data},
	})
}

func (s *Server) send(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.output, "%s\n", data)
	return err
}
