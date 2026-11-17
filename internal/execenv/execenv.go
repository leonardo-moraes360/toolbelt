package execenv

import (
	"os"
	"strings"
)

var secretSubstrings = []string{"TOKEN", "SECRET", "KEY", "PASSWORD"}

var defaultAllowExact = map[string]struct{}{
	"PATH": {}, "HOME": {}, "USER": {}, "LOGNAME": {}, "SHELL": {}, "PWD": {}, "OLDPWD": {},
	"LANG": {}, "LANGUAGE": {}, "TZ": {}, "TERM": {}, "TERMINFO": {}, "COLORTERM": {}, "NO_COLOR": {},
	"TMPDIR": {}, "TMP": {}, "TEMP": {},
	"GOPATH": {}, "GOROOT": {}, "GOCACHE": {}, "GOMODCACHE": {}, "GO111MODULE": {},
	"GOTOOLCHAIN": {}, "GOFLAGS": {}, "GOPROXY": {}, "GOSUMDB": {}, "GOINSECURE": {},
	"GOOS": {}, "GOARCH": {}, "CGO_ENABLED": {}, "CGO_CFLAGS": {}, "CGO_LDFLAGS": {},
	"NODE_PATH": {}, "NODE_OPTIONS": {}, "NODE_ENV": {}, "NODE_EXTRA_CA_CERTS": {},
	"NPM_CONFIG_CACHE": {}, "NPM_CONFIG_PREFIX": {}, "NPM_CONFIG_REGISTRY": {},
	"npm_config_cache": {}, "npm_config_prefix": {}, "npm_config_registry": {},
	"npm_config_user_agent": {}, "npm_node_execpath": {}, "npm_execpath": {},
	"PNPM_HOME": {}, "PNPM_STORE_PATH": {},
	"YARN_CACHE_FOLDER": {}, "BUN_INSTALL": {},
	"CI": {}, "CONTINUOUS_INTEGRATION": {},
	"GITHUB_ACTIONS": {}, "GITHUB_WORKSPACE": {}, "GITHUB_EVENT_NAME": {},
	"GITHUB_REPOSITORY": {}, "GITHUB_REF": {}, "GITHUB_SHA": {}, "GITHUB_RUN_ID": {},
	"GITHUB_JOB": {}, "GITHUB_PATH": {}, "GITHUB_ENV": {}, "GITHUB_STEP_SUMMARY": {},
	"RUNNER_OS": {}, "RUNNER_TEMP": {}, "RUNNER_TOOL_CACHE": {}, "RUNNER_WORKSPACE": {},
	"XDG_CACHE_HOME": {}, "XDG_CONFIG_HOME": {}, "XDG_DATA_HOME": {}, "XDG_RUNTIME_DIR": {},
	"SSL_CERT_FILE": {}, "SSL_CERT_DIR": {}, "REQUESTS_CA_BUNDLE": {}, "CURL_CA_BUNDLE": {},
	"GIT_SSL_CAINFO": {}, "GIT_CONFIG_COUNT": {}, "GIT_EXEC_PATH": {}, "GIT_TERMINAL_PROMPT": {},
	"HTTP_PROXY": {}, "HTTPS_PROXY": {}, "NO_PROXY": {}, "ALL_PROXY": {},
	"http_proxy": {}, "https_proxy": {}, "no_proxy": {}, "all_proxy": {},
	"JAVA_HOME": {}, "JDK_HOME": {},
	"CARGO_HOME": {}, "RUSTUP_HOME": {}, "CARGO_TARGET_DIR": {},
	"VIRTUAL_ENV": {}, "PYTHONPATH": {}, "PYTHONHOME": {}, "PYTHONUSERBASE": {},
	"PIP_CACHE_DIR": {}, "UV_CACHE_DIR": {}, "UV_PYTHON": {},
	"HOMEBREW_PREFIX": {}, "HOMEBREW_CELLAR": {}, "HOMEBREW_REPOSITORY": {},
	"TB_HOME": {}, "TB_GRAPH": {}, "TB_BROWSER": {}, "TB_COLOR": {}, "TB_COLOR_MODE": {},
	"TB_BENCH_PROMPT": {}, "TB_BENCH_WORKDIR": {}, "TB_BENCH_TIMEOUT": {},
}

var defaultAllowPrefixes = []string{
	"LC_",
	"npm_config_",
	"npm_package_",
	"npm_lifecycle_",
	"NPM_CONFIG_",
}

type Options struct {
	Allow []string
	Pass  []string
}

func IsSecret(name string) bool {
	upper := strings.ToUpper(name)
	for _, sub := range secretSubstrings {
		if strings.Contains(upper, sub) {
			return true
		}
	}
	return false
}

func Filter(environ []string, opts Options) []string {
	allow := map[string]struct{}{}
	for k := range defaultAllowExact {
		allow[k] = struct{}{}
	}
	for _, k := range opts.Allow {
		k = strings.TrimSpace(k)
		if k != "" {
			allow[k] = struct{}{}
		}
	}
	pass := map[string]struct{}{}
	for _, k := range opts.Pass {
		k = strings.TrimSpace(k)
		if k != "" {
			pass[k] = struct{}{}
		}
	}

	out := make([]string, 0, len(environ))
	seen := map[string]struct{}{}
	for _, entry := range environ {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		if !allowed(key, allow, pass) {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, entry)
	}
	return out
}

func Environ(opts Options) []string {
	return Filter(os.Environ(), opts)
}

func Merge(base Options, extraAllow, extraPass []string) Options {
	return Options{
		Allow: append(append([]string{}, base.Allow...), extraAllow...),
		Pass:  append(append([]string{}, base.Pass...), extraPass...),
	}
}

func allowed(key string, allow, pass map[string]struct{}) bool {
	if _, ok := pass[key]; ok {
		return true
	}
	if IsSecret(key) {
		return false
	}
	if _, ok := allow[key]; ok {
		return true
	}
	for _, prefix := range defaultAllowPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}
