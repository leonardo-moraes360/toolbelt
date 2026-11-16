package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/registry"
)

func TestInitialize(t *testing.T) {
	reg := registry.New()
	reg.Register(&registry.Tool{
		Name:    "tb_echo",
		Handler: func(ctx context.Context, args map[string]any) (any, error) { return nil, nil },
	})
	srv := NewServer(reg)

	in := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	out := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_ = srv.Run(ctx, strings.NewReader(in), out)

	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("erro: %+v", resp.Error)
	}
}

func TestToolsCall(t *testing.T) {
	reg := registry.New()
	reg.Register(&registry.Tool{
		Name: "tb_echo",
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return args["message"], nil
		},
	})
	srv := NewServer(reg)

	in := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"tb_echo","arguments":{"message":"ok"}}}` + "\n"
	out := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_ = srv.Run(ctx, strings.NewReader(in), out)

	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("erro: %+v", resp.Error)
	}
}

func TestDefaultToolsListHasParity(t *testing.T) {
	srv := NewServer(registry.Default())
	in := `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}` + "\n"
	out := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_ = srv.Run(ctx, strings.NewReader(in), out)

	raw := out.String()
	for _, name := range []string{
		"tb_mem_list", "tb_verify_run", "tb_review", "tb_drift", "tb_guard_check",
	} {
		if !strings.Contains(raw, name) {
			t.Errorf("tools/list sem %s: %s", name, raw)
		}
	}
}
