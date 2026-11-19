package egm

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Vendor profiles (R4, R5, R15, R15a).
const (
	VendorClaude = "claude"
	VendorCustom = "custom"

	OutputClaudeJSON = "claude-json"
	OutputRaw        = "raw"
)

// claudeFlags turn off every tool, MCP server, settings file and session
// persistence of `claude -p` (validated in R0); all content goes on stdin.
var claudeFlags = []string{
	"-p", "--output-format", "json", "--tools", "", "--strict-mcp-config",
	"--setting-sources", "", "--no-session-persistence",
}

// builtinForbidden are refused in any argv (R5), whatever the user config
// says: each one would hand the goldfish unrestricted tools.
var builtinForbidden = []string{"bypassPermissions", "dangerously-skip-permissions"}

// minTokenLife is how long the copied access token must still be valid: a
// copy in the disposable HOME cannot refresh it (dogfood #29).
const minTokenLife = 10 * time.Minute

// Profile is the resolved vendor recipe for one run.
type Profile struct {
	Vendor string
	// Argv is egm.agent_cmd plus the profile flags.
	Argv   []string
	Output string
	// ToolsCheck is false when tool use cannot be detected (tools_check: none,
	// vendor custom or a raw envelope): the caller adds a downgrade (R15a).
	ToolsCheck bool
}

// ResolveProfile validates R4/R4b/R5 and builds the profile.
func ResolveProfile(set *Settings) (*Profile, error) {
	cmd := set.AgentCmd
	if len(cmd) == 0 || cmd[0] == "" {
		return nil, errorf(ExitError, "usage", "egm.agent_cmd is empty: the llm engine needs an agent")
	}
	if !filepath.IsAbs(cmd[0]) {
		return nil, errorf(ExitError, "usage", "egm.agent_cmd[0] must be an absolute path, got %q", cmd[0])
	}
	p, err := vendorProfile(set)
	if err != nil {
		return nil, err
	}
	if p.Output != OutputClaudeJSON && p.Output != OutputRaw {
		return nil, errorf(ExitError, "usage", "egm.output must be claude-json or raw, got %q", p.Output)
	}
	if err := checkForbiddenFlags(p.Argv, set.ForbiddenFlags); err != nil {
		return nil, err
	}
	if err := CheckEnvPass(set.Env.Pass); err != nil {
		return nil, err
	}
	// Phase A only knows how to detect tools in the claude envelope.
	p.ToolsCheck = set.ToolsCheck != "none" && p.Vendor == VendorClaude && p.Output == OutputClaudeJSON
	return p, nil
}

// vendorProfile fills argv and the default envelope of the vendor.
func vendorProfile(set *Settings) (*Profile, error) {
	p := &Profile{Vendor: set.Vendor, Output: set.Output, Argv: append([]string(nil), set.AgentCmd...)}
	defaultOutput := OutputRaw
	switch p.Vendor {
	case VendorClaude:
		p.Argv = append(p.Argv, claudeFlags...)
		defaultOutput = OutputClaudeJSON
	case VendorCustom:
		if !set.ToolsDisabled {
			return nil, errorf(ExitError, "usage", "vendor custom requires egm.tools_disabled: true (R5)")
		}
	default:
		return nil, errorf(ExitError, "usage", "egm.vendor must be claude or custom, got %q", p.Vendor)
	}
	if p.Output == "" {
		p.Output = defaultOutput
	}
	return p, nil
}

// checkForbiddenFlags refuses argv entries matching R5's built-in list or
// egm.forbidden_flags. Substring matching also catches "--flag=value" and
// "--permission-mode bypassPermissions" spellings.
func checkForbiddenFlags(argv, extra []string) error {
	for _, arg := range argv {
		for _, bad := range builtinForbidden {
			if strings.Contains(arg, bad) {
				return errorf(ExitError, "forbidden_flag", "agent argv contains forbidden flag %q", arg)
			}
		}
		for _, bad := range extra {
			if bad != "" && (arg == bad || strings.HasPrefix(arg, bad+"=")) {
				return errorf(ExitError, "forbidden_flag", "agent argv contains %q, listed in egm.forbidden_flags", arg)
			}
		}
	}
	return nil
}

