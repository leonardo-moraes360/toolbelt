package busrun

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/execenv"
	"github.com/leonardo-moraes360/toolbelt/internal/mem"
)

const redacted = "[REDACTED]"

// minSecretLen: values (and value words) from the env or secret_files shorter
// than this are not redacted, to avoid erasing common words. pass_env uses
// minPassEnvLen since the human declared the secret there.
const (
	minSecretLen  = 6
	minPassEnvLen = 4
)

// Value words that are a scheme/header, not a secret.
var notSecretWords = map[string]bool{
	"authorization": true, "authorization:": true, "bearer": true, "basic": true, "token": true,
}

type secretPattern struct {
	re   *regexp.Regexp
	repl string
}

// The harness cannot know where the child keeps secrets, so besides known
// values the child's output goes through generic patterns before the audit.
var genericSecretPatterns = []secretPattern{
	// cookies: the whole value up to end of line
	{regexp.MustCompile(`(?im)((?:^|[\s"'])(?:set-)?cookie"?[ \t]*:[ \t]*)[^\r\n]+`), "${1}" + redacted},
	// credential header in header form (line start, quote or -H): optional
	// scheme + value
	{regexp.MustCompile(`(?im)((?:^|[\s"'>]|-H[ \t]+["']?)[ \t]*(?:proxy-)?(?:authorization|x-api-key|api[-_]?key)"?[ \t]*:[ \t]*"?)(?:(?:bearer|basic|token)[ \t]+)?[^\s"',;]+`), "${1}" + redacted},
	// JWT (dot-separated segments escape the long-token rule)
	{regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]*`), redacted},
	// secret name followed by = or : (JSON keys included), optional scheme
	// (SECRET_KEY, AWS_ACCESS_KEY_ID, "api_key":"x"); digit-only values are
	// kept (counters like max_tokens=4096)
	{regexp.MustCompile(`(?i)\b((?:[A-Za-z0-9_\-]*(?:token|secret|passw(?:or)?d|pwd|credential)|[A-Za-z0-9_\-]*[_\-]key|key)[A-Za-z0-9_\-]*"?[ \t]*[:=][ \t]*)(?:(?:bearer|basic|token)[ \t]+)?("[^"]*"|'[^']*'|[^\s,;&"'}]*[^\d\s,;&"'}][^\s,;&"'}]*)`), "${1}" + redacted},
	// credentials in URLs: userinfo and query string
	{regexp.MustCompile(`(://)[^/\s:@]+:[^/\s]*@`), "${1}" + redacted + "@"}, // up to the last @ (passwords containing @)
	{regexp.MustCompile(`(?i)([?&](?:token|key|sig|signature|secret|access_token|auth|code)=)[^&\s"']+`), "${1}" + redacted},
}

// Long letter+digit runs look like tokens (base64 with `/` included). Kept:
// UUIDs (session/run ids) and absolute paths.
var (
	longTokenRe = regexp.MustCompile(`[A-Za-z0-9_\-+/=]{32,}`)
	uuidRe      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	hasLetterRe = regexp.MustCompile(`[A-Za-z]`)
	hasDigitRe  = regexp.MustCompile(`\d`)
)

// redactor scrubs text bound for the audit (child output, errors). Values are
// read at Open: a rotated secret requires restarting the harness.
type redactor struct {
	known *strings.Replacer
}

// newRedactor gathers known values: pass_env, secret variables of the
// harness process and values from the config's secret_files.
func newRedactor(cfg *Config) (*redactor, error) {
	set := map[string]bool{}
	add := func(v string, minLen int) {
		for _, c := range secretCandidates(v, minLen) {
			set[c] = true
		}
	}
	for _, name := range cfg.PassEnv {
		add(os.Getenv(strings.TrimSpace(name)), minPassEnvLen)
	}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok && execenv.IsSecret(k) {
			add(v, minSecretLen)
		}
	}
	for _, p := range cfg.SecretFiles {
		vals, err := readSecretFile(p)
		if err != nil {
			return nil, err
		}
		for _, v := range vals {
			add(v, minSecretLen)
		}
	}
	vals := make([]string, 0, len(set))
	for v := range set {
		vals = append(vals, v)
	}
	// Replacer matches in argument order: longest first.
	sort.Slice(vals, func(i, j int) bool {
		if len(vals[i]) != len(vals[j]) {
			return len(vals[i]) > len(vals[j])
		}
		return vals[i] < vals[j]
	})
	pairs := make([]string, 0, 2*len(vals))
	for _, v := range vals {
		pairs = append(pairs, v, redacted)
	}
	return &redactor{known: strings.NewReplacer(pairs...)}, nil
}

// secretCandidates returns the whole value, each suffix after a space
// ("Authorization: Bearer X" also redacts "Bearer X") and each word as sh would
// split it (space/tab/newline, unquoted): an env sourced as shell prints loose
// value words in errors.
func secretCandidates(v string, minLen int) []string {
	v = strings.TrimSpace(v)
	var out []string
	if len(v) >= minLen {
		out = append(out, v) // always the whole value, even if it is "token"
	}
	keep := func(c string) {
		if len(c) >= minLen && !notSecretWords[strings.ToLower(c)] {
			out = append(out, c)
		}
	}
	fields := strings.Fields(v)
	for i := range fields {
		if i > 0 {
			keep(strings.Join(fields[i:], " "))
		}
		keep(strings.Trim(fields[i], `"'`))
	}
	return out
}

// readSecretFile parses KEY=VALUE (tolerating comments, blank lines, `export`,
// quotes and trailing comments) without executing anything. Requires a regular
// file at 0600 or stricter, like the wake file. Missing or unreadable is an
// error (fail-closed: without the values, the audit would leak).
func readSecretFile(path string) ([]string, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("secret_files: path %q must be absolute", path)
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("secret_files: %w", err)
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("secret_files: %s is not a regular file", path)
	}
	if st.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("secret_files: %s has permission %o: use chmod 600", path, st.Mode().Perm())
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("secret_files: %w", err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		_, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out = append(out, envValue(strings.TrimSpace(v)))
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("secret_files %s: %w", path, err)
	}
	return out, nil
}

// envValue strips outer quotes or, if unquoted, a trailing " #..." comment.
func envValue(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	if i := strings.Index(v, "\t#"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return v
}

// redact applies, in order: known values, mem patterns, generic patterns, and
// finally masks token-like runs.
func (r *redactor) redact(s string) string {
	if s == "" {
		return s
	}
	if r != nil && r.known != nil {
		s = r.known.Replace(s)
	}
	s = mem.RedactText(s)
	for _, p := range genericSecretPatterns {
		s = p.re.ReplaceAllString(s, p.repl)
	}
	return longTokenRe.ReplaceAllStringFunc(s, func(m string) string {
		if uuidRe.MatchString(m) || strings.HasPrefix(m, "/") || !hasLetterRe.MatchString(m) || !hasDigitRe.MatchString(m) {
			return m
		}
		return redacted
	})
}
