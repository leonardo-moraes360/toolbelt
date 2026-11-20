package bus

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// AC1: revoke on an open thread fences claim, complete and shows as lost.
func TestRevokeOpenThread(t *testing.T) {
	s, _, owner, w := setup(t)
	c, err := s.ClaimThread(w, "T/x.1")
	must(t, err)
	r, err := s.Revoke(owner, "T/x.1", "timeout")
	must(t, err)
	if r.State != RevokeRevoked || r.Claim.RevokedAt == nil || r.Claim.RevokeReason != "timeout" {
		t.Fatalf("revoke: %+v", r)
	}
	if _, err := s.Complete(w, "T/x.1", c.Epoch, nil); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("complete after revoke: %v", err)
	}
	other := third(t, s, "other")
	if _, err := s.ClaimThread(other, "T/x.1"); err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("claim after revoke: %v", err)
	}
	hb, err := s.Heartbeat(w)
	must(t, err)
	if len(hb.Lost) != 1 || hb.Lost[0].Reason != "timeout" {
		t.Fatalf("heartbeat lost: %+v", hb.Lost)
	}
	if _, err := s.Revoke(w, "T/x.1", "timeout"); err == nil {
		t.Fatal("a worker revoked a thread")
	}
	if _, err := s.Revoke(owner, "T/x.1", "Bad Reason"); err == nil {
		t.Fatal("invalid reason accepted")
	}
}

// AC2: a completed thread wins over a revoke; an unclaimed one is closed.
func TestRevokeCompletedAndUnclaimed(t *testing.T) {
	s, _, owner, w := setup(t)
	c, err := s.ClaimThread(w, "T/x.1")
	must(t, err)
	done, err := s.Complete(w, "T/x.1", c.Epoch, []byte(`{"verdict":"PASS"}`))
	must(t, err)
	r, err := s.Revoke(owner, "T/x.1", "timeout")
	must(t, err)
	if r.State != RevokeCompleted || r.Claim.ResultID != done.Message.ID || r.Claim.RevokedAt != nil {
		t.Fatalf("revoke on completed: %+v", r)
	}
	r, err = s.Revoke(owner, "T/y.1", "unclaimed")
	must(t, err)
	if r.State != RevokeRevoked {
		t.Fatalf("revoke unclaimed: %+v", r)
	}
	if _, err := s.ClaimThread(w, "T/y.1"); err == nil {
		t.Fatal("claim on a revoked unclaimed thread")
	}
	hb, err := s.Heartbeat(owner)
	must(t, err)
	if len(hb.Lost) != 0 {
		t.Fatalf("the revoker sees its own tombstone as lost: %+v", hb.Lost)
	}
	got, err := s.Claim("feature-x", "T/x.1")
	must(t, err)
	msg, err := s.Message("feature-x", got.ResultID)
	must(t, err)
	if msg.Type != TypeTaskResult || msg.From != w.ID() {
		t.Fatalf("message: %+v", msg)
	}
}

// AC3: concurrent revoke and complete: exactly one wins and Revoke says which.
func TestRevokeCompleteRace(t *testing.T) {
	for range 20 {
		s, _, owner, w := setup(t)
		c, err := s.ClaimThread(w, "T/r.1")
		must(t, err)
		var wg sync.WaitGroup
		var rev RevokeResult
		var revErr, compErr error
		wg.Add(2)
		go func() { defer wg.Done(); rev, revErr = s.Revoke(owner, "T/r.1", "timeout") }()
		go func() { defer wg.Done(); _, compErr = s.Complete(w, "T/r.1", c.Epoch, nil) }()
		wg.Wait()
		must(t, revErr)
		switch rev.State {
		case RevokeCompleted:
			must(t, compErr)
		case RevokeRevoked:
			if !errors.Is(compErr, ErrLeaseLost) {
				t.Fatalf("revoked but complete returned %v", compErr)
			}
		default:
			t.Fatalf("state %q", rev.State)
		}
	}
}

// AC5: ClaimConfined stores the marker; an ordinary claim or takeover does not.
func TestClaimConfinedMarker(t *testing.T) {
	s, c, _, w := setup(t)
	got, err := s.ClaimConfined(w, "T/m.1")
	must(t, err)
	if got.Confinement != ConfinementMarker {
		t.Fatalf("marker: %+v", got)
	}
	plain, err := s.ClaimThread(w, "T/m.2")
	must(t, err)
	if plain.Confinement != "" {
		t.Fatalf("ordinary claim has a marker: %+v", plain)
	}
	if _, err := s.ClaimConfined(w, "T/m.2"); err == nil {
		t.Fatal("a thread held unconfined gained the marker")
	}
	if again, err := s.ClaimConfined(w, "T/m.1"); err != nil || again.Epoch != got.Epoch {
		t.Fatalf("confined re-claim: %+v %v", again, err)
	}
	c.add(DefaultLease + 1)
	b := third(t, s, "b-agent")
	taken, err := s.ClaimThread(b, "T/m.1")
	must(t, err)
	if taken.Confinement != "" {
		t.Fatalf("takeover inherited the marker: %+v", taken)
	}
}

// AC4: a v2 store migrates to v3 keeping its claims.
func TestMigrateV2(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	must(t, err)
	must(t, s.Close())
	db, err := sql.Open("sqlite", filepath.Join(dir, ".toolbelt", "bus", "bus.db"))
	must(t, err)
	for _, q := range []string{
		`ALTER TABLE claims DROP COLUMN revoked_at`, `ALTER TABLE claims DROP COLUMN revoked_by`,
		`ALTER TABLE claims DROP COLUMN revoke_reason`, `ALTER TABLE claims DROP COLUMN confinement`,
		`INSERT INTO channels(name, owner, created_at) VALUES ('c', 'o', 1)`,
		`INSERT INTO claims(channel, thread, owner, claimed_at, epoch) VALUES ('c', 'T', 'agent:o', 1, 3)`,
		`PRAGMA user_version = 2`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}
	must(t, db.Close())
	s, err = Open(dir)
	must(t, err)
	defer s.Close()
	c, err := s.Claim("c", "T")
	must(t, err)
	if c.Epoch != 3 || c.RevokedAt != nil || c.Confinement != "" {
		t.Fatalf("migrated claim: %+v", c)
	}
}

func TestSelectMessageByID(t *testing.T) {
	if selectMessageByID != selectMessage+` WHERE channel = ? AND id = ?` {
		t.Fatal("selectMessageByID drifted from selectMessage")
	}
}
