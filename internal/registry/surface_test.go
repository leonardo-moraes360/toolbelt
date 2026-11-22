package registry

import (
	"slices"
	"strings"
	"testing"
	"unicode"
)

func TestEveryToolHasATier(t *testing.T) {
	for _, tool := range Default().List() {
		switch tool.Tier {
		case TierStable:
			if strings.HasPrefix(tool.Description, "[experimental]") {
				t.Errorf("%s is stable but its description says experimental", tool.Name)
			}
		case TierExperimental:
			if !strings.HasPrefix(tool.Description, "[experimental] ") {
				t.Errorf("%s is experimental but its description does not start with [experimental]: %q", tool.Name, tool.Description)
			}
		default:
			t.Errorf("%s has no stability tier: add it to stableToolPrefixes or experimentalToolPrefixes", tool.Name)
		}
	}
}

func TestRemovedDebugToolsStayRemoved(t *testing.T) {
	for _, name := range []string{"tb_echo", "tb_env"} {
		if _, ok := Default().Get(name); ok {
			t.Errorf("%s was removed before 1.0 (O2)", name)
		}
	}
	tool, _ := Default().Get("tb_guard_check")
	if _, ok := tool.InputSchema.Properties["strict"]; ok {
		t.Error("tb_guard_check no longer takes strict (O1)")
	}
}

func TestToolDescriptionsAreEnglish(t *testing.T) {
	check := func(where, text string) {
		for _, r := range text {
			if r > unicode.MaxASCII && unicode.IsLetter(r) {
				t.Errorf("%s has a non-ASCII letter %q: tool text is English only: %q", where, r, text)
				return
			}
		}
	}
	for _, tool := range Default().List() {
		check(tool.Name+" description", tool.Description)
		if tool.InputSchema == nil {
			continue
		}
		for name, p := range tool.InputSchema.Properties {
			check(tool.Name+"."+name, p.Description)
		}
	}
}

func TestVaultAndEgmToolsDefaultCwd(t *testing.T) {
	for _, tool := range Default().List() {
		if tool.InputSchema == nil || !(strings.HasPrefix(tool.Name, "tb_vault_") || strings.HasPrefix(tool.Name, "tb_egm_")) {
			continue
		}
		if slices.Contains(tool.InputSchema.Required, "cwd") {
			t.Errorf("%s requires cwd; it defaults to the server's working directory", tool.Name)
		}
	}
}

func TestAbsToolCWD(t *testing.T) {
	if got, err := absToolCWD(map[string]any{}); err != nil || got == "" {
		t.Fatalf("default cwd = %q, %v", got, err)
	}
	if _, err := absToolCWD(map[string]any{"cwd": "rel"}); err == nil {
		t.Fatal("relative cwd accepted")
	}
}
