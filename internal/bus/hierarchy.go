package bus

import (
	"database/sql"
	"errors"
	"fmt"
)

// Delegation trees (docs/specs/agent-hierarchy.md, ADR-020): task.assign may
// name a parent thread; the broker checks the link inside the Send
// transaction and the client's claim is never trusted.

const (
	// MaxDepth is the deepest level below a root (a root is depth 0).
	MaxDepth = 3
	// MaxOpenChildren caps the open children of one parent.
	MaxOpenChildren = 8
)

// Error codes are the error text prefix, like lease_lost, so the CLI, MCP and
// harness can match them with errors.Is or by prefix.
var (
	ErrParentRequiresAssign = errors.New("parent_requires_assign")
	ErrParentRequiresThread = errors.New("parent_requires_thread")
	ErrParentSelf           = errors.New("parent_self")
	ErrParentUnknown        = errors.New("parent_unknown")
	ErrParentNotOwned       = errors.New("parent_not_owned")
	ErrParentImmutable      = errors.New("parent_immutable")
	ErrChildClaimed         = errors.New("child_claimed")
	ErrDepthExceeded        = errors.New("depth_exceeded")
	ErrFanoutExceeded       = errors.New("fanout_exceeded")
)

// linksSchema is shared by the fresh schema and the v3 -> v4 migration so the
// two cannot drift. The partial index serves the "has this thread an assign"
// lookups of every parented send without scanning the channel log.
const linksSchema = `
CREATE TABLE task_links (
	channel    TEXT NOT NULL REFERENCES channels(name),
	thread     TEXT NOT NULL,
	parent     TEXT NOT NULL,
	depth      INTEGER NOT NULL,
	assign_id  TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (channel, thread)
);
CREATE INDEX task_links_parent ON task_links(channel, parent);
CREATE INDEX messages_assign_thread ON messages(channel, thread) WHERE type = 'task.assign';
`

// migrateV3 adds the parent column and the link table. Existing threads get
// no links: they are all roots.
func migrateV3(tx *sql.Tx) error {
	if _, err := tx.Exec(`ALTER TABLE messages ADD COLUMN parent TEXT`); err != nil {
		return fmt.Errorf("migrating bus schema v3 -> v4: %w", err)
	}
	if _, err := tx.Exec(linksSchema); err != nil {
		return fmt.Errorf("migrating bus schema v3 -> v4: %w", err)
	}
	return nil
}

// validateParentShape is the client-independent part of R1.
func validateParentShape(in *SendInput) error {
	if in.Parent == "" {
		return nil
	}
	switch {
	case in.Type != TypeTaskAssign:
		return fmt.Errorf("%w: only task.assign can carry a parent", ErrParentRequiresAssign)
	case in.Thread == "":
		return fmt.Errorf("%w: a parented task.assign needs a thread", ErrParentRequiresThread)
	case in.Thread == in.Parent:
		return fmt.Errorf("%w: thread %q cannot be its own parent", ErrParentSelf, in.Thread)
	}
	return nil
}

type newLink struct{ depth int }

// checkParent runs the broker rules R3-R7 against the stored state. It
// returns the link to record, or nil when nothing new is linked (no parent,
// or a re-send of an already linked subtask).
func checkParent(tx *sql.Tx, m Member, in *SendInput, now int64) (*newLink, error) {
	if in.Parent == "" {
		return nil, nil
	}
	known, err := hasAssign(tx, m.Channel, in.Parent)
	if err != nil {
		return nil, err
	}
	if !known {
		return nil, fmt.Errorf("%w: thread %q has no task.assign in channel %q", ErrParentUnknown, in.Parent, m.Channel)
	}
	if err := requireParentOwner(tx, m, in.Parent); err != nil {
		return nil, err
	}
	assigned, err := hasAssign(tx, m.Channel, in.Thread)
	if err != nil {
		return nil, err
	}
	if assigned {
		cur, linked, err := linkOf(tx, m.Channel, in.Thread)
		if err != nil {
			return nil, err
		}
		if !linked || cur != in.Parent {
			return nil, fmt.Errorf("%w: thread %q already has a task.assign and its parent cannot change", ErrParentImmutable, in.Thread)
		}
		return nil, nil // same subtask sent again: already counted
	}
	if err := requireUnclaimed(tx, m.Channel, in.Thread); err != nil {
		return nil, err
	}
	depth, err := depthOf(tx, m.Channel, in.Parent)
	if err != nil {
		return nil, err
	}
	if depth+1 > MaxDepth {
		return nil, fmt.Errorf("%w: %q is at depth %d, the limit is %d", ErrDepthExceeded, in.Parent, depth, MaxDepth)
	}
	open, err := openChildren(tx, m.Channel, in.Parent, now)
	if err != nil {
		return nil, err
	}
	if len(open) >= MaxOpenChildren {
		return nil, fmt.Errorf("%w: %q already has %d open children", ErrFanoutExceeded, in.Parent, len(open))
	}
	return &newLink{depth: depth + 1}, nil
}

