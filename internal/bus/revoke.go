package bus

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
)

// Bus operations for the router on the bus (docs/bus-router.md, B4a).

// ConfinementMarker is what ClaimConfined stores: the tb harness ran the
// task confined (router_tasks, Landlock). It states what that process
// enforced; it is not a credential.
const ConfinementMarker = `{"router_tasks":true,"landlock":true}`

// Revoke outcomes.
const (
	RevokeCompleted = "completed" // a result got there first; nothing changed
	RevokeRevoked   = "revoked"
)

// RevokeResult is the final state of the thread after Revoke.
type RevokeResult struct {
	State string `json:"state"`
	Claim Claim  `json:"claim"`
}

// selectMessageByID is a single literal (placeholders only, values go in
// args); TestSelectMessageByID keeps its columns in sync with selectMessage.
const selectMessageByID = `SELECT seq, id, channel, sender, recipient, thread, type, causation_id, idempotency_key, ttl, payload, created_at, parent FROM messages WHERE channel = ? AND id = ?`

var reasonRe = regexp.MustCompile(`^[a-z][a-z_]{0,31}$`)

// migrateV2 adds the revoke and confinement columns.
func migrateV2(tx *sql.Tx) error {
	for _, q := range []string{
		`ALTER TABLE claims ADD COLUMN revoked_at INTEGER`,
		`ALTER TABLE claims ADD COLUMN revoked_by TEXT`,
		`ALTER TABLE claims ADD COLUMN revoke_reason TEXT`,
		`ALTER TABLE claims ADD COLUMN confinement TEXT`,
	} {
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("migrating bus schema v2 -> v3: %w", err)
		}
	}
	return nil
}

// ClaimConfined claims like ClaimThread and stores ConfinementMarker. It is
// an internal API for the harness's router_tasks mode: no CLI flag or MCP
// tool reaches it.
func (s *Store) ClaimConfined(m Member, thread string) (Claim, error) {
	return s.claimThread(m, thread, ConfinementMarker)
}

// ErrNoClaim: the thread has no claim row.
var ErrNoClaim = errors.New("no claim")

// Claim reads one claim row. It is a read: it works on an expired channel.
func (s *Store) Claim(channel, thread string) (Claim, error) {
	c, err := scanClaim(s.db.QueryRow(selectClaim, channel, thread))
	if errors.Is(err, sql.ErrNoRows) {
		return Claim{}, fmt.Errorf("%w: thread %q", ErrNoClaim, thread)
	}
	return c, err
}

// Message reads one message by ID; it works on an expired channel.
func (s *Store) Message(channel, id string) (Message, error) {
	msg, err := scanMessage(s.db.QueryRow(selectMessageByID, channel, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, fmt.Errorf("message %q not found", id)
	}
	return msg, err
}

// Revoke closes a thread for good in one IMMEDIATE transaction and returns
// its final state: a completed thread is left as is (the result got there
// first); an open one is released and marked revoked; an unclaimed one gets
// a revoked row owned by the caller. Router and supervisor only.
func (s *Store) Revoke(m Member, thread, reason string) (RevokeResult, error) {
	if m.Role != RoleRouter && m.Role != RoleSupervisor {
		return RevokeResult{}, errors.New("only router or supervisor can revoke a thread")
	}
	if err := validRef("thread", thread); err != nil || thread == "" {
		return RevokeResult{}, fmt.Errorf("invalid thread %q", thread)
	}
	if !reasonRe.MatchString(reason) {
		return RevokeResult{}, fmt.Errorf("invalid revoke reason %q (use [a-z_])", reason)
	}
	var res RevokeResult
	err := s.tx(func(tx *sql.Tx) error {
		now := s.nowNS()
		if _, err := activeChannel(tx, m.Channel, now); err != nil {
			return err
		}
		if err := touch(tx, m, now); err != nil {
			return err
		}
		cur, err := scanClaim(tx.QueryRow(selectClaim, m.Channel, thread))
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if _, err := tx.Exec(`INSERT INTO claims(channel, thread, owner, claimed_at, epoch, released_at, revoked_at, revoked_by, revoke_reason)
				VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?)`, m.Channel, thread, m.ID(), now, now, now, m.ID(), reason); err != nil {
				return err
			}
		case err != nil:
			return err
		case cur.RevokedAt != nil:
			res = RevokeResult{State: RevokeRevoked, Claim: cur}
			return nil
		case cur.CompletedAt != nil:
			res = RevokeResult{State: RevokeCompleted, Claim: cur}
			return nil
		default:
			if _, err := tx.Exec(`UPDATE claims SET released_at = ?, revoked_at = ?, revoked_by = ?, revoke_reason = ? WHERE channel = ? AND thread = ?`,
				now, now, m.ID(), reason, m.Channel, thread); err != nil {
				return err
			}
		}
		cur, err = scanClaim(tx.QueryRow(selectClaim, m.Channel, thread))
		res = RevokeResult{State: RevokeRevoked, Claim: cur}
		return err
	})
	return res, err
}
