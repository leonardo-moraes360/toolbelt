package bus

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Lease, heartbeat and watchdog (docs/bus-lease.md, B2).

const (
	DefaultLease = 2 * time.Minute
	MinLease     = 30 * time.Second
	MaxLease     = time.Hour

	// WatchdogSender is the broker-only sender of lease_expired: agent
	// names cannot hold ":", so no session can send as system:*.
	WatchdogSender = "system:watchdog"
)

// ErrLeaseLost: the caller no longer holds the thread at the epoch it
// presented (taken over, released, or another epoch).
var ErrLeaseLost = errors.New("lease_lost")

// migrateV1 adds the lease columns; existing agents start alive at the
// migration time, so an upgrade does not expire everyone at once.
func migrateV1(tx *sql.Tx, now int64) error {
	for _, q := range []string{
		`ALTER TABLE channels ADD COLUMN lease_ns INTEGER NOT NULL DEFAULT 120000000000`,
		`ALTER TABLE agents ADD COLUMN last_seen INTEGER`,
		`ALTER TABLE claims ADD COLUMN epoch INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE claims ADD COLUMN released_at INTEGER`,
		`ALTER TABLE claims ADD COLUMN prev_owner TEXT`,
	} {
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("migrating bus schema v1 -> v2: %w", err)
		}
	}
	if _, err := tx.Exec(`UPDATE agents SET last_seen = ?`, now); err != nil {
		return err
	}
	_, err := tx.Exec(setSchemaVersion)
	return err
}

// touch renews the caller's lease.
func touch(tx *sql.Tx, m Member, now int64) error {
	_, err := tx.Exec(`UPDATE agents SET last_seen = ? WHERE channel = ? AND name = ?`, now, m.Channel, m.Name)
	return err
}

// leaseOf returns an agent's last_seen, clamped to now (a clock stepped
// backwards cannot extend a lease), and whether it is alive.
func leaseOf(q queryRower, channel, agentID string, lease time.Duration, now int64) (lastSeen int64, alive bool, err error) {
	var ls sql.NullInt64
	err = q.QueryRow(`SELECT last_seen FROM agents WHERE channel = ? AND ? = 'agent:' || name`, channel, agentID).Scan(&ls)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	v := min(ls.Int64, now)
	return v, ls.Valid && now < v+int64(lease), nil
}

// ClaimThread takes a thread (docs/bus-lease.md R2): a free thread with
// epoch 1; one's own held thread idempotently; a released thread, or one
// whose owner is not alive, by takeover with epoch+1 (publishing
// lease_expired when it was not released yet). An alive owner keeps it.
func (s *Store) ClaimThread(m Member, thread string) (Claim, error) {
	if thread == "" {
		return Claim{}, errors.New("thread is required")
	}
	if err := validRef("thread", thread); err != nil {
		return Claim{}, err
	}
	var c Claim
	err := s.tx(func(tx *sql.Tx) error {
		now := s.nowNS()
		ch, err := activeChannel(tx, m.Channel, now)
		if err != nil {
			return err
		}
		if err := touch(tx, m, now); err != nil {
			return err
		}
		cur, err := scanClaim(tx.QueryRow(selectClaim, m.Channel, thread))
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if _, err := tx.Exec(`INSERT INTO claims(channel, thread, owner, claimed_at, epoch) VALUES (?, ?, ?, ?, 1)`,
				m.Channel, thread, m.ID(), now); err != nil {
				return err
			}
			c = Claim{Channel: m.Channel, Thread: thread, Owner: m.ID(), ClaimedAt: fromNS(now), Epoch: 1}
			return nil
		case err != nil:
			return err
		case cur.CompletedAt != nil:
			return fmt.Errorf("thread %q already completed by %s", thread, cur.Owner)
		case cur.ReleasedAt != nil:
			c, err = takeover(tx, &cur, m, now)
			return err
		case cur.Owner == m.ID():
			c = cur
			return nil
		}
		lastSeen, alive, err := leaseOf(tx, m.Channel, cur.Owner, ch.Lease, now)
		if err != nil {
			return err
		}
		if alive {
			return fmt.Errorf("thread %q already claimed by %s (lease ends %s)", thread, cur.Owner,
				fromNS(lastSeen+int64(ch.Lease)).Format(time.RFC3339))
		}
		if err := publishExpired(tx, &cur, lastSeen, now); err != nil {
			return err
		}
		c, err = takeover(tx, &cur, m, now)
		return err
	})
	return c, err
}

