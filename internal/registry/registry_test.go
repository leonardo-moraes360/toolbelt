package registry

import (
	"context"
	"errors"
	"testing"
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
