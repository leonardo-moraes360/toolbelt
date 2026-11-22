package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/envelope"
	"github.com/leonardo-moraes360/toolbelt/internal/verify"
)

// Exit codes (docs/cli.md, "Exit Codes").
const (
	exitOK        = 0
	exitFail      = 1
	exitUsage     = 2
	exitTimeout   = 124
	exitNotFound  = 127
	exitInterrupt = 130
)

// runStarted flips once cobra has parsed the flags and validated the
// arguments. An error before that point is a usage error (exit 2); after it,
// the command was already running. Every PersistentPreRunE calls markRunStarted.
var runStarted bool

func markRunStarted() { runStarted = true }

// stdoutDocument is set once a JSON document went to stdout, so a later error
// does not add a second one: stdout stays a single document.
var stdoutDocument bool

func printStdout(b []byte) error {
	stdoutDocument = true
	_, err := fmt.Fprintln(os.Stdout, string(b))
	return err
}

// stampedJSON is the document printed for kind: the payload with "v" first.
func stampedJSON(kind string, v any, indent bool) ([]byte, error) {
	raw, err := envelope.Stamp(kind, v)
	if err != nil {
		return nil, err
	}
	if !indent {
		return raw, nil
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// emitJSON prints the payload as a compact one-line document.
func emitJSON(kind string, v any) error {
	b, err := stampedJSON(kind, v, false)
	if err != nil {
		return err
	}
	return printStdout(b)
}

// emitJSONIndent prints the payload indented.
func emitJSONIndent(kind string, v any) error {
	b, err := stampedJSON(kind, v, true)
	if err != nil {
		return err
	}
	return printStdout(b)
}

// emitJSONOrFile writes the indented document to outPath when set, otherwise
// to stdout.
func emitJSONOrFile(kind string, v any, outPath string) error {
	b, err := stampedJSON(kind, v, true)
	if err != nil {
		return err
	}
	if outPath != "" {
		return os.WriteFile(outPath, b, 0o600)
	}
	return printStdout(b)
}

// exitCodeFor maps the error returned by cobra to the stable exit code: a
// usage error (bad flag or argument, unknown subcommand, invalid ruler) is 2,
// a command not found is 127, anything else 1.
func exitCodeFor(err error) int {
	var coded *exitCodeError
	if errors.As(err, &coded) {
		return coded.code
	}
	switch {
	case verify.IsUsageError(err):
		return exitUsage
	case !runStarted || isRequiredFlagError(err):
		return exitUsage
	}
	return exitFail
}

func isRequiredFlagError(err error) bool {
	msg := err.Error()
	return strings.HasPrefix(msg, `required flag(s) "`) ||
		strings.HasPrefix(msg, "if any flags in the group [") ||
		strings.HasPrefix(msg, "at least one of the flags in the group [")
}

// errorReasonFor is the reason field of the error/1.0 body.
func errorReasonFor(err error, code int) string {
	switch code {
	case exitUsage:
		return "usage"
	case exitNotFound:
		return "command_not_found"
	}
	return envelope.ReasonOf(err)
}

// wantsJSON reports --json before cobra parsed it (a flag error aborts the
// parse), so the error body still goes out as JSON.
func wantsJSON(args []string) bool {
	if runStarted {
		return jsonOutput
	}
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "--json" || a == "--json=true" {
			return true
		}
	}
	return false
}

// reportError prints the error/1.0 body on stdout under --json, unless a
// result document already went there; the text message always stays on stderr.
func reportError(err error, code int, args []string) {
	if stdoutDocument {
		return
	}
	if wantsJSON(args) {
		_ = emitJSON(envelope.Error, envelope.NewError(err.Error(), errorReasonFor(err, code), code))
	}
}

// printErrorBody is the error path of commands with their own reasons and
// exit codes (egm, vault).
func printErrorBody(err error, reason string, code int) {
	b, _ := json.Marshal(envelope.NewError(err.Error(), reason, code))
	_ = printStdout(b)
}

// exitCodeError carries a specific exit code out of RunE (127 for a command
// that does not exist).
type exitCodeError struct {
	code int
	err  error
}

func (e *exitCodeError) Error() string { return e.err.Error() }
func (e *exitCodeError) Unwrap() error { return e.err }