// ArgvSHA is the hex sha256 of the argv joined by NUL, so the report can
// tell runs made with different agent commands apart.
func ArgvSHA(p *Profile) string {
	return sha256Hex([]byte(strings.Join(p.Argv, "\x00")))
}

// claudeEnvelope is the subset of `claude -p --output-format json` that tb
// reads (R15, R15a).
type claudeEnvelope struct {
	Type              string            `json:"type"`
	IsError           bool              `json:"is_error"`
	Result            *string           `json:"result"`
	NumTurns          int               `json:"num_turns"`
	PermissionDenials []json.RawMessage `json:"permission_denials"`
	Messages          []json.RawMessage `json:"messages"`
}

// errNoPayload means the envelope parsed but carried no reviewer text.
var errNoPayload = errors.New("envelope without result")

// extractPayload strips the profile envelope from stdout. It returns the
// reviewer text and whether the envelope shows tool use.
func extractPayload(output string, stdout []byte) (payload []byte, toolsUsed bool, err error) {
	if output == OutputRaw {
		return stdout, false, nil
	}
	var env claudeEnvelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		return nil, false, err
	}
	toolsUsed = env.NumTurns > 1 || len(env.PermissionDenials) > 0 || hasToolUse(env.Messages)
	if env.Result == nil || env.IsError {
		return nil, toolsUsed, errNoPayload
	}
	return []byte(*env.Result), toolsUsed, nil
}

// hasToolUse looks for a "tool_use" content block in any message the
// envelope carries (older/verbose envelopes include them).
func hasToolUse(msgs []json.RawMessage) bool {
	for _, m := range msgs {
		if strings.Contains(string(m), `"tool_use"`) {
			return true
		}
	}
	return false
}

// claudeCredential reads the claudeAiOauth entry of a credentials file and
// its expiry. Values are never logged.
func claudeCredential(path string) (oauth json.RawMessage, expires time.Time, err error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the user's own credential file
	if err != nil {
		return nil, time.Time{}, err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, time.Time{}, errors.New("credentials file is not valid JSON")
	}
	oauth, ok := all["claudeAiOauth"]
	if !ok {
		return nil, time.Time{}, errors.New("credentials file has no claudeAiOauth entry")
	}
	var meta struct {
		ExpiresAt int64 `json:"expiresAt"`
	}
	if err := json.Unmarshal(oauth, &meta); err != nil || meta.ExpiresAt == 0 {
		return nil, time.Time{}, errors.New("credentials file has no claudeAiOauth.expiresAt")
	}
	return oauth, time.UnixMilli(meta.ExpiresAt), nil
}

// checkClaudeAuth refuses to start when the access token expires within
// minTokenLife: every goldfish would fail on auth halfway through the round.
func checkClaudeAuth(credPath string) error {
	_, exp, err := claudeCredential(credPath)
	if err != nil {
		return errorf(ExitError, "auth_expired", "claude credential unusable (%v): run `claude` outside the sandbox to log in", err)
	}
	if left := time.Until(exp); left < minTokenLife {
		return errorf(ExitError, "auth_expired", "claude access token expires in %s (%s): run `claude` outside the sandbox to refresh it",
			left.Round(time.Minute), exp.Format(time.RFC3339))
	}
	return nil
}

// installClaudeCredential copies only the claudeAiOauth key into the
// disposable HOME (0600), so no other account data reaches the goldfish.
func installClaudeCredential(credPath, home string) error {
	oauth, _, err := claudeCredential(credPath)
	if err != nil {
		return err
	}
	out, err := json.Marshal(map[string]json.RawMessage{"claudeAiOauth": oauth})
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".credentials.json"), out, 0o600)
}