// depthOf is the stored depth of a thread; a root has none, so 0.
func depthOf(tx *sql.Tx, channel, thread string) (int, error) {
	depth := 0
	err := tx.QueryRow(`SELECT depth FROM task_links WHERE channel = ? AND thread = ?`, channel, thread).Scan(&depth)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	return depth, nil
}

// requireUnclaimed refuses linking a thread that already has a claim row.
func requireUnclaimed(tx *sql.Tx, channel, thread string) error {
	var one int
	err := tx.QueryRow(`SELECT 1 FROM claims WHERE channel = ? AND thread = ?`, channel, thread).Scan(&one)
	switch {
	case err == nil:
		return fmt.Errorf("%w: thread %q already has a claim and cannot be linked", ErrChildClaimed, thread)
	case errors.Is(err, sql.ErrNoRows):
		return nil
	}
	return err
}

// requireParentOwner is R4: the router, or the live owner of the parent claim.
// A released claim (watchdog or takeover) no longer entitles its old owner.
func requireParentOwner(tx *sql.Tx, m Member, parent string) error {
	if m.Role == RoleRouter {
		return nil
	}
	c, err := scanClaim(tx.QueryRow(selectClaim, m.Channel, parent))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && c.Owner == m.ID() && c.CompletedAt == nil && c.ReleasedAt == nil && c.RevokedAt == nil {
		return nil
	}
	return fmt.Errorf("%w: %s does not hold a live claim on %q", ErrParentNotOwned, m.ID(), parent)
}

func hasAssign(q queryRower, channel, thread string) (bool, error) {
	var one int
	err := q.QueryRow(`SELECT 1 FROM messages WHERE channel = ? AND thread = ? AND type = 'task.assign' LIMIT 1`, channel, thread).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func linkOf(q queryRower, channel, thread string) (parent string, linked bool, err error) {
	err = q.QueryRow(`SELECT parent FROM task_links WHERE channel = ? AND thread = ?`, channel, thread).Scan(&parent)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return parent, err == nil, err
}

func insertLink(tx *sql.Tx, channel, thread, parent string, depth int, assignID string, now int64) error {
	_, err := tx.Exec(`INSERT INTO task_links(channel, thread, parent, depth, assign_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		channel, thread, parent, depth, assignID, now)
	return err
}

// selectOpenChildren: a child is open unless its claim is completed or
// revoked, or it has no claim and every task.assign on it expired by ttl. A
// released claim stays open since another agent can take it over.
const selectOpenChildren = `SELECT l.thread FROM task_links l
	WHERE l.channel = ? AND l.parent = ?
	AND NOT EXISTS (SELECT 1 FROM claims c WHERE c.channel = l.channel AND c.thread = l.thread
		AND (c.completed_at IS NOT NULL OR c.revoked_at IS NOT NULL))
	AND (EXISTS (SELECT 1 FROM claims c WHERE c.channel = l.channel AND c.thread = l.thread)
		OR EXISTS (SELECT 1 FROM messages a WHERE a.channel = l.channel AND a.thread = l.thread
			AND a.type = 'task.assign' AND (a.expires_at IS NULL OR a.expires_at > ?)))
	ORDER BY l.thread`

func openChildren(q queryer, channel, parent string, now int64) ([]string, error) {
	rows, err := q.Query(selectOpenChildren, channel, parent, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
