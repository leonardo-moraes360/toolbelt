package registry

import (
	"fmt"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
)

// passEnvArgDescription is the schema text shared by every tool that accepts
// pass_env.
const passEnvArgDescription = "Secrets (*TOKEN*/SECRET/KEY/PASSWORD) to pass to the child. " +
	"Only keys already listed in the config's env.pass are accepted; any other key is rejected"

// toolPassEnv returns the pass_env keys requested by an MCP call, refusing
// any key the human has not already opted in through trusted config
// (env.pass in the global or -c config; the repository's own config is
// writable by the agent, so it does not count).
//
// On the CLI, --pass-env is typed by a human. Over MCP the argument comes from
// the model, and a prompt-injected agent could ask for GITHUB_TOKEN and print
// it through tb_run. So the tool argument can only narrow what config already
// allows, never widen it.
func toolPassEnv(args map[string]any) ([]string, error) {
	requested := StringSliceArg(args, "pass_env", nil)
	if len(requested) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{}
	if cfg, err := config.LoadTrusted(); err == nil {
		for _, k := range cfg.Env.Pass {
			allowed[strings.TrimSpace(k)] = struct{}{}
		}
	}
	var denied []string
	for _, k := range requested {
		if _, ok := allowed[k]; !ok {
			denied = append(denied, k)
		}
	}
	if len(denied) > 0 {
		return nil, fmt.Errorf("pass_env refused for %s: over MCP, secrets must already be listed in env.pass of the global toolbelt config (a human opt-in outside the repository), not only in the tool call",
			strings.Join(denied, ", "))
	}
	return requested, nil
}
