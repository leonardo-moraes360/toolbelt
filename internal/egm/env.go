package egm

import (
	"os"
	"strconv"
	"strings"
)

// Goldfish env variables set by tb (R3, R4b).
const (
	envOwner   = "TB_EGM_OWNER"
	envAttempt = "TB_EGM_ATTEMPT"
)

// forbiddenEnvPrefixes load code into an interpreter or redirect git before
// the agent runs a single instruction (R4b).
var forbiddenEnvPrefixes = []string{"NODE_", "PYTHON", "LD_", "DYLD_", "RUBY", "PERL5", "GIT_"}

// forbiddenEnvNames are shell startup hooks (R4b).
var forbiddenEnvNames = []string{"BASH_ENV", "ENV", "PROMPT_COMMAND"}

// reservedEnvNames are set by tb or the sandbox; passing them through would
// let the user config forge the owner attribution or the isolation dirs.
var reservedEnvNames = []string{"HOME", "PATH", "TMPDIR", "LANG"}

// reservedEnvPrefixes: TB_ covers TB_EGM_* and the sandbox trampoline's
// TB_SANDBOX_* payload; XDG_ is derived from HOME.
var reservedEnvPrefixes = []string{"TB_", "XDG_"}

// CheckEnvPass is R4b's refusal list for egm.env.pass, plus the names tb
// sets itself.
func CheckEnvPass(names []string) error {
	for _, n := range names {
		if n == "" || strings.ContainsAny(n, "=\x00") {
			return errorf(ExitError, "env_forbidden", "egm.env.pass has an invalid name %q", n)
		}
		if envForbidden(n) {
			return errorf(ExitError, "env_forbidden", "egm.env.pass cannot forward %s (R4b)", n)
		}
	}
	return nil
}

func envForbidden(name string) bool {
	if containsStr(forbiddenEnvNames, name) || containsStr(reservedEnvNames, name) {
		return true
	}
	for _, p := range forbiddenEnvPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	for _, p := range reservedEnvPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// goldfishEnv builds the child env from scratch, never from os.Environ()
// (R4b). HOME, TMPDIR, XDG_* and PATH are added by the sandbox from its
// Config fields; extra holds vendor variables such as CLAUDE_CONFIG_DIR.
// Unset names in pass are skipped, so an empty variable is not invented.
func goldfishEnv(owner string, attempt int, pass []string, extra ...string) []string {
	env := []string{
		"LANG=C.UTF-8",
		envOwner + "=" + owner,
		envAttempt + "=" + strconv.Itoa(attempt),
	}
	env = append(env, extra...)
	for _, name := range pass {
		if v, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+v)
		}
	}
	return env
}
