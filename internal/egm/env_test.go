package egm

import (
	"strings"
	"testing"
)

func TestEnvPassRefusals(t *testing.T) {
	bad := []string{
		"NODE_OPTIONS", "PYTHONPATH", "PYTHONSTARTUP", "LD_PRELOAD", "DYLD_INSERT_LIBRARIES",
		"RUBYOPT", "PERL5OPT", "GIT_DIR", "BASH_ENV", "ENV", "PROMPT_COMMAND",
		"HOME", "PATH", "TB_EGM_OWNER", "TB_SANDBOX_LANDLOCK_PAYLOAD", "XDG_CONFIG_HOME", "", "A=B",
	}
	for _, n := range bad {
		wantReason(t, CheckEnvPass([]string{"ANTHROPIC_API_KEY", n}), "env_forbidden")
	}
	if err := CheckEnvPass([]string{"ANTHROPIC_API_KEY", "ENVIRONMENT", "HTTPS_PROXY"}); err != nil {
		t.Fatalf("allowed names refused: %v", err)
	}
}

func TestEnvBuiltFromScratch(t *testing.T) {
	t.Setenv("NODE_OPTIONS", "--require=/tmp/evil.js")
	t.Setenv("EGM_TEST_PASS", "kept")
	env := goldfishEnv("egm:s:r1:design", 2, []string{"EGM_TEST_PASS", "EGM_TEST_UNSET"}, "CLAUDE_CONFIG_DIR=/h/.claude")
	want := []string{
		"LANG=C.UTF-8", "TB_EGM_OWNER=egm:s:r1:design", "TB_EGM_ATTEMPT=2",
		"CLAUDE_CONFIG_DIR=/h/.claude", "EGM_TEST_PASS=kept",
	}
	if strings.Join(env, "\n") != strings.Join(want, "\n") {
		t.Fatalf("env = %q", env)
	}
}
