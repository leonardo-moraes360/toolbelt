package registry

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/version"
)

func init() {
	RegisterBuiltins()
}

func RegisterBuiltins() {
	Register(&Tool{
		Name:        "tb_version",
		Description: "Retorna informações de versão do toolbelt",
		Handler:     handleVersion,
	})
	Register(&Tool{
		Name:        "tb_echo",
		Description: "Ecoa a mensagem de entrada",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"message": {Type: "string", Description: "Mensagem para ecoar"},
			},
			Required: []string{"message"},
		},
		Handler: handleEcho,
	})
	Register(&Tool{
		Name:        "tb_env",
		Description: "Retorna informações do ambiente de execução",
		Handler:     handleEnv,
	})
	Register(&Tool{
		Name:        "tb_run",
		Description: "Executa processo no shell com timeout opcional",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"command": {Type: "string", Description: "Processo a executar"},
				"timeout": {Type: "string", Description: "Timeout (ex: 30s)", Default: "30s"},
				"cwd":     {Type: "string", Description: "Diretório de trabalho"},
			},
			Required: []string{"command"},
		},
		Handler: handleRun,
	})
}

func handleVersion(_ context.Context, _ map[string]any) (any, error) {
	return version.Get(), nil
}

func handleEcho(_ context.Context, args map[string]any) (any, error) {
	msg, _ := args["message"].(string)
	return msg, nil
}

func handleEnv(_ context.Context, _ map[string]any) (any, error) {
	cwd, _ := os.Getwd()
	return map[string]any{
		"cwd":        cwd,
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"go_version": runtime.Version(),
		"tb_home":    os.Getenv("TB_HOME"),
	}, nil
}

func handleRun(ctx context.Context, args map[string]any) (any, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("processo não especificado")
	}
	timeoutStr, _ := args["timeout"].(string)
	if timeoutStr == "" {
		timeoutStr = "30s"
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("timeout inválido: %w", err)
	}
	cwd, _ := args["cwd"].(string)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command) //nolint:gosec // ferramenta MCP explícita
	if cwd != "" {
		cmd.Dir = cwd
	}

	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			return map[string]any{
				"exit_code": -1,
				"output":    string(output),
				"timeout":   true,
			}, nil
		} else {
			return nil, err
		}
	}

	return map[string]any{
		"exit_code": exitCode,
		"output":    string(output),
		"timeout":   false,
	}, nil
}

func StringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return strings.TrimSpace(v)
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func AbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
