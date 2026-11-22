package merge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/leonardo-moraes360/toolbelt/internal/version"
)

// EnvStateDir overrides the receipt state directory (tests).
const EnvStateDir = "TB_MERGE_STATE_DIR"

const (
	receiptVersion = 1
	receiptMaxAge  = 30 * 24 * time.Hour
)

var reReceiptID = regexp.MustCompile(`^[0-9a-f]{16}$`)

// Error is a merge failure with a machine reason (prompt_budget,
// receipt_stale, agent_unavailable...). Its text starts with the reason.
type Error struct {
	Reason string
	Msg    string
}

func (e *Error) Error() string { return e.Reason + ": " + e.Msg }

func reasonErr(reason, format string, a ...any) *Error {
	return &Error{Reason: reason, Msg: fmt.Sprintf(format, a...)}
}

// ReceiptRef points at the receipt a prompt proposal wrote.
type ReceiptRef struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// ReceiptTicket binds a receipt to the ticket heads it was built from.
type ReceiptTicket struct {
	ID   string `json:"id"`
	Base string `json:"base"`
	Head string `json:"head"`
}

// ReceiptAgent identifies the agent configuration that produced the answers.
// It never holds the environment or any env.pass value.
type ReceiptAgent struct {
	Vendor     string `json:"vendor"`
	Output     string `json:"output"`
	ArgvSHA256 string `json:"argv_sha256"`
	ToolsCheck bool   `json:"tools_check"`
	TB         string `json:"tb_version"`
}

// RunRecord is one goldfish attempt (no stderr, no environment).
type RunRecord struct {
	File          string `json:"file"`
	Attempt       int    `json:"attempt"`
	Reason        string `json:"reason,omitempty"`
	DurationMS    int64  `json:"duration_ms"`
	PromptSHA256  string `json:"prompt_sha256"`
	PayloadSHA256 string `json:"payload_sha256,omitempty"`
}

// ReceiptResolution is a Resolution plus the text it replays.
type ReceiptResolution struct {
	Resolution
	Text string `json:"text,omitempty"`
}

// ReceiptUnresolved is a file that stayed in conflict in the proposal.
type ReceiptUnresolved struct {
	File   string `json:"file"`
	Ticket string `json:"ticket"`
	Commit string `json:"commit"`
	Reason string `json:"reason,omitempty"`
	Hunks  []Hunk `json:"hunks,omitempty"`
}

// Receipt is the audit record of a prompt proposal. The apply step replays
// the agent resolutions it holds; nothing in it can move a ref by itself.
type Receipt struct {
	V           int                 `json:"v"`
	CreatedAt   string              `json:"created_at"`
	Target      string              `json:"target"`
	StartSHA    string              `json:"start_sha"`
	Strategy    string              `json:"strategy"`
	Tickets     []ReceiptTicket     `json:"tickets"`
	Agent       ReceiptAgent        `json:"agent"`
	Runs        []RunRecord         `json:"runs"`
	Resolutions []ReceiptResolution `json:"resolutions"`
	Unresolved  []ReceiptUnresolved `json:"unresolved"`
	ResultTree  string              `json:"result_tree"`
}

// receiptDir is $TB_MERGE_STATE_DIR (or $XDG_STATE_HOME/toolbelt/merge)
// plus a hash of the repository root: outside the repository and its git
// directory, so a worker or a ticket cannot plant a receipt.
func receiptDir(top string) (string, error) {
	base := os.Getenv(EnvStateDir)
	if base == "" {
		if x := os.Getenv("XDG_STATE_HOME"); x != "" && filepath.IsAbs(x) {
			base = filepath.Join(x, "toolbelt", "merge")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("merge receipt directory: %w", err)
			}
			base = filepath.Join(home, ".local", "state", "toolbelt", "merge")
		}
	}
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("%s must be absolute", EnvStateDir)
	}
	abs := top
	if resolved, err := filepath.EvalSymlinks(top); err == nil {
		abs = resolved
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Join(base, hex.EncodeToString(sum[:])[:16]), nil
}

