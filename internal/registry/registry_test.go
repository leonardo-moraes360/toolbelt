package registry

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/leonardo-moraes360/toolbelt/internal/envelope"
)

func TestRegisterAndCall(t *testing.T) {
	r := New()
	r.Register(&Tool{
		Name: "echo",
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return args["message"], nil
		},
	})

	res, err := r.Call(context.Background(), "echo", map[string]any{"message": "oi"})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || res.Content[0].Text != "oi" {
		t.Fatalf("resultado inesperado: %+v", res)
	}
}

func TestCallNotFound(t *testing.T) {
	r := New()
	res, err := r.Call(context.Background(), "missing", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("esperava erro")
	}
}

func TestCallHandlerError(t *testing.T) {
	r := New()
	r.Register(&Tool{
		Name: "fail",
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return nil, errors.New("falhou")
		},
	})
	res, err := r.Call(context.Background(), "fail", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("esperava erro")
	}
}

func TestDefaultHasBuiltins(t *testing.T) {
	tools := Default().List()
	if len(tools) == 0 {
		t.Fatal("registro vazio")
	}
	found := false
	for _, t := range tools {
		if t.Name == "tb_version" {
			found = true
		}
	}
	if !found {
		t.Fatal("tb_version não registrada")
	}
}

func TestToolKindsNameRegisteredTools(t *testing.T) {
	for name := range envelope.ToolKinds {
		if _, ok := Default().Get(name); !ok {
			t.Errorf("envelope.ToolKinds names %s, which is not registered", name)
		}
	}
}

func TestCallStampsKindAndErrorBody(t *testing.T) {
	res, err := Default().Call(context.Background(), "tb_version", nil)
	if err != nil || res.IsError {
		t.Fatalf("tb_version: %v %+v", err, res)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].Text), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["v"] != envelope.Version {
		t.Fatalf("v = %v", doc["v"])
	}

	res, err = Default().Call(context.Background(), "tb_no_such_tool", nil)
	if err != nil || !res.IsError {
		t.Fatalf("want isError: %v %+v", err, res)
	}
	var body envelope.ErrorBody
	if err := json.Unmarshal([]byte(res.Content[0].Text), &body); err != nil {
		t.Fatal(err)
	}
	if body.V != envelope.Error || body.Reason != "tool_not_found" || body.ExitCode != 2 {
		t.Fatalf("body = %+v", body)
	}
}
