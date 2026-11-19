// Package egm is the native Elephant-Goldfish Model (docs/specs/egm-nativo.md):
// design documents reviewed by isolated reviewers until the review converges.
package egm

import "fmt"

// Exit codes (R32).
const (
	ExitOK         = 0
	ExitFail       = 1
	ExitError      = 2
	ExitNotPassing = 3
)

// Error carries the exit code and the machine reason (R32, R4c, R4d, R21).
type Error struct {
	Code   int
	Reason string
	Msg    string
}

func (e *Error) Error() string {
	if e.Reason == "" {
		return e.Msg
	}
	return fmt.Sprintf("%s: %s", e.Reason, e.Msg)
}

func errorf(code int, reason, format string, a ...any) *Error {
	return &Error{Code: code, Reason: reason, Msg: fmt.Sprintf(format, a...)}
}

// Warning is an entry of report.json "warnings" (R33b).
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Path    string `json:"path,omitempty"`
}