// writeReceipt stores rc once (0600, temp plus rename) and sweeps old ones.
func writeReceipt(top string, rc *Receipt) (*ReceiptRef, error) {
	dir, err := receiptDir(top)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := secureStateDir(dir); err != nil {
		return nil, err
	}
	sweepReceipts(dir, time.Now())
	b, err := json.MarshalIndent(rc, "", "  ")
	if err != nil {
		return nil, err
	}
	b = append(b, '\n')
	sum := sha256.Sum256(b)
	id := hex.EncodeToString(sum[:])[:16]
	path := filepath.Join(dir, id+".json")
	tmp, err := os.CreateTemp(dir, ".receipt-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // no-op after the rename
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return nil, err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return nil, err
	}
	return &ReceiptRef{ID: id, Path: path}, nil
}

// sweepReceipts removes receipts older than receiptMaxAge.
func sweepReceipts(dir string, now time.Time) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		fi, err := e.Info()
		if err == nil && fi.Mode().IsRegular() && reReceiptID.MatchString(e.Name()[:min(16, len(e.Name()))]) && now.Sub(fi.ModTime()) > receiptMaxAge {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// loadReceipt reads receipt id and checks that its bytes still hash to the
// ID (receipt_tampered) and that it is applicable (receipt_incomplete).
func loadReceipt(top, id string) (*Receipt, error) {
	if !reReceiptID.MatchString(id) {
		return nil, fmt.Errorf("invalid receipt id %q (16 hex characters)", id)
	}
	dir, err := receiptDir(top)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(dir, id+".json")) //nolint:gosec // id is validated hex
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("receipt %s not found for this repository (run the prompt proposal first)", id)
	}
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:])[:16] != id {
		return nil, reasonErr("receipt_tampered", "receipt %s no longer matches its ID", id)
	}
	var rc Receipt
	if err := json.Unmarshal(b, &rc); err != nil || rc.V != receiptVersion {
		return nil, reasonErr("receipt_tampered", "receipt %s is not a version %d receipt", id, receiptVersion)
	}
	if len(rc.Unresolved) > 0 || rc.ResultTree == "" {
		return nil, reasonErr("receipt_incomplete", "receipt %s left %d file(s) unresolved; it cannot be applied", id, len(rc.Unresolved))
	}
	return &rc, nil
}

// checkStale compares the receipt with this run's target, start and tickets.
func (rc *Receipt) checkStale(target, startSHA string, prep []prepared) error {
	stale := func(what string) error {
		return reasonErr("receipt_stale", "%s changed since the proposal; rerun it with --strategy prompt", what)
	}
	if rc.Target != target || rc.StartSHA != startSHA {
		return stale("the target branch")
	}
	if len(rc.Tickets) != len(prep) {
		return stale("the ticket list")
	}
	for i := range prep {
		if rc.Tickets[i] != (ReceiptTicket{ID: prep[i].t.ID, Base: prep[i].base, Head: prep[i].head}) {
			return stale("ticket " + prep[i].t.ID)
		}
	}
	return nil
}

func newReceipt(target, startSHA string, prep []prepared, res *Result, ag *agentEnv, tree string) *Receipt {
	rc := &Receipt{
		V: receiptVersion, CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Target: target, StartSHA: startSHA, Strategy: string(StrategyPrompt),
		Runs: []RunRecord{}, Resolutions: []ReceiptResolution{}, Unresolved: []ReceiptUnresolved{}, ResultTree: tree,
	}
	for i := range prep {
		rc.Tickets = append(rc.Tickets, ReceiptTicket{ID: prep[i].t.ID, Base: prep[i].base, Head: prep[i].head})
	}
	if ag != nil {
		rc.Agent = ReceiptAgent{Vendor: ag.prof.Vendor, Output: ag.prof.Output, ArgvSHA256: ag.argvSHA, ToolsCheck: ag.prof.ToolsCheck, TB: version.Get().Version}
		rc.Runs = append(rc.Runs, ag.runs...)
	}
	for i := range res.Resolutions {
		rr := ReceiptResolution{Resolution: res.Resolutions[i]}
		if rr.Rule == ruleAgent {
			rr.Text = rr.Resolution.Text
		}
		rc.Resolutions = append(rc.Resolutions, rr)
	}
	for i := range res.Conflicts {
		if c := &res.Conflicts[i]; c.Resolution == "" {
			rc.Unresolved = append(rc.Unresolved, ReceiptUnresolved{File: c.File, Ticket: c.Ticket, Commit: c.Commit, Reason: c.Reason, Hunks: c.Hunks})
		}
	}
	return rc
}
