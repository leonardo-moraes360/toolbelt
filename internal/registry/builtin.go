package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
	"github.com/leonardo-moraes360/toolbelt/internal/execenv"
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
		Description: "Executa processo no shell com timeout opcional (env filtrado por allowlist)",
		InputSchema: &InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"command": {Type: "string", Description: "Processo a executar"},
				"timeout": {Type: "string", Description: "Timeout (ex: 30s)", Default: "30s"},
				"cwd":     {Type: "string", Description: "Diretório de trabalho"},
				"env": {
					Type:        "array",
					Description: "Chaves extras na allowlist de ambiente",
					Items:       &Property{Type: "string"},
				},
				"pass_env": {
					Type:        "array",
					Description: "Opt-in explícito para segredos (*TOKEN*/SECRET/KEY/PASSWORD)",
					Items:       &Property{Type: "string"},
				},
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

	cmd := exec.CommandContext(ctx, "sh", "-c", command) //nolint:gosec // explicit MCP tool
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Env = execenv.Environ(resolveExecEnv(args))

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

func resolveExecEnv(args map[string]any) execenv.Options {
	opts := execenv.Options{
		Allow: StringSliceArg(args, "env", nil),
		Pass:  StringSliceArg(args, "pass_env", nil),
	}
	if fileCfg, err := config.Load(); err == nil {
		opts = execenv.Merge(execenv.Options{Allow: fileCfg.Env.Allow, Pass: fileCfg.Env.Pass}, opts.Allow, opts.Pass)
	}
	return opts
}

func StringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return strings.TrimSpace(v)
}

func HasArg(args map[string]any, key string) bool {
	_, ok := args[key]
	return ok
}

func IntArg(args map[string]any, key string, def int) int {
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return def
		}
		return int(i)
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return def
		}
		var i int
		if _, err := fmt.Sscanf(s, "%d", &i); err != nil {
			return def
		}
		return i
	default:
		return def
	}
}

func BoolArg(args map[string]any, key string, def bool) bool {
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		s := strings.TrimSpace(strings.ToLower(b))
		switch s {
		case "":
			return def
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		default:
			return def
		}
	case float64:
		return b != 0
	case int:
		return b != 0
	default:
		return def
	}
}

func FloatArg(args map[string]any, key string, def float64) float64 {
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return def
		}
		return f
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return def
		}
		var f float64
		if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
			return def
		}
		return f
	default:
		return def
	}
}

func StringSliceArg(args map[string]any, key string, def []string) []string {
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	switch s := v.(type) {
	case []string:
		out := make([]string, 0, len(s))
		for _, item := range s {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		if len(out) == 0 {
			return def
		}
		return out
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			str := strings.TrimSpace(fmt.Sprint(item))
			if str != "" {
				out = append(out, str)
			}
		}
		if len(out) == 0 {
			return def
		}
		return out
	case string:
		parts := splitCSVArg(s)
		if len(parts) == 0 {
			return def
		}
		return parts
	default:
		return def
	}
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