// takeover gives cur to m with epoch+1; prev_owner records another owner.
func takeover(tx *sql.Tx, cur *Claim, m Member, now int64) (Claim, error) {
	prev := sql.NullString{}
	if cur.Owner != m.ID() {
		prev = sql.NullString{String: cur.Owner, Valid: true}
	}
	epoch := cur.Epoch + 1
	if _, err := tx.Exec(`UPDATE claims SET owner = ?, claimed_at = ?, epoch = ?, released_at = NULL, prev_owner = ? WHERE channel = ? AND thread = ?`,
		m.ID(), now, epoch, prev, cur.Channel, cur.Thread); err != nil {
		return Claim{}, err
	}
	return Claim{Channel: cur.Channel, Thread: cur.Thread, Owner: m.ID(), ClaimedAt: fromNS(now), Epoch: epoch, PrevOwner: prev.String}, nil
}

// expiredEvent is the lease_expired payload (R5).
type expiredEvent struct {
	State         string `json:"state"`
	Thread        string `json:"thread"`
	PreviousOwner string `json:"previous_owner"`
	Epoch         int64  `json:"epoch"`
	LastSeen      string `json:"last_seen,omitempty"`
}

// publishExpired releases cur and publishes lease_expired to topic:board
// from system:watchdog.
func publishExpired(tx *sql.Tx, cur *Claim, lastSeen, now int64) error {
	if _, err := tx.Exec(`UPDATE claims SET released_at = ? WHERE channel = ? AND thread = ?`, now, cur.Channel, cur.Thread); err != nil {
		return err
	}
	ev := expiredEvent{State: "lease_expired", Thread: cur.Thread, PreviousOwner: cur.Owner, Epoch: cur.Epoch}
	if lastSeen > 0 {
		ev.LastSeen = fromNS(lastSeen).Format(time.RFC3339)
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = insertMessage(tx, cur.Channel, WatchdogSender, "topic:board", cur.Thread, TypeTaskStatus, "", "", "", 0, payload, now)
	return err
}

type CompleteResult struct {
	Claim   Claim   `json:"claim"`
	Message Message `json:"message"`
}

// Complete closes a thread with a result (R3): only the current owner, at
// the epoch it got from claim, on a claim that was not released; it also
// publishes task.result to topic:board.
func (s *Store) Complete(m Member, thread string, epoch int64, result json.RawMessage) (CompleteResult, error) {
	if thread == "" {
		return CompleteResult{}, errors.New("thread is required")
	}
	if epoch <= 0 {
		return CompleteResult{}, errors.New("epoch is required: pass the epoch claim returned")
	}
	payload, err := sanitizePayload(result)
	if err != nil {
		return CompleteResult{}, err
	}
	var res CompleteResult
	err = s.tx(func(tx *sql.Tx) error {
		now := s.nowNS()
		if _, err := activeChannel(tx, m.Channel, now); err != nil {
			return err
		}
		cur, err := scanClaim(tx.QueryRow(selectClaim, m.Channel, thread))
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("thread %q was not claimed (use claim first)", thread)
		}
		if err != nil {
			return err
		}
		switch {
		case cur.CompletedAt != nil:
			return fmt.Errorf("thread %q already completed", thread)
		case cur.Owner != m.ID():
			return fmt.Errorf("%w: thread %q is now owned by %s (epoch %d)", ErrLeaseLost, thread, cur.Owner, cur.Epoch)
		case cur.ReleasedAt != nil:
			return fmt.Errorf("%w: thread %q was released after the lease expired; claim it again", ErrLeaseLost, thread)
		case cur.Epoch != epoch:
			return fmt.Errorf("%w: thread %q is at epoch %d, not %d", ErrLeaseLost, thread, cur.Epoch, epoch)
		}
		if err := touch(tx, m, now); err != nil {
			return err
		}
		msg, err := insertMessage(tx, m.Channel, m.ID(), "topic:board", thread, TypeTaskResult, "",
			"", "", 0, payload, now)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE claims SET completed_at = ?, result_id = ? WHERE channel = ? AND thread = ?`,
			now, msg.ID, m.Channel, thread); err != nil {
			return err
		}
		t := fromNS(now)
		cur.CompletedAt = &t
		cur.ResultID = msg.ID
		res = CompleteResult{Claim: cur, Message: msg}
		return nil
	})
	return res, err
}

// HeldThread is a thread the agent holds.
type HeldThread struct {
	Thread string `json:"thread"`
	Epoch  int64  `json:"epoch"`
}

// LostThread is a thread the agent lost: taken over (NewOwner set) or
// released by the watchdog.
type LostThread struct {
	Thread   string `json:"thread"`
	NewOwner string `json:"new_owner,omitempty"`
}

// HeartbeatResult is R4's answer.
type HeartbeatResult struct {
	LeaseEndsAt       time.Time    `json:"lease_ends_at"`
	HeartbeatInterval string       `json:"heartbeat_interval"`
	Held              []HeldThread `json:"held"`
	Lost              []LostThread `json:"lost"`
}

// Heartbeat renews the caller's lease and reports what it holds and has
// lost (R4). It never revives a released claim.
func (s *Store) Heartbeat(m Member) (HeartbeatResult, error) {
	var res HeartbeatResult
	err := s.tx(func(tx *sql.Tx) error {
		now := s.nowNS()
		ch, err := activeChannel(tx, m.Channel, now)
		if err != nil {
			return err
		}
		if err := touch(tx, m, now); err != nil {
			return err
		}
		res = HeartbeatResult{LeaseEndsAt: fromNS(now + int64(ch.Lease)), HeartbeatInterval: (ch.Lease / 3).String(),
			Held: []HeldThread{}, Lost: []LostThread{}}
		rows, err := tx.Query(`SELECT thread, owner, epoch, released_at, prev_owner FROM claims
			WHERE channel = ? AND completed_at IS NULL AND (owner = ? OR prev_owner = ?) ORDER BY thread`,
			m.Channel, m.ID(), m.ID())
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var thread, owner string
			var epoch int64
			var released sql.NullInt64
			var prev sql.NullString
			if err := rows.Scan(&thread, &owner, &epoch, &released, &prev); err != nil {
				return err
			}
			switch {
			case owner != m.ID():
				res.Lost = append(res.Lost, LostThread{Thread: thread, NewOwner: owner})
			case released.Valid:
				res.Lost = append(res.Lost, LostThread{Thread: thread})
			default:
				res.Held = append(res.Held, HeldThread{Thread: thread, Epoch: epoch})
			}
		}
		return rows.Err()
	})
	return res, err
}

// Expired is a claim the watchdog released (or would release).
type Expired struct {
	Thread        string    `json:"thread"`
	PreviousOwner string    `json:"previous_owner"`
	Epoch         int64     `json:"epoch"`
	LastSeen      time.Time `json:"last_seen"`
}

// Watchdog releases every open claim whose owner is not alive and
// publishes one lease_expired per claim (R5). Human or router only (CLI).
func (s *Store) Watchdog(channel string, dryRun bool) ([]Expired, error) {
	out := []Expired{}
	err := s.tx(func(tx *sql.Tx) error {
		now := s.nowNS()
		ch, err := activeChannel(tx, channel, now)
		if err != nil {
			return err // an expired channel refuses writes, the watchdog's too (R9)
		}
		claims, err := openClaims(tx, channel)
		if err != nil {
			return err
		}
		for i := range claims {
			c := &claims[i]
			if c.ReleasedAt != nil {
				continue
			}
			lastSeen, alive, err := leaseOf(tx, channel, c.Owner, ch.Lease, now)
			if err != nil {
				return err
			}
			if alive {
				continue
			}
			out = append(out, Expired{Thread: c.Thread, PreviousOwner: c.Owner, Epoch: c.Epoch, LastSeen: fromNS(lastSeen)})
			if !dryRun {
				if err := publishExpired(tx, c, lastSeen, now); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return out, err
}

func openClaims(tx queryer, channel string) ([]Claim, error) {
	rows, err := tx.Query(`SELECT `+claimColumns+` FROM claims WHERE channel = ? AND completed_at IS NULL ORDER BY thread`, channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Claim
	for rows.Next() {
		c, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// AgentLease is an agent in status: no Agent Card (its session field is a
// vendor resume handle), no token.
type AgentLease struct {
	Name     string    `json:"name"`
	Role     Role      `json:"role"`
	Alive    bool      `json:"alive"`
	LastSeen time.Time `json:"last_seen"`
}

// ClaimLease is an open claim in status.
type ClaimLease struct {
	Thread string `json:"thread"`
	Owner  string `json:"owner"`
	Epoch  int64  `json:"epoch"`
	State  string `json:"state"` // held, expired or released
}

// LeaseStatus is R8's status.
type LeaseStatus struct {
	Channel string       `json:"channel"`
	Lease   string       `json:"lease"`
	Agents  []AgentLease `json:"agents"`
	Claims  []ClaimLease `json:"claims"`
}

// Status lists agents' liveness and open claims (R8), reading only. Both
// reads share one transaction so a heartbeat cannot land between them.
func (s *Store) Status(channel string) (LeaseStatus, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return LeaseStatus{}, err
	}
	defer tx.Rollback() //nolint:errcheck // read-only
	now := s.nowNS()
	ch, err := activeChannel(tx, channel, now)
	if err != nil && !errors.Is(err, ErrChannelExpired) {
		return LeaseStatus{}, err
	}
	st := LeaseStatus{Channel: channel, Lease: ch.Lease.String(), Agents: []AgentLease{}, Claims: []ClaimLease{}}
	alive, err := agentLeases(tx, &st, ch.Lease, now)
	if err != nil {
		return LeaseStatus{}, err
	}
	claims, err := openClaims(tx, channel)
	if err != nil {
		return LeaseStatus{}, err
	}
	for i := range claims {
		c := &claims[i]
		state := "held"
		switch {
		case c.ReleasedAt != nil:
			state = "released"
		case !alive[c.Owner]:
			state = "expired"
		}
		st.Claims = append(st.Claims, ClaimLease{Thread: c.Thread, Owner: c.Owner, Epoch: c.Epoch, State: state})
	}
	return st, nil
}

// agentLeases fills st.Agents and returns liveness by agent ID.
func agentLeases(tx *sql.Tx, st *LeaseStatus, lease time.Duration, now int64) (map[string]bool, error) {
	rows, err := tx.Query(`SELECT name, role, last_seen FROM agents WHERE channel = ? ORDER BY name`, st.Channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	alive := map[string]bool{}
	for rows.Next() {
		var a AgentLease
		var ls sql.NullInt64
		if err := rows.Scan(&a.Name, &a.Role, &ls); err != nil {
			return nil, err
		}
		v := min(ls.Int64, now)
		a.LastSeen, a.Alive = fromNS(v), ls.Valid && now < v+int64(lease)
		alive[AgentID(a.Name)] = a.Alive
		st.Agents = append(st.Agents, a)
	}
	return alive, rows.Err()
}
