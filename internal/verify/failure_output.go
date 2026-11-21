package verify

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	ansiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
	// failLineRE matches the lines a test runner uses to name what broke
	// (go test, vitest/jest, pytest, playwright, cargo, tsc, generic errors).
	// Case-sensitive and anchored at line start, so ordinary log noise
	// ("expected config to be set") does not enter the signature.
	failLineRE = regexp.MustCompile(`^(--- FAIL|FAIL\b|FAILED\b|✗|✘|×|\d+\) |error(\[\w+\])?: |Error: |\w*Error: |panic: |thread '.*' panicked)`)
	// volatileRE removes durations, timestamps, hex addresses and ports so that
	// two runs of the same deterministic failure produce the same signature.
	volatileRE = regexp.MustCompile(`\d+(\.\d+)?\s?(ms|s|m|µs|ns)\b|\(\d+(\.\d+)?s\)|0x[0-9a-fA-F]+|\d{4}-\d{2}-\d{2}[T ][\d:.]+Z?|:\d{2,5}\b`)
)

// withNoColor asks child processes for plain output unless the operator
// already chose a color mode; agents read the reason, not a terminal.
func withNoColor(env []string) []string {
	has := func(name string) bool {
		for _, e := range env {
			if strings.HasPrefix(e, name+"=") {
				return true
			}
		}
		return false
	}
	out := append([]string(nil), env...)
	if !has("NO_COLOR") && !has("FORCE_COLOR") {
		out = append(out, "NO_COLOR=1", "FORCE_COLOR=0")
	}
	return out
}

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func failureLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(line)
		if trim != "" && failLineRE.MatchString(trim) {
			lines = append(lines, trim)
		}
	}
	return lines
}

// excerpt keeps the failure lines first and then the tail of the output
// (runners print the summary last), within max bytes.
func excerpt(out string, max int) string {
	if len(out) <= max {
		return out
	}
	var b strings.Builder
	seen := map[string]struct{}{}
	for _, l := range failureLines(out) {
		if _, dup := seen[l]; dup {
			continue
		}
		seen[l] = struct{}{}
		if b.Len()+len(l)+1 > max/2 {
			break
		}
		b.WriteString(l)
		b.WriteByte('\n')
	}
	rest := max - b.Len()
	if b.Len() > 0 {
		b.WriteString("…\n")
	}
	start := len(out) - rest
	for start < len(out) && !utf8.RuneStart(out[start]) {
		start++
	}
	tail := out[start:]
	if i := strings.IndexByte(tail, '\n'); i >= 0 && i < len(tail)-1 {
		tail = tail[i+1:]
	}
	b.WriteString(tail)
	return b.String()
}

// failureSignature normalizes the failure lines of a run. A run without
// recognizable failure lines (silent exit 1, crash) yields "" and keeps being
// retried: only a named, repeated failure counts as deterministic.
func failureSignature(output string) string {
	lines := failureLines(stripANSI(output))
	if len(lines) == 0 {
		return ""
	}
	norm := make([]string, 0, len(lines))
	for _, l := range lines {
		norm = append(norm, volatileRE.ReplaceAllString(l, "#"))
	}
	sort.Strings(norm)
	return strings.Join(norm, "\n")
}
